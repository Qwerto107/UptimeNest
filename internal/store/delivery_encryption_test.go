package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeliveryEventsAreEncryptedAndMigrated(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "events.db")
	s, m := alertFixture(t, dsn)
	appendAlert(t, s, m, time.Now().UnixMilli(), "down", 0)
	records, err := s.NotificationRecords(ctx, 0)
	if err != nil || len(records) == 0 {
		t.Fatalf("no delivery queued: %+v %v", records, err)
	}
	id, name := records[0].ID, records[0].Event.MonitorName
	read := func(s *Store) string {
		var raw string
		if err := s.DB.QueryRow("SELECT event FROM notification_deliveries WHERE id=?", id).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	if raw := read(s); !strings.HasPrefix(raw, encryptedPrefix) || (name != "" && strings.Contains(raw, name)) {
		t.Fatalf("event stored in plaintext: %s", raw)
	}
	// 模拟版本 5 的库：事件为明文，版本记录只到 5。
	if _, err = s.DB.Exec("UPDATE notification_deliveries SET event=? WHERE id=?", encode(records[0].Event), id); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("DELETE FROM schema_version WHERE version>=6"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(ctx, "sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if raw := read(s); !strings.HasPrefix(raw, encryptedPrefix) {
		t.Fatalf("legacy event not migrated: %s", raw)
	}
	records, err = s.NotificationRecords(ctx, 0)
	if err != nil || len(records) == 0 || records[0].Event.MonitorName != name {
		t.Fatalf("migrated event unreadable: %+v %v", records, err)
	}
}
