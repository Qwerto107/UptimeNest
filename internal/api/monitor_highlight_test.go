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
	"github.com/Qwerto107/UptimeNest/internal/monitor"
	"github.com/Qwerto107/UptimeNest/internal/store"
)

func TestMonitorCertificateHighlightOverride(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := store.Open(ctx, "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	token := strings.Repeat("h", 64)
	if _, err := s.DB.Exec("INSERT INTO sessions(token,expires) VALUES(?,?)", tokenHash(token), time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	handler := New(s, monitor.New(ctx, s, 1), config.Config{})
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
	m := model.Monitor{Name: "TLS", Type: "http", Target: "https://example.com", Interval: 60, Timeout: 10, RetryInterval: 1}
	if w := call("POST", "/api/admin/monitors", m, false); w.Code != 401 {
		t.Fatalf("unauthenticated save: %d", w.Code)
	}
	w := call("POST", "/api/admin/monitors", m, true)
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &m) != nil {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if m.CertExpiryWarningDays != nil {
		t.Fatal("missing field must inherit system setting")
	}
	if _, err := s.SavePanel(ctx, model.Panel{Name: "TLS", Slug: "tls", MonitorIDs: []int64{m.ID}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		days    any
		present bool
		valid   bool
	}{
		{"custom", 7, true, true},
		{"disabled", 0, true, true},
		{"maximum", 3650, true, true},
		{"negative", -1, true, false},
		{"too large", 3651, true, false},
		{"fraction", 1.5, true, false},
		{"string", "7", true, false},
		{"reset to inherit", nil, true, true},
		{"omitted legacy field", nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(m)
			var body map[string]any
			json.Unmarshal(data, &body)
			if tc.present {
				body["certExpiryWarningDays"] = tc.days
			} else {
				delete(body, "certExpiryWarningDays")
			}
			w := call("PUT", "/api/admin/monitors/"+strconv.FormatInt(m.ID, 10), body, true)
			if !tc.valid {
				if w.Code != 400 {
					t.Fatalf("invalid threshold accepted: %d %s", w.Code, w.Body.String())
				}
				return
			}
			if w.Code != 200 {
				t.Fatalf("save: %d %s", w.Code, w.Body.String())
			}
			stored, err := s.Monitor(ctx, m.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.days == nil {
				if stored.CertExpiryWarningDays != nil {
					t.Fatal("inheritance was converted into an override")
				}
			} else if stored.CertExpiryWarningDays == nil || *stored.CertExpiryWarningDays != tc.days.(int) {
				t.Fatalf("override not persisted: %+v", stored.CertExpiryWarningDays)
			}
			w = call("GET", "/api/public/panels/tls", nil, false)
			var panel struct {
				Monitors []map[string]any `json:"monitors"`
			}
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &panel) != nil || len(panel.Monitors) != 1 {
				t.Fatalf("public panel: %d %s", w.Code, w.Body.String())
			}
			value, exists := panel.Monitors[0]["certExpiryWarningDays"]
			if tc.days == nil {
				if exists {
					t.Fatal("inherited field should be omitted from public data")
				}
			} else if !exists || value != float64(tc.days.(int)) {
				t.Fatalf("public override missing or changed: %v", value)
			}
		})
	}
}
