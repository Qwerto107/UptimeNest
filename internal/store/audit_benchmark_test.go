package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

// BenchmarkAuditQueries 使用完整 24 小时样本，分别覆盖常规数量与高频心跳。
func BenchmarkAuditQueries(b *testing.B) {
	for _, scenario := range []struct {
		name             string
		monitors, points int
		interval         time.Duration
	}{
		{"100-monitors-60s", 100, 1440, time.Minute},
		{"10-monitors-1s", 10, 86400, time.Second},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			ctx := context.Background()
			s, err := Open(ctx, "sqlite", filepath.Join(b.TempDir(), "bench.db"))
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			monitors := []model.Monitor{}
			for i := range scenario.monitors {
				m, err := s.SaveMonitor(ctx, model.Monitor{Name: fmt.Sprintf("monitor-%d", i), Enabled: true})
				if err != nil {
					b.Fatal(err)
				}
				monitors = append(monitors, m)
			}
			tx, err := s.DB.BeginTx(ctx, nil)
			if err != nil {
				b.Fatal(err)
			}
			defer tx.Rollback()
			now := time.Now().UnixMilli()
			for _, m := range monitors {
				// 数据在计时前一次性生成，避免把夹具构建算入查询成本。
				h := model.Heartbeat{MonitorID: m.ID, Time: now, Status: "up", Latency: 12, TLSVersion: "TLS 1.3", HTTPVersion: "HTTP/2.0", CertIssuerOrg: "Example CA", TLSCipherSuite: "TLS_AES_128_GCM_SHA256"}
				_, err = tx.ExecContext(ctx, "WITH RECURSIVE points(n) AS (SELECT 0 UNION ALL SELECT n+1 FROM points WHERE n+1<?) INSERT INTO heartbeats(monitor_id,time,status,latency,data) SELECT ?,?-n*?,'up',12,? FROM points", scenario.points, m.ID, now, scenario.interval.Milliseconds(), encode(h))
				if err != nil {
					b.Fatal(err)
				}
			}
			if err = tx.Commit(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := s.Summaries(ctx, monitors, 420); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
