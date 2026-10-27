package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/config"
	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/Qwerto107/UptimeNest/internal/monitor"
	"github.com/Qwerto107/UptimeNest/internal/store"
)

func TestNotificationAPISecretsPreviewAndAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := store.Open(ctx, "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	engine := monitor.New(ctx, s, 1)
	token := strings.Repeat("n", 64)
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
	for _, path := range []string{"/api/admin/notification-channels", "/api/admin/notifications/records", "/api/admin/notifications/templates"} {
		if w := call("GET", path, nil, false); w.Code != 401 {
			t.Fatalf("unprotected: %s %d", path, w.Code)
		}
	}
	c := model.NotificationChannel{Name: "hook", Type: "webhook", Enabled: true, Config: model.NotificationConfig{URL: "https://example.com/hook?token=private-url", Headers: map[string]string{"Authorization": "Bearer private-header"}, Secret: "private-secret"}}
	w := call("POST", "/api/admin/notification-channels", c, true)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	metadata := call("GET", "/api/admin/notifications/templates", nil, true)
	var templates struct {
		Channels map[string]struct{ Subject, Body, Format string } `json:"channels"`
	}
	if metadata.Code != 200 || json.Unmarshal(metadata.Body.Bytes(), &templates) != nil || len(templates.Channels) != 5 || templates.Channels["feishu"].Format != "card" || templates.Channels["webhook"].Format != "json" {
		t.Fatalf("missing channel templates: %s", metadata.Body.String())
	}
	var view channelView
	if err = json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.ID != 1 || len(view.Configured) != 3 {
		t.Fatalf("bad view: %+v", view)
	}
	if view.Config.To == nil || view.Config.Headers == nil {
		t.Fatal("channel collections should not be null in the editor")
	}
	w = call("GET", "/api/admin/notification-channels", nil, true)
	for _, secret := range []string{"private-url", "private-header", "private-secret"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("secret leaked: %s", secret)
		}
	}
	view.Name = "renamed"
	w = call("PUT", "/api/admin/notification-channels/1", view, true)
	if w.Code != 200 {
		t.Fatalf("edit redacted: %d %s", w.Code, w.Body.String())
	}
	stored, err := s.NotificationChannel(ctx, 1)
	if err != nil || stored.Config.URL != c.Config.URL || stored.Config.Headers["Authorization"] != "Bearer private-header" {
		t.Fatalf("credentials lost: %+v %v", stored, err)
	}
	w = call("PUT", "/api/admin/notification-channels/1", struct {
		channelView
		ClearSecrets []string `json:"clearSecrets"`
	}{view, []string{"secret"}}, true)
	if w.Code != 200 {
		t.Fatalf("clear: %d %s", w.Code, w.Body.String())
	}
	stored, _ = s.NotificationChannel(ctx, 1)
	if stored.Config.Secret != "" {
		t.Fatal("optional secret not cleared")
	}
	w = call("POST", "/api/admin/notifications/preview", map[string]string{"type": "webhook", "subject": "{{alert_type}}", "body": `{"error":"{{error_message}}"}`, "alertType": "service_down"}, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "service_down") {
		t.Fatalf("preview: %d %s", w.Code, w.Body.String())
	}
	w = call("POST", "/api/admin/notification-channels/1/test", nil, true)
	if w.Code != 202 {
		t.Fatalf("test queue: %d %s", w.Code, w.Body.String())
	}
	w = call("POST", "/api/admin/notification-channels/1/test", nil, true)
	if w.Code != 400 {
		t.Fatalf("duplicate tests not bounded: %d", w.Code)
	}
	w = call("GET", "/api/public/bootstrap", nil, false)
	if strings.Contains(w.Body.String(), "notification") || strings.Contains(w.Body.String(), "private-") {
		t.Fatal("public configuration leak")
	}
}

