package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/config"
	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/Qwerto107/UptimeNest/internal/store"
)

func TestSplitTemplatePersistenceAndDraftDelivery(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	token := strings.Repeat("s", 64)
	if _, err := s.DB.Exec("INSERT INTO sessions(token,expires) VALUES(?,?)", tokenHash(token), time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	deliveries := make(chan map[string]string, 5)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var received map[string]string
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		deliveries <- received
		w.WriteHeader(200)
	}))
	defer receiver.Close()
	handler := New(s, nil, config.Config{})
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	c := model.NotificationChannel{Name: "split", Type: "webhook", Body: `{"kind":"service","event":"{{alert_type}}"}`, CertificateTemplate: &model.NotificationTemplate{Body: `{"kind":"certificate","event":"{{alert_type}}"}`}, Config: model.NotificationConfig{URL: receiver.URL}}
	w := call("POST", "/api/admin/notification-channels", c)
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &c) != nil {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	stored, err := s.NotificationChannel(ctx, c.ID)
	if err != nil || stored.Body != c.Body || stored.CertificateTemplate == nil || stored.CertificateTemplate.Body != c.CertificateTemplate.Body {
		t.Fatalf("templates not persisted: %+v %v", stored, err)
	}
	for _, event := range []string{"service_down", "service_recovered", "certificate_expiring", "certificate_expired", "certificate_recovered"} {
		w = call("POST", "/api/admin/notification-channels/test", map[string]any{"name": c.Name, "type": c.Type, "body": c.Body, "certificateTemplate": c.CertificateTemplate, "config": stored.Config, "alertType": event})
		if w.Code != 200 {
			t.Fatalf("draft test: %d %s", w.Code, w.Body.String())
		}
		received := <-deliveries
		kind := "service"
		if strings.HasPrefix(event, "certificate_") {
			kind = "certificate"
		}
		if w.Code != 200 || received["kind"] != kind || received["event"] != event {
			t.Fatalf("wrong delivery for %s: %d %+v %s", event, w.Code, received, w.Body.String())
		}
	}
	// 较旧客户端省略新字段时，不能覆盖已经独立配置的证书模板。
	c.CertificateTemplate = nil
	c.Body = `{"changed":true}`
	w = call("PUT", "/api/admin/notification-channels/"+strconv.FormatInt(c.ID, 10), c)
	updated, err := s.NotificationChannel(ctx, c.ID)
	if w.Code != 200 || err != nil || updated.CertificateTemplate == nil || updated.CertificateTemplate.Body != stored.CertificateTemplate.Body {
		t.Fatalf("legacy update lost certificate template: %d %+v %v", w.Code, updated, err)
	}
	// 草稿测试不会创建额外渠道，也不会改写已保存模板。
	channels, err := s.NotificationChannels(ctx)
	if err != nil || len(channels) != 1 {
		t.Fatalf("draft was persisted: %d %v", len(channels), err)
	}
}
