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

func TestWebSocketMonitorSaveAndPrivacy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := store.Open(ctx, "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	engine := monitor.New(ctx, s, 1)
	defer engine.Wait()
	token := strings.Repeat("w", 64)
	if _, err := s.DB.Exec("INSERT INTO sessions(token,expires) VALUES(?,?)", tokenHash(token), time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	handler := New(s, engine, config.Config{})
	call := func(method, path string, body any, auth bool) *httptest.ResponseRecorder {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(encoded))
		r.Header.Set("Content-Type", "application/json")
		if auth {
			r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	m := model.Monitor{Name: "ws", Type: "websocket", Target: "wss://service.example:8443/private-path?token=private-query", Interval: 60, Timeout: 2, RetryInterval: 1, HTTP: &model.HTTPOptions{}, WebSocket: &model.WebSocketOptions{Headers: `{"Authorization":"Bearer private-header"}`, Origin: "https://private-origin.example", ConnectIP: " 127.0.0.1 ", Subprotocols: []string{"private-protocol"}}}
	if w := call("POST", "/api/admin/monitors", m, false); w.Code != 401 {
		t.Fatalf("unauthenticated save status %d", w.Code)
	}
	w := call("POST", "/api/admin/monitors", m, true)
	if w.Code != 201 {
		t.Fatalf("save status %d: %s", w.Code, w.Body.String())
	}
	m = model.Monitor{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m.WebSocket == nil || m.WebSocket.Mode != "handshake" || m.WebSocket.ConnectIP != "127.0.0.1" || m.HTTP != nil {
		t.Fatalf("bad normalized config: %+v", m)
	}
	stored, err := s.Monitor(ctx, m.ID)
	if err != nil || stored.WebSocket == nil || stored.WebSocket.Headers != m.WebSocket.Headers || stored.WebSocket.Subprotocols[0] != "private-protocol" {
		t.Fatalf("config not persisted: %+v, %v", stored, err)
	}
	if _, err := s.SavePanel(ctx, model.Panel{Name: "public", Slug: "ws", MonitorIDs: []int64{m.ID}}); err != nil {
		t.Fatal(err)
	}
	w = call("GET", "/api/public/panels/ws", nil, false)
	if w.Code != 200 {
		t.Fatalf("public status %d: %s", w.Code, w.Body.String())
	}
	for _, secret := range []string{"private-path", "private-query", "private-header", "private-origin", "private-protocol", "127.0.0.1", `"websocket":{`} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("public response leaked %q", secret)
		}
	}
	var public struct {
		Monitors []publicMonitor `json:"monitors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &public); err != nil {
		t.Fatal(err)
	}
	if len(public.Monitors) != 1 || public.Monitors[0].Address != "service.example:8443" || public.Monitors[0].Protocol != "WSS" {
		t.Fatalf("wrong public endpoint: %+v", public)
	}
	m.HideAddress = true
	m.WebSocket.Mode = "ping"
	w = call("PUT", fmt.Sprintf("/api/admin/monitors/%d", m.ID), m, true)
	if w.Code != 200 {
		t.Fatalf("update failed: %s", w.Body.String())
	}
	w = call("GET", "/api/public/panels/ws", nil, false)
	if w.Code != 200 || strings.Contains(w.Body.String(), "service.example") {
		t.Fatalf("hidden host leaked: %s", w.Body.String())
	}
	// 改为其他类型时清理不适用的专属配置，旧的四种类型仍能正常保存。
	for _, kind := range []string{"http", "tcp", "ping", "dns"} {
		next := m
		next.Type = kind
		next.Target = "example.com"
		next.Port = 443
		next.DNSRecord = "A"
		if kind == "http" {
			next.Target = "https://example.com"
		}
		if err := validateMonitor(&next); err != nil || next.WebSocket != nil {
			t.Fatalf("old type %s: %+v, %v", kind, next, err)
		}
	}
}
