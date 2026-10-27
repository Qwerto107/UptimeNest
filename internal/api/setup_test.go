package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Qwerto107/UptimeNest/internal/config"
	"github.com/Qwerto107/UptimeNest/internal/store"
)

func setupRequest(handler http.Handler, token string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"password": "test-password", "token": token})
	r := httptest.NewRequest("POST", "/api/admin/setup", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestSetupRequiresCurrentToken(t *testing.T) {
	s, err := store.Open(context.Background(), "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	handler := New(s, nil, config.Config{SetupToken: "current-startup-token"})
	for _, token := range []string{"", "wrong-token", "previous-startup-token"} {
		w := setupRequest(handler, token)
		if w.Code != http.StatusForbidden || len(w.Result().Cookies()) != 0 {
			t.Fatalf("invalid token accepted: %d %s", w.Code, w.Body.String())
		}
	}
	var accounts, sessions int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM account").Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if accounts != 0 || sessions != 0 {
		t.Fatal("invalid token changed authentication state")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/admin/session", nil))
	if strings.Contains(w.Body.String(), "current-startup-token") {
		t.Fatal("setup token leaked through session response")
	}
	w = setupRequest(handler, "current-startup-token")
	if w.Code != http.StatusCreated || len(w.Result().Cookies()) != 1 {
		t.Fatalf("valid setup failed: %d %s", w.Code, w.Body.String())
	}
	if w = setupRequest(handler, "current-startup-token"); w.Code != http.StatusConflict {
		t.Fatalf("setup token reused: %d", w.Code)
	}
}

func TestSetupWithoutConfiguredTokenIsClosed(t *testing.T) {
	s, err := store.Open(context.Background(), "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	w := setupRequest(New(s, nil, config.Config{}), "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("unconfigured setup accepted: %d", w.Code)
	}
}

func TestConcurrentSetupCreatesOneAccountAndSession(t *testing.T) {
	s, err := store.Open(context.Background(), "sqlite", filepath.Join(t.TempDir(), "setup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	handler := New(s, nil, config.Config{SetupToken: "startup-token"})
	var wg sync.WaitGroup
	codes := make(chan int, 4)
	for range 4 {
		wg.Go(func() { codes <- setupRequest(handler, "startup-token").Code })
	}
	wg.Wait()
	close(codes)
	created := 0
	for code := range codes {
		if code == http.StatusCreated {
			created++
		} else if code != http.StatusConflict {
			t.Fatalf("unexpected concurrent setup status: %d", code)
		}
	}
	var accounts, sessions int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM account").Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if created != 1 || accounts != 1 || sessions != 1 {
		t.Fatalf("concurrent setup: %d successes, %d accounts, %d sessions", created, accounts, sessions)
	}
}
