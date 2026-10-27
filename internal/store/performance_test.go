package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

func fileStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), "sqlite", filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLiteReadPoolAndConnectionPragmas(t *testing.T) {
	s := fileStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if s.DB == s.ReadDB() {
		t.Fatal("file database still shares one read/write pool")
	}
	connections := []*sql.Conn{}
	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
	}()
	for range 4 {
		conn, err := s.ReadDB().Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		connections = append(connections, conn)
		checkSQLitePragmas(t, conn, 1)
		if _, err := conn.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES('forbidden','{}')"); err == nil {
			t.Fatal("read pool accepted a write")
		}
	}
	// 四个读连接全部占用时，写池仍能提交监控配置。
	m, err := s.SaveMonitor(ctx, model.Monitor{Name: "independent-writer", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, conn := range connections {
		conn.Close()
	}
	connections = nil

	reader, err := s.ReadDB().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Rollback()
	var count int
	if err = reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM heartbeats").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Append(ctx, m, model.Heartbeat{MonitorID: m.ID, Time: time.Now().UnixMilli(), Status: "up"}); err != nil {
		t.Fatalf("WAL reader blocked heartbeat commit: %v", err)
	}
	reader.Rollback()

	// 强制关闭空闲连接，覆盖 database/sql 重建连接后的配置和级联删除。
	for _, pool := range []*sql.DB{s.DB, s.ReadDB()} {
		pool.SetMaxIdleConns(0)
	}
	writer, err := s.DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	checkSQLitePragmas(t, writer, 0)
	writer.Close()
	if err = s.DeleteMonitor(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.ReadDB().QueryRowContext(ctx, "SELECT COUNT(*) FROM heartbeats").Scan(&count); err != nil || count != 0 {
		t.Fatalf("cascade lost after reconnect: %d %v", count, err)
	}
	conn, err := s.ReadDB().Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	checkSQLitePragmas(t, conn, 1)
}

func checkSQLitePragmas(t *testing.T, conn *sql.Conn, queryOnly int) {
	t.Helper()
	for name, want := range map[string]int{"foreign_keys": 1, "busy_timeout": 5000, "query_only": queryOnly, "synchronous": 2} {
		var got int
		if err := conn.QueryRowContext(context.Background(), "PRAGMA "+name).Scan(&got); err != nil || got != want {
			t.Fatalf("%s = %d, want %d: %v", name, got, want, err)
		}
	}
}

func TestSettingsCacheIsolationAndConcurrentSaves(t *testing.T) {
	s := fileStore(t)
	ctx := context.Background()
	settings := model.DefaultSettings()
	settings.TrustedProxies = []string{"127.0.0.1"}
	if err := s.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	settings.TrustedProxies[0] = "changed-by-caller"
	got, err := s.Settings(ctx)
	if err != nil || got.TrustedProxies[0] != "127.0.0.1" {
		t.Fatalf("save aliases caller memory: %+v %v", got, err)
	}
	got.TrustedProxies[0] = "changed-by-reader"
	got, err = s.Settings(ctx)
	if err != nil || got.TrustedProxies[0] != "127.0.0.1" {
		t.Fatalf("read aliases cache memory: %+v %v", got, err)
	}
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			value := model.DefaultSettings()
			value.SiteName = fmt.Sprintf("site-%d", i)
			if err := s.SaveSettings(ctx, value); err != nil {
				t.Error(err)
			}
			if _, err := s.Settings(ctx); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	var data string
	if err := s.ReadDB().QueryRowContext(ctx, "SELECT value FROM settings WHERE key='system'").Scan(&data); err != nil {
		t.Fatal(err)
	}
	var persisted model.Settings
	if err := json.Unmarshal([]byte(data), &persisted); err != nil {
		t.Fatal(err)
	}
	got, err = s.Settings(ctx)
	if err != nil || !reflect.DeepEqual(got, persisted) {
		t.Fatalf("cache differs from last committed settings: %+v %+v %v", got, persisted, err)
	}
	if _, err := s.DB.Exec("DROP TABLE settings"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSettings(ctx, model.DefaultSettings()); err == nil {
		t.Fatal("save unexpectedly succeeded")
	}
	got, err = s.Settings(ctx)
	if err != nil || !reflect.DeepEqual(got, persisted) {
		t.Fatalf("cache depends on database reads or publishes failed saves: %+v %v", got, err)
	}
}

func TestSummaryKeepsLatestAndFailureDetails(t *testing.T) {
	s := fileStore(t)
	ctx := context.Background()
	m, err := s.SaveMonitor(ctx, model.Monitor{Name: "summary", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	for i, status := range []string{"up", "down", "up", "up"} {
		h := model.Heartbeat{MonitorID: m.ID, Time: now, Status: status, Latency: float64(i), Message: "HTTP 状态码 503", TLSVersion: "TLS 1.3", CertExpiry: now + 86400000}
		if _, err = s.Append(ctx, m, h); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := s.Summary(ctx, m, 3)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Uptime == nil || *summary.Uptime != 75 || summary.Average == nil || *summary.Average != 5.0/3 {
		t.Fatalf("statistics changed: %+v", summary)
	}
	beats := summary.Heartbeats
	if len(beats) != 3 || beats[0].Status != "down" || beats[0].Message != "HTTP 状态码 503" || beats[1].TLSVersion != "" || beats[2].TLSVersion != "TLS 1.3" || summary.Latest.ID != beats[2].ID {
		t.Fatalf("latest, failure, or ordering lost: %+v", summary)
	}
	history, err := s.Heartbeats(ctx, m.ID, 0, 4)
	if err != nil || len(history) != 4 || history[2].TLSVersion != "TLS 1.3" {
		t.Fatalf("full history lost metadata: %+v %v", history, err)
	}
	rows, err := s.ReadDB().QueryContext(ctx, "EXPLAIN QUERY PLAN SELECT COUNT(*),AVG(latency) FROM heartbeats WHERE monitor_id=? AND time>=?", m.ID, now-1000)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detail)
	}
	if err := rows.Err(); err != nil || !strings.Contains(plan.String(), "COVERING INDEX heartbeats_summary") {
		t.Fatalf("summary not covered by index: %s %v", plan.String(), err)
	}
}

func TestUnchangedAlertsDoNotWriteState(t *testing.T) {
	s, m := alertFixture(t, ":memory:")
	now := time.Now().UnixMilli()
	appendAlert(t, s, m, now, "up", now+30*86400000)
	if _, err := s.DB.Exec("CREATE TRIGGER unchanged_alert BEFORE UPDATE ON alert_states BEGIN SELECT RAISE(ABORT,'unexpected alert state update'); END"); err != nil {
		t.Fatal(err)
	}
	appendAlert(t, s, m, now+1, "up", now+30*86400000)
	if err := s.EvaluateAlertTimers(context.Background()); err != nil {
		t.Fatalf("timer rewrote unchanged alert: %v", err)
	}
}
