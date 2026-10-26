package api

import (
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
	"github.com/Qwerto107/UptimeNest/internal/store"
)

func TestLatencyEndpoint(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Close() })
	m, err := s.SaveMonitor(ctx, model.Monitor{Name: "latency"})
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 64)
	if _, err := s.DB.Exec("INSERT INTO sessions(token,expires) VALUES(?,?)", tokenHash(token), time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	settings, err := s.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionDays = 1
	if err := s.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	handler := New(s, nil, config.Config{})
	get := func(id int64, period string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", fmt.Sprintf("/api/admin/monitors/%d/latency%s", id, period), nil)
		if auth {
			r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for period, duration := range map[string]time.Duration{"": 24 * time.Hour, "1h": time.Hour, "3h": 3 * time.Hour, "6h": 6 * time.Hour, "12h": 12 * time.Hour, "24h": 24 * time.Hour, "3d": 72 * time.Hour, "1w": 168 * time.Hour} {
		t.Run("range_"+period, func(t *testing.T) {
			w := get(m.ID, "?range="+period, true)
			if w.Code != 200 {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			var history model.LatencyHistory
			if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil {
				t.Fatal(err)
			}
			if history.End-history.Start != duration.Milliseconds() || history.AvailableStart != max(history.Start, history.End-24*time.Hour.Milliseconds()) {
				t.Fatalf("wrong range/retention: %+v", history)
			}
			if history.Buckets == nil || history.Statistics != nil {
				t.Fatalf("wrong empty response: %+v", history)
			}
		})
	}
	if w := get(m.ID, "?range=2h", true); w.Code != 400 {
		t.Fatalf("invalid range status %d", w.Code)
	}
	if w := get(m.ID, "", false); w.Code != 401 {
		t.Fatalf("unauthenticated status %d", w.Code)
	}
	if w := get(m.ID+1, "", true); w.Code != 404 {
		t.Fatalf("missing monitor status %d", w.Code)
	}
}
