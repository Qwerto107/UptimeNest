package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"golang.org/x/net/idna"
)

// probeTransport 在发送 HTTP 请求前完成 QUIC 握手；仅握手失败可自动回退，避免重发 POST。
type probeTransport struct {
	tcp     *http.Transport
	version string
	timeout time.Duration
	beat    *model.Heartbeat
	host    string
	ip      string
}

// connectAddress 仅覆盖原目标主机，保留请求端口，不影响跳转到其他主机的连接。
func (t *probeTransport) connectAddress(address string) string {
	if t.ip == "" {
		return address
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || !t.matchesHost(host) {
		return address
	}
	return net.JoinHostPort(t.ip, port)
}

// proxy 对指定 IP 的目标直接连接，其他主机仍遵循环境代理。
func (t *probeTransport) proxy(req *http.Request) (*url.URL, error) {
	if t.ip != "" && t.matchesHost(req.URL.Hostname()) {
		return nil, nil
	}
	return http.ProxyFromEnvironment(req)
}

// matchesHost 兼容 net/http 拨号时的 IDNA 转换，避免国际化域名漏用指定 IP。
func (t *probeTransport) matchesHost(host string) bool {
	if strings.EqualFold(host, t.host) {
		return true
	}
	source, sourceErr := idna.Lookup.ToASCII(t.host)
	target, targetErr := idna.Lookup.ToASCII(host)
	return sourceErr == nil && targetErr == nil && strings.EqualFold(source, target)
}

func (t *probeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resetTLS(t.beat)
	if t.version == "http3" && req.URL.Scheme != "https" {
		return nil, fmt.Errorf("仅 HTTP/3 模式不能请求非 HTTPS 地址")
	}
	if req.URL.Scheme != "https" || (t.version != "auto" && t.version != "http3") {
		return t.tcp.RoundTrip(req)
	}
	if t.tcp.Proxy != nil {
		proxy, err := t.tcp.Proxy(req)
		if err != nil {
			return nil, err
		}
		if proxy != nil {
			if t.version == "http3" {
				return nil, fmt.Errorf("当前环境代理不支持 HTTP/3 UDP 连接")
			}
			return t.tcp.RoundTrip(req)
		}
	}
	tlsConfig := t.tcp.TLSClientConfig.Clone()
	tlsConfig.NextProtos = []string{http3.NextProtoH3}
	if tlsConfig.ServerName == "" {
		tlsConfig.ServerName = req.URL.Hostname()
	}
	port := req.URL.Port()
	if port == "" {
		port = "443"
	}
	budget := t.timeout
	if t.version == "auto" {
		budget = min(2*time.Second, t.timeout/3)
	}
	dialCtx, cancel := context.WithTimeout(req.Context(), budget)
	start := time.Now()
	// 不使用 DialAddrEarly，不主动启用 0-RTT；详情只报告连接的真实使用状态。
	conn, err := quic.DialAddr(dialCtx, t.connectAddress(net.JoinHostPort(req.URL.Hostname(), port)), tlsConfig, &quic.Config{HandshakeIdleTimeout: budget})
	cancel()
	if err != nil {
		if t.version == "auto" && req.Context().Err() == nil {
			return t.tcp.RoundTrip(req)
		}
		return nil, fmt.Errorf("HTTP/3 连接失败：%w", err)
	}
	handshake := float64(time.Since(start).Microseconds()) / 1000
	transport := &http3.Transport{
		TLSClientConfig: tlsConfig,
		Dial: func(context.Context, string, *tls.Config, *quic.Config) (*quic.Conn, error) {
			return conn, nil
		},
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		transport.Close()
		conn.CloseWithError(0, "")
		return nil, err
	}
	state := conn.ConnectionState()
	readTLS(state.TLS, t.beat)
	t.beat.TLSLatency = handshake
	t.beat.QUICVersion = state.Version.String()
	used0RTT := state.Used0RTT
	t.beat.ZeroRTT = &used0RTT
	resp.Body = &http3Body{ReadCloser: resp.Body, transport: transport, conn: conn}
	return resp, nil
}

// http3Body 在读取完成后关闭流、连接和 UDP 资源，每次检测重新测量握手。
type http3Body struct {
	io.ReadCloser
	transport *http3.Transport
	conn      *quic.Conn
}

func (b *http3Body) Close() error {
	err := b.ReadCloser.Close()
	b.transport.Close()
	b.conn.CloseWithError(0, "")
	return err
}
