package store

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

func TestMonitorDeletionResumesAfterRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	dsn := filepath.Join(t.TempDir(), "store.db")
	s, err := Open(ctx, "sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	removed, err := s.SaveMonitor(ctx, model.Monitor{Name: "removed", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	kept, err := s.SaveMonitor(ctx, model.Monitor{Name: "kept", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	panel, err := s.SavePanel(ctx, model.Panel{Slug: "panel", Groups: []model.PanelGroup{
		{ID: "group", Name: "Group", MonitorIDs: []int64{removed.ID, kept.ID}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	const series = "WITH RECURSIVE series(n) AS (VALUES(1) UNION ALL SELECT n+1 FROM series WHERE n<750) "
	for _, query := range []string{
		series + "INSERT INTO heartbeats(monitor_id,time,status,latency,data) SELECT ?,n,'down',0,'{}' FROM series",
		series + "INSERT INTO monitor_logs(monitor_id,time,status,message) SELECT ?,n,'down','' FROM series",
	} {
		if _, err = s.DB.ExecContext(ctx, query, removed.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.Append(ctx, kept, model.Heartbeat{MonitorID: kept.ID, Time: 1, Status: "up"}); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteMonitor(ctx, removed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Monitor(ctx, removed.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted monitor still visible: %v", err)
	}
	if _, err = s.SaveMonitor(ctx, removed); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted monitor remained editable: %v", err)
	}
	if _, err = s.Append(ctx, removed, model.Heartbeat{MonitorID: removed.ID, Status: "up"}); !errors.Is(err, ErrStale) {
		t.Fatalf("deleted monitor accepted a heartbeat: %v", err)
	}
	if p, err := s.PanelBySlug(ctx, panel.Slug); err != nil || len(p.Groups) != 1 ||
		!slices.Equal(p.MonitorIDs, []int64{kept.ID}) || !slices.Equal(p.Groups[0].MonitorIDs, []int64{kept.ID}) {
		t.Fatalf("panel references not removed: %+v %v", p, err)
	}
	if _, err = s.SavePanel(ctx, panel); !errors.Is(err, ErrNotFound) {
		t.Fatalf("panel restored a deleted monitor: %v", err)
	}
	// 模拟只提交一个清理批次就退出，剩余历史由下次启动接续处理。
	if n, err := s.purgeMonitorHistoryBatch(ctx, "heartbeats", removed.ID); err != nil || n != 500 {
		t.Fatalf("first batch deleted %d rows, want 500: %v", n, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, "sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	done := make(chan struct{})
	go func() { defer close(done); reopened.RunMonitorPurge(ctx) }()
	defer func() { cancel(); <-done }()
	for {
		var pending bool
		if err = reopened.ReadDB().QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM monitors WHERE id=?)", removed.ID).Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if !pending {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("cleanup did not resume after restart")
		case <-time.After(10 * time.Millisecond):
		}
	}
	for _, table := range []string{"heartbeats", "monitor_logs"} {
		var count int
		if err = reopened.ReadDB().QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE monitor_id=?", removed.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s retained %d rows for the deleted monitor: %v", table, count, err)
		}
		if err = reopened.ReadDB().QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE monitor_id=?", kept.ID).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s retained %d rows for the other monitor, want 1: %v", table, count, err)
		}
	}
}
