package monitor

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/quic-go/quic-go/http3"
)

func TestValidateHTTPConnectIP(t *testing.T) {
	for _, value := range []struct {
		input, want string
		valid       bool
	}{
		{"", "", true}, {"  ", "", true}, {" 192.0.2.10 ", "192.0.2.10", true},
		{"2001:0db8::1", "2001:db8::1", true}, {"::1", "::1", true},
		{"example.com", "", false}, {"192.0.2.1:443", "", false},
		{"[::1]", "", false}, {"[::1]:443", "", false}, {"fe80::1%eth0", "", false},
		{"192.0.2.1,192.0.2.2", "", false}, {"999.1.1.1", "", false},
	} {
		t.Run(value.input, func(t *testing.T) {
			m := model.Monitor{Target: "https://monitor.test", HTTP: &model.HTTPOptions{ConnectIP: value.input}}
			err := ValidateHTTP(&m)
			if (err == nil) != value.valid {
				t.Fatalf("ValidateHTTP() = %v, valid = %v", err, value.valid)
			}
			if err == nil && m.HTTP.ConnectIP != value.want {
				t.Fatalf("ConnectIP = %q, want %q", m.HTTP.ConnectIP, value.want)
			}
		})
	}
	m := model.Monitor{Target: "https://monitor.test"}
	if err := ValidateHTTP(&m); err != nil || m.HTTP.ConnectIP != "" {
		t.Fatalf("旧配置默认值不兼容：%v", err)
	}
}

func TestHTTPConnectAddress(t *testing.T) {
	for _, value := range []struct{ ip, address, want string }{
		{"", "monitor.test:443", "monitor.test:443"},
		{"192.0.2.10", "MONITOR.test:8443", "192.0.2.10:8443"},
		{"2001:db8::1", "monitor.test:443", "[2001:db8::1]:443"},
		{"192.0.2.10", "other.test:443", "other.test:443"},
		{"192.0.2.10", "monitor.test.evil:443", "monitor.test.evil:443"},
		{"192.0.2.10", "invalid", "invalid"},
	} {
		transport := &probeTransport{host: "monitor.test", ip: value.ip}
		if got := transport.connectAddress(value.address); got != value.want {
			t.Errorf("connectAddress(%q, %q) = %q, want %q", value.ip, value.address, got, value.want)
		}
	}
	for _, host := range []string{"bücher.example", "xn--bcher-kva.example"} {
		transport := &probeTransport{host: host, ip: "192.0.2.10"}
		if got := transport.connectAddress("xn--bcher-kva.example:443"); got != "192.0.2.10:443" {
			t.Errorf("国际化域名未覆盖连接地址：%q", got)
		}
		if !transport.matchesHost("bücher.example") || transport.matchesHost("other.example") {
			t.Fatal("国际化域名匹配边界错误")
		}
	}
}

// 测试固定一个不可用的本机代理，确保指定 IP 的请求确实直连，且不访问外部网络。
func unavailableProxy(t *testing.T) {
	t.Helper()
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("NO_PROXY", "")
}

func TestHTTPConnectIPProxy(t *testing.T) {
	unavailableProxy(t)
	for _, value := range []struct {
		ip, target string
		proxied    bool
	}{
		{"192.0.2.10", "https://monitor.test:8443", false},
		{"192.0.2.10", "http://MONITOR.test", false},
		{"192.0.2.10", "https://other.test", true},
		{"", "https://monitor.test", true},
	} {
		req, err := http.NewRequest(http.MethodGet, value.target, nil)
		if err != nil {
			t.Fatal(err)
		}
		proxy, err := (&probeTransport{host: "monitor.test", ip: value.ip}).proxy(req)
		if err != nil || (proxy != nil) != value.proxied {
			t.Errorf("proxy(%q, %q) = %v, %v", value.ip, value.target, proxy, err)
		}
	}
}

func connectIPMonitor(t *testing.T, address, ip, version string, auth model.HTTPAuth) model.Monitor {
	t.Helper()
	target, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	target.Host = net.JoinHostPort("monitor.test", target.Port())
	m := model.Monitor{Target: target.String(), Timeout: 5, HTTP: &model.HTTPOptions{
		ConnectIP: ip, Version: version, Auth: auth,
	}}
	if err := ValidateHTTP(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestHTTPConnectIPHostAndRedirect(t *testing.T) {
	unavailableProxy(t)
	requests := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Host + r.URL.Path
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
		}
	}))
	defer server.Close()
	m := connectIPMonitor(t, server.URL+"/start", "127.0.0.1", "standard", model.HTTPAuth{})
	if err := probeHTTP(context.Background(), m, "", &model.Heartbeat{}); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(m.Target)
	for _, path := range []string{"/start", "/final"} {
		if got := <-requests; got != u.Host+path {
			t.Fatalf("请求主机或路径改变：%q", got)
		}
	}
	// 自定义 Host 仅控制请求主机，不影响连接域名匹配。
	m.Target = strings.TrimSuffix(m.Target, "/start")
	m.HTTP.Headers = `{"Host":"other.test"}`
	if err := probeHTTP(context.Background(), m, "", &model.Heartbeat{}); err != nil {
		t.Fatal(err)
	}
	if got := <-requests; got != "other.test/" {
		t.Fatalf("自定义 Host 未保留：%q", got)
	}
	m.Target = strings.Replace(m.Target, "monitor.test", "bücher.example", 1)
	m.HTTP.Headers = ""
	if err := probeHTTP(context.Background(), m, "", &model.Heartbeat{}); err != nil {
		t.Fatal(err)
	}
	if got, want := <-requests, net.JoinHostPort("xn--bcher-kva.example", u.Port())+"/"; got != want {
		t.Fatalf("国际化域名请求主机 = %q, want %q", got, want)
	}
}

