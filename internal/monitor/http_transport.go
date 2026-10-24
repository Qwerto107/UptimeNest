package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// probeTransport 在发送 HTTP 请求前完成 QUIC 握手；仅握手失败可自动回退，避免重发 POST。
type probeTransport struct {
	tcp     *http.Transport
	version string
	timeout time.Duration
	beat    *model.Heartbeat
}

func (t *probeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resetTLS(t.beat)
	if t.version == "standard" || req.URL.Scheme != "https" {
		if t.version == "http3" {
			return nil, fmt.Errorf("仅 HTTP/3 模式不能请求非 HTTPS 地址")
		}
		return t.tcp.RoundTrip(req)
	}
	proxy, err := http.ProxyFromEnvironment(req)
	if err != nil {
		return nil, err
	}
	if proxy != nil {
		if t.version == "http3" {
			return nil, fmt.Errorf("当前环境代理不支持 HTTP/3 UDP 连接")
		}
		return t.tcp.RoundTrip(req)
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
	conn, err := quic.DialAddr(dialCtx, net.JoinHostPort(req.URL.Hostname(), port), tlsConfig, &quic.Config{HandshakeIdleTimeout: budget})
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
