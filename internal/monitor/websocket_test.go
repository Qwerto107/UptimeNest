package monitor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/gorilla/websocket"
)

func TestValidateWebSocket(t *testing.T) {
	m := model.Monitor{Target: "wss://example.com/ws?token=private"}
	if err := ValidateWebSocket(&m); err != nil || m.WebSocket.Mode != "handshake" || m.WebSocket.Subprotocols == nil {
		t.Fatalf("default options: %+v, %v", m.WebSocket, err)
	}
	m.WebSocket = &model.WebSocketOptions{Mode: "ping", ConnectIP: " 2001:0db8::1 ", Origin: "https://example.com/", Subprotocols: []string{" chat "}, Headers: `{"Authorization":"Bearer private","Host":"custom.example"}`}
	if err := ValidateWebSocket(&m); err != nil || m.WebSocket.ConnectIP != "2001:db8::1" || m.WebSocket.Origin != "https://example.com" || m.WebSocket.Subprotocols[0] != "chat" {
		t.Fatalf("normalization: %+v, %v", m.WebSocket, err)
	}
	for _, target := range []string{"https://example.com", "ws://", "ws://user:pass@example.com", "ws://example.com/#fragment", "ws://example.com/#", "ws://example.com:0", "ws://example.com:65536", "ws://example.com:bad"} {
		t.Run(target, func(t *testing.T) {
			m := model.Monitor{Target: target}
			if ValidateWebSocket(&m) == nil {
				t.Fatal("accepted invalid target")
			}
		})
	}
	for name, options := range map[string]model.WebSocketOptions{
		"mode": {Mode: "other"}, "ip": {ConnectIP: "127.0.0.1:80"},
		"origin path": {Origin: "https://example.com/path"}, "origin query": {Origin: "https://example.com?token=private"},
		"origin credentials": {Origin: "https://user:pass@example.com"}, "origin newline": {Origin: "https://example.com\r\nX-Evil: yes"},
		"duplicate protocols": {Subprotocols: []string{"chat", "chat"}}, "invalid token": {Subprotocols: []string{"bad protocol"}},
		"too many protocols": {Subprotocols: strings.Fields(strings.Repeat("p ", 17))}, "too long token": {Subprotocols: []string{strings.Repeat("a", 129)}},
		"json": {Headers: `{"Authorization":123}`}, "newline": {Headers: `{"X-Test":"a\r\nInjected:yes"}`},
		"large headers": {Headers: strings.Repeat("x", (64<<10)+1)},
	} {
		t.Run(name, func(t *testing.T) {
			m := model.Monitor{Target: "ws://example.com", WebSocket: &options}
			if ValidateWebSocket(&m) == nil {
				t.Fatal("accepted invalid options")
			}
		})
	}
	for _, name := range []string{"Connection", "Upgrade", "Sec-WebSocket-Key", "sec-websocket-protocol", "Sec-WebSocket-Extensions", "Origin", "Content-Length"} {
		t.Run(name, func(t *testing.T) {
			m := model.Monitor{Target: "ws://example.com", WebSocket: &model.WebSocketOptions{Headers: `{"` + name + `":"bad"}`}}
			if ValidateWebSocket(&m) == nil {
				t.Fatal("accepted managed header")
			}
		})
	}
}

func wsMonitor(serverURL string, mode string) model.Monitor {
	return model.Monitor{Type: "websocket", Target: strings.Replace(serverURL, "http", "ws", 1), Timeout: 2, WebSocket: &model.WebSocketOptions{Mode: mode}}
}

func readWebSocket(conn *websocket.Conn) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func TestWebSocketHandshakeAndPing(t *testing.T) {
	for _, mode := range []string{"handshake", "ping"} {
		t.Run(mode, func(t *testing.T) {
			var pings atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Host != "custom.example" || r.Header.Get("Origin") != "https://app.example" || r.Header.Get("Authorization") != "Bearer private" || r.Header.Get("User-Agent") != "WS-test" || r.URL.Path != "/ws" || r.URL.RawQuery != "token=private" {
					t.Errorf("request settings lost: host=%s path=%s", r.Host, r.URL.Path)
				}
				upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }, Subprotocols: []string{"chat"}}
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				conn.SetPingHandler(func(payload string) error {
					pings.Add(1)
					// 普通业务消息与不匹配的 Pong 不能抢先结束本次心跳检测。
					if err := conn.WriteMessage(websocket.TextMessage, []byte("ready")); err != nil {
						return err
					}
					if err := conn.WriteControl(websocket.PongMessage, []byte("wrong"), time.Now().Add(time.Second)); err != nil {
						return err
					}
					return conn.WriteControl(websocket.PongMessage, []byte(payload), time.Now().Add(time.Second))
				})
				readWebSocket(conn)
			}))
			defer server.Close()
			u, _ := url.Parse(server.URL)
			m := wsMonitor("http://unresolvable.invalid:"+u.Port()+"/ws?token=private", mode)
			m.WebSocket.ConnectIP = "127.0.0.1"
			m.WebSocket.Origin = "https://app.example"
			m.WebSocket.Subprotocols = []string{"chat"}
			m.WebSocket.Headers = `{"Host":"custom.example","Authorization":"Bearer private"}`
			h := Probe(context.Background(), m, "WS-test")
			if h.Status != "up" || h.HTTPVersion != "HTTP/1.1" || h.Latency <= 0 || strings.Contains(h.Message, "private") {
				t.Fatalf("probe failed: %+v", h)
			}
			if (mode == "ping") != (pings.Load() == 1) {
				t.Fatalf("wrong ping count: %d", pings.Load())
			}
		})
	}
}