func TestHTTPConnectIPCrossHostRedirect(t *testing.T) {
	unavailableProxy(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://other.test/health", http.StatusFound)
	}))
	defer server.Close()
	m := connectIPMonitor(t, server.URL, "127.0.0.1", "standard", model.HTTPAuth{})
	err := probeHTTP(context.Background(), m, "", &model.Heartbeat{})
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Fatalf("跨域请求未使用原环境代理：%v", err)
	}
}

// 自签证书仅在测试监控的附加 CA 中受信任，不关闭证书链或主机名校验。
func connectIPCertificate(t *testing.T) (tls.Certificate, model.HTTPAuth) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1), DNSNames: []string{"monitor.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage:    x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	pair, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		t.Fatal(err)
	}
	return pair, model.HTTPAuth{Type: "mtls", ClientCert: certPEM, ClientKey: keyPEM, CACert: certPEM}
}

func TestHTTPSConnectIP(t *testing.T) {
	unavailableProxy(t)
	certificate, auth := connectIPCertificate(t)
	for _, mode := range []struct {
		name, version, protocol string
		quic                    bool
	}{
		{"http1", "http1", "HTTP/1.1", false},
		{"http2", "http2", "HTTP/2.0", false},
		{"standard", "standard", "HTTP/2.0", false},
		{"auto-fallback", "auto", "HTTP/2.0", false},
		{"auto-http3", "auto", "HTTP/3.0", true},
		{"http3", "http3", "HTTP/3.0", true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			requests := make(chan string, 2)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.Host + " " + r.TLS.ServerName + " " + r.Proto
			})
			server := httptest.NewUnstartedServer(handler)
			server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
			server.EnableHTTP2 = true
			server.StartTLS()
			defer server.Close()
			if mode.quic {
				packet, err := net.ListenPacket("udp", server.Listener.Addr().String())
				if err != nil {
					t.Fatal(err)
				}
				quicServer := &http3.Server{TLSConfig: server.TLS.Clone(), Handler: handler}
				done := make(chan struct{})
				go func() { defer close(done); _ = quicServer.Serve(packet) }()
				defer func() { _ = quicServer.Close(); _ = packet.Close(); <-done }()
			}
			m := connectIPMonitor(t, server.URL, "127.0.0.1", mode.version, auth)
			m.HTTP.Method = http.MethodPost
			m.HTTP.Body = `{"probe":true}`
			beat := &model.Heartbeat{}
			if err := probeHTTP(context.Background(), m, "", beat); err != nil {
				t.Fatal(err)
			}
			u, _ := url.Parse(m.Target)
			if got, want := <-requests, u.Host+" monitor.test "+mode.protocol; got != want {
				t.Fatalf("Host / SNI / 协议 = %q, want %q", got, want)
			}
			if beat.HTTPVersion != mode.protocol || beat.CertExpiry == 0 || len(requests) != 0 {
				t.Fatalf("连接信息无效或请求被重复发送：%+v", beat)
			}
			// 指定 IP 不应绕过域名校验，即使连接地址相同也必须拒绝错误证书。
			m.Target = strings.Replace(m.Target, "monitor.test", "wrong.test", 1)
			if err := probeHTTP(context.Background(), m, "", &model.Heartbeat{}); err == nil || !strings.Contains(err.Error(), "certificate") {
				t.Fatalf("错误证书主机名未被拒绝：%v", err)
			}
		})
	}
}

func TestHTTPConnectIPv6(t *testing.T) {
	unavailableProxy(t)
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("环境不支持 IPv6 回环：%v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	defer server.Close()
	m := connectIPMonitor(t, server.URL, "::1", "standard", model.HTTPAuth{})
	if err := probeHTTP(context.Background(), m, "", &model.Heartbeat{}); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPConnectIPOAuth2Isolation(t *testing.T) {
	unavailableProxy(t)
	tokens := make(chan struct{}, 2)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens <- struct{}{}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"access_token":"fixture-token","token_type":"Bearer"}`)
	}))
	defer tokenServer.Close()
	requests := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Header.Get("Authorization")
	}))
	defer server.Close()
	auth := model.HTTPAuth{Type: "oauth2", TokenURL: tokenServer.URL, ClientID: "fixture", ClientSecret: "fixture"}
	m := connectIPMonitor(t, server.URL, "127.0.0.1", "standard", auth)
	if err := probeHTTP(context.Background(), m, "", &model.Heartbeat{}); err != nil {
		t.Fatal(err)
	}
	<-tokens
	if got := <-requests; got != "Bearer fixture-token" {
		t.Fatalf("OAuth2 认证未送达目标：%q", got)
	}
	// Token URL 即使用了相同域名，也不能继承检测目标的 IP 覆盖或代理豁免。
	u, _ := url.Parse(tokenServer.URL)
	u.Host = net.JoinHostPort("monitor.test", u.Port())
	m.HTTP.Auth.TokenURL = u.String()
	if err := probeHTTP(context.Background(), m, "", &model.Heartbeat{}); err == nil {
		t.Fatal("OAuth2 令牌请求错误地继承了指定 IP")
	}
	if len(tokens) != 0 || len(requests) != 0 {
		t.Fatal("失败的令牌请求仍访问了目标")
	}
}
