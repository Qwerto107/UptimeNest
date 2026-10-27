package store

import (
	"context"
	"database/sql"
	"path/filepath"
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
