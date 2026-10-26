package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/config"
	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/Qwerto107/UptimeNest/internal/store"
)

func TestTLSLatencyWarningSettings(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	token := strings.Repeat("t", 64)
	if _, err := s.DB.Exec("INSERT INTO sessions(token,expires) VALUES(?,?)", tokenHash(token), time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	handler := New(s, nil, config.Config{})
	call := func(method, path string, body []byte, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if auth {
			r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	settings := model.DefaultSettings()
	settings.UserAgent = "private-user-agent"
	for _, threshold := range []int{0, 500, 120000} {
		settings.TLSLatencyWarningMS = threshold
		body, err := json.Marshal(settings)
		if err != nil {
			t.Fatal(err)
		}
		if w := call("PUT", "/api/admin/settings", body, false); w.Code != 401 {
			t.Fatalf("unauthorized settings status %d", w.Code)
		}
		if w := call("PUT", "/api/admin/settings", body, true); w.Code != 200 {
			t.Fatalf("save threshold %d: %d %s", threshold, w.Code, w.Body.String())
		}
		stored, err := s.Settings(ctx)
		if err != nil || stored.TLSLatencyWarningMS != threshold {
			t.Fatalf("threshold not persisted: %+v %v", stored, err)
		}
		w := call("GET", "/api/public/bootstrap", nil, false)
		var value struct {
			TLSLatencyWarningMS int `json:"tlsLatencyWarningMs"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &value) != nil || value.TLSLatencyWarningMS != threshold || strings.Contains(w.Body.String(), settings.UserAgent) {
			t.Fatalf("invalid public display settings: %s", w.Body.String())
		}
	}
	for _, threshold := range []int{-1, 120001} {
		settings.TLSLatencyWarningMS = threshold
		body, _ := json.Marshal(settings)
		if w := call("PUT", "/api/admin/settings", body, true); w.Code != 400 {
			t.Fatalf("invalid threshold %d status %d", threshold, w.Code)
		}
	}
	settings.TLSLatencyWarningMS = 500
	body, _ := json.Marshal(settings)
	body = bytes.Replace(body, []byte(`"tlsLatencyWarningMs":500`), []byte(`"tlsLatencyWarningMs":1.5`), 1)
	if w := call("PUT", "/api/admin/settings", body, true); w.Code != 400 {
		t.Fatalf("fractional threshold status %d", w.Code)
	}
	// 旧设置缺少新字段时保持关闭，不需要迁移或重写用户配置。
	legacy, _ := json.Marshal(model.DefaultSettings())
	legacy = bytes.Replace(legacy, []byte(`"tlsLatencyWarningMs":0,`), nil, 1)
	if _, err := s.DB.Exec("UPDATE settings SET value=? WHERE key='system'", string(legacy)); err != nil {
		t.Fatal(err)
	}
	stored, err := s.Settings(ctx)
	if err != nil || stored.TLSLatencyWarningMS != 0 {
		t.Fatalf("legacy settings default: %+v %v", stored, err)
	}
}