func TestWebSocketMissingPong(t *testing.T) {
	for _, mode := range []string{"handshake", "ping"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				conn.SetPingHandler(func(string) error {
					return conn.WriteControl(websocket.PongMessage, []byte("unsolicited"), time.Now().Add(time.Second))
				})
				readWebSocket(conn)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			h := Probe(ctx, wsMonitor(server.URL, mode), "")
			if (h.Status == "up") != (mode == "handshake") {
				t.Fatalf("unexpected result: %+v", h)
			}
			if mode == "ping" && (h.Latency < 100 || h.Latency > 1500) {
				t.Fatalf("timeout not enforced: %+v", h)
			}
		})
	}
}

func TestWebSocketCancelDuringHandshake(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan model.Heartbeat, 1)
	go func() { done <- Probe(ctx, wsMonitor(server.URL, "handshake"), "") }()
	select {
	case <-started:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("handshake never started")
	}
	select {
	case h := <-done:
		if h.Status != "down" {
			t.Fatalf("cancelled probe succeeded: %+v", h)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not close handshake connection")
	}
}

func TestWebSocketRejectBadHandshakeAndRedirect(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer target.Close()
	for _, status := range []int{101, 200, 302, 401} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL+"/private")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Sec-WebSocket-Accept", "invalid")
				w.WriteHeader(status)
			}))
			defer server.Close()
			h := Probe(context.Background(), wsMonitor(server.URL, "handshake"), "")
			if h.Status != "down" || !strings.Contains(h.Message, "握手失败") || strings.Contains(h.Message, "private") {
				t.Fatalf("bad handshake accepted/leaked: %+v", h)
			}
		})
	}
	if redirected.Load() != 0 {
		t.Fatal("followed redirect")
	}
}

func TestWebSocketTLSValidationAndPinnedSNI(t *testing.T) {
	seen := make(chan string, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("untrusted TLS reached HTTP handler") }))
	server.TLS = &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) { seen <- hello.ServerName; return nil, nil }}
	server.StartTLS()
	defer server.Close()
	u, _ := url.Parse(server.URL)
	m := wsMonitor("https://secure.example:"+u.Port()+"/ws", "handshake")
	m.WebSocket.ConnectIP = "127.0.0.1"
	h := Probe(context.Background(), m, "")
	if h.Status != "down" || !strings.Contains(h.Message, "certificate") {
		t.Fatalf("untrusted TLS accepted: %+v", h)
	}
	if sni := <-seen; sni != "secure.example" {
		t.Fatalf("pinned IP changed SNI: %q", sni)
	}
}

func TestWebSocketTrustedTLS(t *testing.T) {
	// 信任根是进程级设置，子进程验证受信任 WSS，避免影响其他证书拒绝测试。
	if os.Getenv("UPTIMENEST_WS_TRUST_TEST") != "1" {
		command := exec.Command(os.Args[0], "-test.run=^TestWebSocketTrustedTLS$", "-test.timeout=15s")
		command.Env = append(os.Environ(), "UPTIMENEST_WS_TRUST_TEST=1", "GODEBUG=x509usefallbackroots=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("trusted TLS: %v\n%s", err, output)
		}
		return
	}
	certificate, _ := connectIPCertificate(t)
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	x509.SetFallbackRoots(pool)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil || host != "monitor.test" || r.TLS.ServerName != "monitor.test" {
			t.Error("hostname/SNI lost")
		}
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err == nil {
			readWebSocket(conn)
		}
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
	server.StartTLS()
	defer server.Close()
	u, _ := url.Parse(server.URL)
	m := wsMonitor("https://monitor.test:"+u.Port()+"/ws", "ping")
	m.WebSocket.ConnectIP = "127.0.0.1"
	h := Probe(context.Background(), m, "")
	if h.Status != "up" || h.TLSVersion == "" || h.TLSLatency <= 0 || h.Latency < h.TLSLatency || h.CertExpiry <= time.Now().UnixMilli() || h.TLSCipherSuite == "" {
		t.Fatalf("TLS metadata missing: %+v", h)
	}
	m.Target = "wss://wrong.test:" + u.Port() + "/ws"
	if h := Probe(context.Background(), m, ""); h.Status != "down" {
		t.Fatalf("trusted certificate bypassed hostname verification: %+v", h)
	}
}

func TestWebSocketEarlyCloseAndLargeMessage(t *testing.T) {
	for _, large := range []bool{false, true} {
		t.Run(map[bool]string{false: "close", true: "large"}[large], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.Close()
				if large {
					_ = conn.WriteMessage(websocket.TextMessage, []byte(strings.Repeat("s", 128<<10)))
				} else {
					_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(1000, "server-private-secret"), time.Now().Add(time.Second))
				}
			}))
			defer server.Close()
			h := Probe(context.Background(), wsMonitor(server.URL, "ping"), "")
			if h.Status != "down" || strings.Contains(h.Message, "server-private-secret") {
				t.Fatalf("bad message accepted/leaked: %+v", h)
			}
		})
	}
}

func TestWebSocketPinnedIPv6(t *testing.T) {
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("IPv6 loopback unavailable")
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err == nil {
			readWebSocket(conn)
		}
	}))
	server.Listener.Close()
	server.Listener = listener
	server.Start()
	defer server.Close()
	u, _ := url.Parse(server.URL)
	m := wsMonitor("http://ipv6.example:"+u.Port(), "ping")
	m.WebSocket.ConnectIP = "::1"
	if h := Probe(context.Background(), m, ""); h.Status != "up" {
		t.Fatalf("IPv6 failed: %+v", h)
	}
}
