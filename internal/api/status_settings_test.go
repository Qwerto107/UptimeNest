package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	legacyStore := legacySettingsStore(t, legacy)
	stored, err := legacyStore.Settings(ctx)
	if err != nil || stored.TLSLatencyWarningMS != 0 {
		t.Fatalf("legacy settings default: %+v %v", stored, err)
	}
}

func TestStatusPageViewSettings(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	token := strings.Repeat("v", 64)
	if _, err := s.DB.Exec("INSERT INTO sessions(token,expires) VALUES(?,?)", tokenHash(token), time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	handler := New(s, nil, config.Config{})
	for _, view := range []string{"compact", "cards", "detailed", "", "unknown"} {
		settings := model.DefaultSettings()
		settings.StatusPageView = view
		settings.UserAgent = "private-user-agent"
		body, _ := json.Marshal(settings)
		r := httptest.NewRequest("PUT", "/api/admin/settings", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("anonymous default view change: %d", w.Code)
		}
		r = httptest.NewRequest("PUT", "/api/admin/settings", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if view == "" || view == "unknown" {
			if w.Code != 400 {
				t.Fatalf("invalid view accepted: %q %d", view, w.Code)
			}
			continue
		}
		stored, err := s.Settings(ctx)
		if w.Code != 200 || err != nil || stored.StatusPageView != view {
			t.Fatalf("view not persisted: %s %v", w.Body.String(), err)
		}
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/public/bootstrap", nil))
		var public struct {
			View string `json:"statusPageView"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &public) != nil || public.View != view || strings.Contains(w.Body.String(), settings.UserAgent) {
			t.Fatalf("incorrect public default: %s", w.Body.String())
		}
	}
	// 旧设置缺少该字段时保留原详细行布局，不需要迁移或改写已保存配置。
	legacy, _ := json.Marshal(model.DefaultSettings())
	legacy = bytes.Replace(legacy, []byte(`"statusPageView":"detailed",`), nil, 1)
	legacyStore := legacySettingsStore(t, legacy)
	stored, err := legacyStore.Settings(ctx)
	if err != nil || stored.StatusPageView != "detailed" {
		t.Fatalf("legacy default changed: %+v %v", stored, err)
	}
}

func TestCertificateHighlightSettings(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	token := strings.Repeat("c", 64)
	if _, err = s.DB.Exec("INSERT INTO sessions(token,expires) VALUES(?,?)", tokenHash(token), time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	handler := New(s, nil, config.Config{})
	for _, days := range []int{0, 7, 3650, -1, 3651} {
		settings := model.DefaultSettings()
		settings.CertExpiryWarningDays = days
		body, _ := json.Marshal(settings)
		r := httptest.NewRequest("PUT", "/api/admin/settings", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if days < 0 || days > 3650 {
			if w.Code != 400 {
				t.Fatalf("invalid threshold accepted: %d %d", days, w.Code)
			}
			continue
		}
		if w.Code != 200 {
			t.Fatalf("save: %d %s", w.Code, w.Body.String())
		}
		stored, err := s.Settings(ctx)
		if err != nil || stored.CertExpiryWarningDays != days {
			t.Fatalf("threshold not persisted: %+v %v", stored, err)
		}
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/public/bootstrap", nil))
		var value struct {
			Days int `json:"certExpiryWarningDays"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &value) != nil || value.Days != days {
			t.Fatalf("public setting: %s", w.Body.String())
		}
	}
	legacy, _ := json.Marshal(model.DefaultSettings())
	legacy = bytes.Replace(legacy, []byte(`"certExpiryWarningDays":30,`), nil, 1)
	legacyStore := legacySettingsStore(t, legacy)
	stored, err := legacyStore.Settings(ctx)
	if err != nil || stored.CertExpiryWarningDays != 30 {
		t.Fatalf("legacy highlight default: %+v %v", stored, err)
	}
}

// 用重启前的数据库内容验证旧设置，运行时写入统一经过 SaveSettings 更新缓存。
func legacySettingsStore(t *testing.T, data []byte) *store.Store {
	t.Helper()
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "legacy.db")
	s, err := store.Open(ctx, "sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("INSERT INTO settings(key,value) VALUES('system',?)", string(data)); err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = store.Open(ctx, "sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
