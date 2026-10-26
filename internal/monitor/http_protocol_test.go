package monitor

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

func TestHTTPProtocolValidationAndDefaults(t *testing.T) {
	for _, mode := range []string{"http1", "http2", "http3", "standard", "auto", ""} {
		m := model.Monitor{Target: "https://monitor.test", HTTP: &model.HTTPOptions{Version: mode}}
		if err := ValidateHTTP(&m); err != nil {
			t.Fatalf("协议 %q 被拒绝：%v", mode, err)
		}
		want := mode
		if want == "" {
			want = "auto"
		}
		if m.HTTP.Version != want {
			t.Fatalf("协议 = %q, want %q", m.HTTP.Version, want)
		}
	}
	if got := (model.Monitor{}).HTTPConfig().Version; got != "auto" {
		t.Fatalf("缺少 HTTP 配置时默认协议 = %q", got)
	}
	for _, mode := range []string{"invalid", "http3"} {
		m := model.Monitor{Target: "http://monitor.test", HTTP: &model.HTTPOptions{Version: mode}}
		if err := ValidateHTTP(&m); err == nil {
			t.Fatalf("无效模式或不适用的目标被接受：%q", mode)
		}
	}
}

func TestHTTPProtocolStrictnessAndFallback(t *testing.T) {
	unavailableProxy(t)
	certificate, auth := connectIPCertificate(t)
	for _, mode := range []struct {
		version string
		success bool
	}{
		{"http1", true}, {"http2", false}, {"http3", false}, {"standard", true}, {"auto", true},
	} {
		t.Run(mode.version, func(t *testing.T) {
			requests := make(chan string, 2)
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.Proto
			}))
			// 模拟未发送 ALPN 的旧 TLS 服务，验证手动 HTTP/2 不会降级发送 HTTP/1.1。
			withoutALPN := &tls.Config{Certificates: []tls.Certificate{certificate}}
			server.TLS = &tls.Config{GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
				return withoutALPN, nil
			}}
			server.StartTLS()
			defer server.Close()
			m := connectIPMonitor(t, server.URL, "127.0.0.1", mode.version, auth)
			m.HTTP.Method = http.MethodPost
			m.HTTP.Body = `{"probe":true}`
			beat := &model.Heartbeat{}
			err := probeHTTP(context.Background(), m, "", beat)
			if (err == nil) != mode.success {
				t.Fatalf("检测结果 = %v, success = %v", err, mode.success)
			}
			if !mode.success {
				if len(requests) != 0 {
					t.Fatal("手动模式降级发送了请求")
				}
				return
			}
			if len(requests) != 1 || beat.HTTPVersion != "HTTP/1.1" {
				t.Fatalf("回退协议错误或 POST 被重复发送：%s, %d 次", beat.HTTPVersion, len(requests))
			}
		})
	}
}

func TestHTTP2Cleartext(t *testing.T) {
	unavailableProxy(t)
	requests := make(chan int, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.ProtoMajor
	}))
	server.Config.Protocols = new(http.Protocols)
	server.Config.Protocols.SetUnencryptedHTTP2(true)
	server.Start()
	defer server.Close()
	m := connectIPMonitor(t, server.URL, "127.0.0.1", "http2", model.HTTPAuth{})
	beat := &model.Heartbeat{}
	if err := probeHTTP(context.Background(), m, "", beat); err != nil {
		t.Fatal(err)
	}
	if got := <-requests; got != 2 || beat.HTTPVersion != "HTTP/2.0" {
		t.Fatalf("明文 HTTP/2 未生效：%d, %s", got, beat.HTTPVersion)
	}
}
