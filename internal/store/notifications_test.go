package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

func alertFixture(t *testing.T, dsn string) (*Store, model.Monitor) {
	t.Helper()
	s, err := Open(context.Background(), "sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	c, err := s.SaveNotificationChannel(context.Background(), model.NotificationChannel{Name: "hook", Type: "webhook", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.SaveMonitor(context.Background(), model.Monitor{Name: "monitor", Type: "http", Target: "https://example.com", Enabled: true, Alerts: &model.AlertOptions{Service: model.AlertRule{Enabled: true, ChannelIDs: []int64{c.ID}, Recovery: true}, Certificate: model.AlertRule{Enabled: true, ChannelIDs: []int64{c.ID}, Days: 7, Recovery: true}}})
	if err != nil {
		t.Fatal(err)
	}
	return s, m
}

func appendAlert(t *testing.T, s *Store, m model.Monitor, at int64, status string, expiry int64) {
	t.Helper()
	_, err := s.Append(context.Background(), m, model.Heartbeat{MonitorID: m.ID, Time: at, Status: status, CertExpiry: expiry, Message: "probe result"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGRPCServiceChangeResetsAlerts(t *testing.T) {
	s, m := alertFixture(t, ":memory:")
	m.Type = "grpc"
	m.Target = "service.example"
	m.Port = 443
	m.GRPC = &model.GRPCOptions{Security: "tls", ServiceName: "first.Service"}
	var err error
	m, err = s.SaveMonitor(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	appendAlert(t, s, m, now, "down", now+2*86400000)
	m.GRPC.ServiceName = "second.Service"
	m, err = s.SaveMonitor(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	appendAlert(t, s, m, now+1, "down", now+2*86400000)
	records, err := s.NotificationRecords(context.Background(), 0)
	if err != nil || len(records) != 4 {
		t.Fatalf("expected service and certificate alerts for both configurations: %+v %v", records, err)
	}
	for i, record := range records {
		expected := "pending"
		if i >= 2 {
			expected = "cancelled"
		}
		if record.Status != expected {
			t.Fatalf("stale alert state or pending notification survived: %+v", records)
		}
	}
}

func TestAlertsPersistRestartDeduplicateAndRecover(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "alerts.db")
	s, m := alertFixture(t, dsn)
	now := time.Now().UnixMilli()
	appendAlert(t, s, m, now, "up", 0)
	appendAlert(t, s, m, now+1, "down", 0)
	appendAlert(t, s, m, now+2, "down", 0)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, "sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	appendAlert(t, reopened, m, now+3, "down", 0)
	appendAlert(t, reopened, m, now+4, "up", 0)
	records, err := reopened.NotificationRecords(ctx, 0)
	if err != nil || len(records) != 2 || records[1].Event.AlertType != "service_down" || records[0].Event.AlertType != "service_recovered" || records[0].Event.Started != now+1 {
		t.Fatalf("unexpected events: %+v, %v", records, err)
	}
	first, err := reopened.ClaimNotification(ctx, now+100)
	if err != nil || first == nil || first.Event.AlertType != "service_down" {
		t.Fatalf("claim: %+v %v", first, err)
	}
	blocked, err := reopened.ClaimNotification(ctx, now+101)
	if err != nil || blocked != nil {
		t.Fatalf("recovery overtook in-flight failure: %+v %v", blocked, err)
	}
	if err = reopened.FinishNotification(ctx, *first, "pending", "retry", now+1000); err != nil {
		t.Fatal(err)
	}
	blocked, err = reopened.ClaimNotification(ctx, now+999)
	if err != nil || blocked != nil {
		t.Fatalf("recovery overtook retry: %+v %v", blocked, err)
	}
	retry, err := reopened.ClaimNotification(ctx, now+1001)
	if err != nil || retry == nil || retry.ID != first.ID || retry.Attempts != 2 {
		t.Fatalf("retry: %+v %v", retry, err)
	}
	if err = reopened.FinishNotification(ctx, *retry, "sent", "", 0); err != nil {
		t.Fatal(err)
	}
	recovery, err := reopened.ClaimNotification(ctx, now+1002)
	if err != nil || recovery == nil || recovery.Event.AlertType != "service_recovered" {
		t.Fatalf("recovery: %+v %v", recovery, err)
	}
}

func TestCertificateThresholdExpiryRenewalAndMissingTLS(t *testing.T) {
	s, m := alertFixture(t, ":memory:")
	now := time.Now().UnixMilli()
	expires := now + 8*86400000
	appendAlert(t, s, m, now, "up", expires)
	appendAlert(t, s, m, now+86400000, "up", expires)
	appendAlert(t, s, m, now+2*86400000, "down", 0)
	appendAlert(t, s, m, expires, "down", 0)
	appendAlert(t, s, m, expires+1000, "up", expires+90*86400000)
	records, err := s.NotificationRecords(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, d := range records {
		counts[d.Event.AlertType]++
	}
	for _, kind := range []string{"certificate_expiring", "certificate_expired", "certificate_recovered", "service_down", "service_recovered"} {
		if counts[kind] != 1 {
			t.Fatalf("%s count: %v", kind, counts)
		}
	}
}

func TestAlertRepeatDisableStaleSnapshotAndChannelReference(t *testing.T) {
	s, m := alertFixture(t, ":memory:")
	ctx := context.Background()
	m.Alerts.Service.RepeatMinutes = 1
	var err error
	m, err = s.SaveMonitor(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	appendAlert(t, s, m, now, "down", 0)
	initial, err := s.ClaimNotification(ctx, now)
	if err != nil || initial == nil {
		t.Fatalf("initial claim: %+v %v", initial, err)
	}
	if err = s.FinishNotification(ctx, *initial, "sent", "", 0); err != nil {
		t.Fatal(err)
	}
	appendAlert(t, s, m, now+59999, "down", 0)
	appendAlert(t, s, m, now+60000, "down", 0)
	appendAlert(t, s, m, now+120000, "down", 0)
	records, err := s.NotificationRecords(ctx, 0)
	if err != nil || len(records) != 2 || !records[0].Event.Repeat {
		t.Fatalf("repeat: %+v %v", records, err)
	}
	if err = s.DeleteNotificationChannel(ctx, m.Alerts.Service.ChannelIDs[0]); !errors.Is(err, ErrInvalid) {
		t.Fatalf("deleted bound channel: %v", err)
	}
	old := m
	m.Enabled = false
	m, err = s.SaveMonitor(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Append(ctx, old, model.Heartbeat{MonitorID: old.ID, Time: now + 70000, Status: "down"}); !errors.Is(err, ErrStale) {
		t.Fatalf("accepted stale heartbeat: %v", err)
	}
	d, err := s.ClaimNotification(ctx, now+80000)
	if err != nil || d != nil {
		t.Fatalf("disabled monitor still queued: %+v %v", d, err)
	}
}

func TestAlertTimerDoesNotInventInitialEvent(t *testing.T) {
	s, m := alertFixture(t, ":memory:")
	ctx := context.Background()
	now := time.Now().Add(-2 * time.Minute).UnixMilli()
	appendAlert(t, s, m, now, "up", now+time.Minute.Milliseconds())
	if err := s.EvaluateAlertTimers(ctx); err != nil {
		t.Fatal(err)
	}
	records, err := s.NotificationRecords(ctx, 0)
	if err != nil || len(records) != 2 || records[0].Event.AlertType != "certificate_expired" {
		t.Fatalf("timer: %+v %v", records, err)
	}
	m.Target = "https://new.example.com"
	if _, err = s.SaveMonitor(ctx, m); err != nil {
		t.Fatal(err)
	}
	if err = s.EvaluateAlertTimers(ctx); err != nil {
		t.Fatal(err)
	}
	records, err = s.NotificationRecords(ctx, 0)
	if err != nil || len(records) != 2 {
		t.Fatalf("old target triggered new rule: %+v %v", records, err)
	}
}

func TestNotificationMigrationPreservesVersionTwoData(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "legacy.db")
	s, err := Open(ctx, "sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.SaveMonitor(ctx, model.Monitor{Name: "legacy", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	// 模拟版本 2 的库，保留旧监控与系统设置再执行新增迁移。
	for _, q := range []string{"DROP TABLE alert_states", "DROP TABLE notification_deliveries", "DROP TABLE notification_channels", "DROP INDEX heartbeats_summary", "DELETE FROM schema_version WHERE version>=3"} {
		if _, err = s.DB.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	s, err = Open(ctx, "sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	legacy, err := s.Monitor(ctx, m.ID)
	if err != nil || legacy.Name != "legacy" || legacy.Alerts != nil {
		t.Fatalf("legacy config changed: %+v %v", legacy, err)
	}
	if records, err := s.NotificationRecords(ctx, 0); err != nil || len(records) != 0 {
		t.Fatalf("queue migration: %+v %v", records, err)
	}
}

func TestServiceAlertIndependentThresholdPersistsAndResets(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "threshold.db")
	s, m := alertFixture(t, dsn)
	m.Retries = 9
	m.Alerts.Service.FailureThreshold = 3
	var err error
	m, err = s.SaveMonitor(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Minute).UnixMilli()
	appendAlert(t, s, m, now, "down", 0)
	if err = s.EvaluateAlertTimers(ctx); err != nil {
		t.Fatal(err)
	}
	if records, err := s.NotificationRecords(ctx, 0); err != nil || len(records) != 0 {
		t.Fatalf("timer counted a failure: %+v %v", records, err)
	}
	s.Close()
	s, err = Open(ctx, "sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	appendAlert(t, s, m, now+1, "down", 0)
	if records, err := s.NotificationRecords(ctx, 0); err != nil || len(records) != 0 {
		t.Fatalf("alert before threshold: %+v %v", records, err)
	}
	appendAlert(t, s, m, now+2, "up", 0)
	appendAlert(t, s, m, now+3, "down", 0)
	appendAlert(t, s, m, now+4, "down", 0)
	if records, err := s.NotificationRecords(ctx, 0); err != nil || len(records) != 0 {
		t.Fatalf("success did not reset count: %+v %v", records, err)
	}
	appendAlert(t, s, m, now+5, "down", 0)
	records, err := s.NotificationRecords(ctx, 0)
	if err != nil || len(records) != 1 || records[0].Event.AlertType != "service_down" || records[0].Event.Started != now+3 {
		t.Fatalf("independent threshold: %+v %v", records, err)
	}
	appendAlert(t, s, m, now+6, "up", 0)
	records, err = s.NotificationRecords(ctx, 0)
	if err != nil || len(records) != 2 || records[0].Event.AlertType != "service_recovered" {
		t.Fatalf("recovery: %+v %v", records, err)
	}
}