func TestMonitorAlertValidation(t *testing.T) {
	base := model.Monitor{Name: "test", Type: "http", Target: "https://example.com", Interval: 60, Timeout: 10, RetryInterval: 1}
	for _, rule := range []model.AlertRule{{Enabled: true}, {Enabled: true, ChannelIDs: []int64{1}, Days: 0}, {Enabled: true, ChannelIDs: []int64{1}, Days: 7, RepeatMinutes: -1}} {
		m := base
		m.Alerts = &model.AlertOptions{Certificate: rule}
		if err := validateMonitor(&m); err == nil {
			t.Fatalf("invalid rule accepted: %+v", rule)
		}
	}
	base.Target = "http://example.com"
	base.Alerts = &model.AlertOptions{Certificate: model.AlertRule{Enabled: true, ChannelIDs: []int64{1}, Days: 7}}
	if err := validateMonitor(&base); err == nil {
		t.Fatal("certificate alert on plaintext monitor")
	}
	base.Target = "https://example.com"
	base.Alerts.Certificate.Enabled = false
	for _, count := range []int{-1, 1001} {
		base.Alerts.Service.FailureThreshold = count
		if err := validateMonitor(&base); err == nil {
			t.Fatalf("invalid consecutive count accepted: %d", count)
		}
	}
}

func TestNotificationDraftSendUsesUnsavedConfigAndPreservesChannel(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	token := strings.Repeat("d", 64)
	if _, err = s.DB.Exec("INSERT INTO sessions(token,expires) VALUES(?,?)", tokenHash(token), time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	var reject atomic.Bool
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != "Bearer retained" {
			t.Error("test lost retained credentials")
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload["event"] != "test" || payload["source"] != "unsaved-template" {
			t.Errorf("wrong template payload: %+v %v", payload, err)
		}
		if reject.Load() {
			w.WriteHeader(503)
		}
	}))
	defer receiver.Close()
	handler := New(s, nil, config.Config{})
	call := func(c any, authenticated bool) *httptest.ResponseRecorder {
		data, _ := json.Marshal(c)
		r := httptest.NewRequest("POST", "/api/admin/notification-channels/test", bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		if authenticated {
			r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	draft := model.NotificationChannel{Name: "draft", Type: "webhook", Body: `{"event":"{{alert_type}}","source":"unsaved-template"}`, Config: model.NotificationConfig{URL: receiver.URL + "?key=private-key", Headers: map[string]string{"Authorization": "Bearer retained"}}}
	if w := call(draft, false); w.Code != 401 || requests.Load() != 0 {
		t.Fatal("draft test was not protected")
	}
	if w := call(draft, true); w.Code != 200 || !strings.Contains(w.Body.String(), `"sent":true`) {
		t.Fatalf("draft test: %d %s", w.Code, w.Body.String())
	}
	if channels, err := s.NotificationChannels(ctx); err != nil || len(channels) != 0 {
		t.Fatalf("test saved a draft channel: %+v %v", channels, err)
	}
	// 已保存渠道留空的凭据继续沿用，但本次测试的模板和名称不写回配置。
	draft.Body = `{"original":true}`
	saved, err := s.SaveNotificationChannel(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	view := redactChannel(saved)
	view.Name = "not saved"
	view.Body = `{"event":"{{alert_type}}","source":"unsaved-template"}`
	if w := call(view, true); w.Code != 200 {
		t.Fatalf("retained secret test: %d %s", w.Code, w.Body.String())
	}
	stored, err := s.NotificationChannel(ctx, saved.ID)
	if err != nil || stored.Name != saved.Name || stored.Body != saved.Body {
		t.Fatalf("draft test changed channel: %+v %v", stored, err)
	}
	reject.Store(true)
	w := call(view, true)
	if w.Code != 502 || strings.Contains(w.Body.String(), "private-key") || strings.Contains(w.Body.String(), receiver.URL) {
		t.Fatalf("test error leaked URL: %d %s", w.Code, w.Body.String())
	}
}
