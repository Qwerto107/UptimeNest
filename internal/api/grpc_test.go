package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/config"
	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/Qwerto107/UptimeNest/internal/monitor"
	"github.com/Qwerto107/UptimeNest/internal/store"
)

func TestGRPCMonitorSaveAndPrivacy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := store.Open(ctx, "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	engine := monitor.New(ctx, s, 1)
	token := strings.Repeat("g", 64)
	if _, err = s.DB.Exec("INSERT INTO sessions(token,expires) VALUES(?,?)", tokenHash(token), time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	handler := New(s, engine, config.Config{})
	call := func(method, path string, body any, auth bool) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		if auth {
			r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	m := model.Monitor{Name: "gRPC", Type: "grpc", Target: "service.example", Port: 443, Interval: 60, Timeout: 2, RetryInterval: 1, HTTP: &model.HTTPOptions{}, WebSocket: &model.WebSocketOptions{}, GRPC: &model.GRPCOptions{ServiceName: " private.Service ", Metadata: `{"Authorization":"Bearer private-token"}`}}
	if w := call("POST", "/api/admin/monitors", m, false); w.Code != 401 {
		t.Fatalf("unauthenticated save: %d", w.Code)
	}
	secretMetadata := m.GRPC.Metadata
	w := call("POST", "/api/admin/monitors", m, true)
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &m) != nil {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	// 创建响应按脱敏规则返回，不含凭据明文，并通过 configured 标明已配置。
	if strings.Contains(w.Body.String(), "private-token") || !strings.Contains(w.Body.String(), "grpc.metadata") {
		t.Fatalf("create response not redacted: %s", w.Body.String())
	}
	stored, err := s.Monitor(ctx, m.ID)
	if err != nil || stored.GRPC == nil || stored.GRPC.Security != "tls" || stored.GRPC.ServiceName != "private.Service" || stored.GRPC.Metadata != secretMetadata || stored.HTTP != nil || stored.WebSocket != nil {
		t.Fatalf("bad persisted configuration: %+v %v", stored, err)
	}
	if _, err := s.SavePanel(ctx, model.Panel{Name: "public", Slug: "grpc", MonitorIDs: []int64{m.ID}}); err != nil {
		t.Fatal(err)
	}
	w = call("GET", "/api/public/panels/grpc", nil, false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "service.example:443") || !strings.Contains(w.Body.String(), `"protocol":"gRPC"`) {
		t.Fatalf("public endpoint: %d %s", w.Code, w.Body.String())
	}
	for _, secret := range []string{"private.Service", "private-token", "metadata", "caCert", `"grpc":{`} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("public data leaked %q", secret)
		}
	}
	m.HideAddress = true
	if w := call("PUT", fmt.Sprintf("/api/admin/monitors/%d", m.ID), m, true); w.Code != 200 {
		t.Fatalf("hide address: %s", w.Body.String())
	}
	if w := call("GET", "/api/public/panels/grpc", nil, false); strings.Contains(w.Body.String(), "service.example") {
		t.Fatal("hidden gRPC host leaked")
	}
	m.Type = "tcp"
	if err := validateMonitor(&m); err != nil || m.GRPC != nil {
		t.Fatalf("stale gRPC configuration was retained: %+v %v", m, err)
	}
}

func TestGRPCCertificateAlertEligibilityAndPublicFailure(t *testing.T) {
	m := model.Monitor{Name: "gRPC", Type: "grpc", Target: "::1", Port: 443, Interval: 60, Timeout: 2, RetryInterval: 1, Alerts: &model.AlertOptions{Certificate: model.AlertRule{Enabled: true, ChannelIDs: []int64{1}, Days: 7}}}
	if err := validateMonitor(&m); err != nil {
		t.Fatalf("TLS default should allow certificate alerts: %v", err)
	}
	m.GRPC.Security = "none"
	if err := validateMonitor(&m); err == nil {
		t.Fatal("plaintext accepted certificate alerts")
	}
	for _, message := range []string{"gRPC 健康状态：NOT_SERVING", "gRPC 健康状态：UNKNOWN", "gRPC 健康状态：SERVICE_UNKNOWN"} {
		if failure, code := publicFailure(model.Heartbeat{Status: "down", Message: message}); failure != "response" || code != 0 {
			t.Fatalf("lost unhealthy response marker: %s", message)
		}
	}
}
