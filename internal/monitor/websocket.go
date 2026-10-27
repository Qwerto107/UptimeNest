package monitor

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http/httptrace"
	"net/url"
	"slices"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/gorilla/websocket"
)

func probeWebSocket(ctx context.Context, m model.Monitor, ua string, h *model.Heartbeat) error {
	if err := ValidateWebSocket(&m); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(m.Timeout)*time.Second)
	defer cancel()
	v := m.WebSocketConfig()
	u, _ := url.Parse(m.Target)
	headers, _ := httpHeaders(v.Headers)
	if ua == "" {
		ua = model.DefaultUserAgent
	}
	if headers.Get("User-Agent") == "" {
		headers.Set("User-Agent", ua)
	}
	if v.Origin != "" {
		headers.Set("Origin", v.Origin)
	}
	var tlsStart time.Time
	trace := &httptrace.ClientTrace{
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			h.TLSLatency = float64(time.Since(tlsStart).Microseconds()) / 1000
			if err == nil {
				readTLS(state, h)
			}
		},
	}
	ctx = httptrace.WithClientTrace(ctx, trace)
	probeCtx := ctx
	target := &probeTransport{host: u.Hostname(), ip: v.ConnectIP}
	netDialer := &net.Dialer{}
	var stopCancel func() bool
	defer func() {
		if stopCancel != nil {
			stopCancel()
		}
	}()
	dialer := websocket.Dialer{
		Proxy:            target.proxy,
		TLSClientConfig:  &tls.Config{MinVersion: tls.VersionTLS12},
		Subprotocols:     v.Subprotocols,
		HandshakeTimeout: time.Duration(m.Timeout) * time.Second,
		NetDialContext: func(dialCtx context.Context, network, address string) (net.Conn, error) {
			conn, err := netDialer.DialContext(dialCtx, network, target.connectAddress(address))
			if err == nil {
				// 取消调度时同时中断 HTTP 升级读取和后续心跳等待，不等超时耗尽。
				stopCancel = context.AfterFunc(probeCtx, func() { conn.Close() })
			}
			return conn, err
		},
	}
	conn, resp, err := dialer.DialContext(ctx, m.Target, headers)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		readFailedCertificate(err, h)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if resp != nil {
			return fmt.Errorf("WebSocket 握手失败（HTTP %d）", resp.StatusCode)
		}
		return fmt.Errorf("WebSocket 连接失败：%w", err)
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	defer func() {
		closeDeadline := time.Now().Add(100 * time.Millisecond)
		if deadline.Before(closeDeadline) {
			closeDeadline = deadline
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), closeDeadline)
	}()
	h.HTTPVersion = resp.Proto
	if selected := conn.Subprotocol(); selected != "" && !slices.Contains(v.Subprotocols, selected) {
		return fmt.Errorf("WebSocket 服务端返回了未请求的子协议")
	}
	if v.Mode == "ping" {
		if err := websocketPing(ctx, conn, deadline); err != nil {
			return err
		}
		h.Message = "WebSocket 握手与 Ping/Pong 成功"
	} else {
		h.Message = "WebSocket 握手成功"
	}
	return nil
}

// websocketPing 仅接受与本次随机 Ping 相同的 Pong，业务消息丢弃且限制单条大小。
func websocketPing(ctx context.Context, conn *websocket.Conn, deadline time.Time) error {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	payload := hex.EncodeToString(nonce[:])
	received := false
	done := errors.New("已收到匹配的 Pong")
	conn.SetReadLimit(64 << 10)
	if err := conn.SetReadDeadline(deadline); err != nil {
		return err
	}
	conn.SetPongHandler(func(value string) error {
		if value == payload {
			received = true
			return done
		}
		return nil
	})
	if err := conn.WriteControl(websocket.PingMessage, []byte(payload), deadline); err != nil {
		return fmt.Errorf("WebSocket 发送 Ping 失败：%w", err)
	}
	for {
		_, reader, err := conn.NextReader()
		if received {
			return nil
		}
		if err == nil {
			_, err = io.Copy(io.Discard, reader)
		}
		if received {
			return nil
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var closed *websocket.CloseError
			if errors.As(err, &closed) {
				return fmt.Errorf("WebSocket 心跳前连接已关闭（代码 %d）", closed.Code)
			}
			return fmt.Errorf("WebSocket 等待 Pong 失败：%w", err)
		}
	}
}
