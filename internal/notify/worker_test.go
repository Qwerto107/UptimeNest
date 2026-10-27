package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/Qwerto107/UptimeNest/internal/store"
)

func TestWorkerRetriesAndRecordsAcceptance(t *testing.T) {
	var requests atomic.Int32
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(503)
		}
	}))
	defer receiver.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := store.Open(ctx, "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	c := model.NotificationChannel{Name: "local", Type: "webhook", Enabled: true, Config: model.NotificationConfig{URL: receiver.URL + "?secret=must-not-leak"}}
	if err = Validate(&c); err != nil {
		t.Fatal(err)
	}
	c, err = s.SaveNotificationChannel(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.QueueNotificationTest(ctx, c.ID, SampleEvent()); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); Run(ctx, s) }()
	defer func() { cancel(); <-done }()
	waitFor := func(status string, attempts int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			records, err := s.NotificationRecords(ctx, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(records) == 1 && records[0].Status == status && records[0].Attempts == attempts {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("expected %s/%d, got %+v", status, attempts, records)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	waitFor("pending", 1)
	if _, err = s.DB.Exec("UPDATE notification_deliveries SET next_attempt=0 WHERE status='pending'"); err != nil {
		t.Fatal(err)
	}
	waitFor("sent", 2)
	if requests.Load() != 2 {
		t.Fatalf("unexpected requests: %d", requests.Load())
	}
}
