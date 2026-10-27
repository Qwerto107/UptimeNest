package monitor

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

func TestServiceAlertOnlyAfterProbeRetries(t *testing.T) {
	// net/http 缓存环境代理；与同包的代理测试共用固定本机代理，目标仍走本机直连。
	unavailableProxy(t)
	for _, recoverOnRetry := range []bool{false, true} {
		t.Run(map[bool]string{false: "exhausted", true: "recovered"}[recoverOnRetry], func(t *testing.T) {
			var requests atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := requests.Add(1)
				if !recoverOnRetry || n < 3 {
					w.WriteHeader(503)
				}
			}))
			defer target.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s, err := store.Open(ctx, "sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer s.DB.Close()
			c, err := s.SaveNotificationChannel(ctx, model.NotificationChannel{Name: "test", Type: "webhook", Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			options := model.Monitor{}.HTTPConfig()
			options.Version = "http1"
			m, err := s.SaveMonitor(ctx, model.Monitor{Name: "test", Type: "http", Target: target.URL, Enabled: true, HTTP: &options, Interval: 60, Timeout: 2, Retries: 2, RetryInterval: 1, Alerts: &model.AlertOptions{Service: model.AlertRule{Enabled: true, ChannelIDs: []int64{c.ID}}}})
			if err != nil {
				t.Fatal(err)
			}
			engine := New(ctx, s, 1)
			defer engine.Wait()
			defer cancel()
			if err = engine.Reload(ctx); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(8 * time.Second)
			for {
				summary, err := s.Summary(ctx, m, 1)
				if err != nil {
					t.Fatal(err)
				}
				if summary.Latest != nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("probe never completed")
				}
				time.Sleep(20 * time.Millisecond)
			}
			records, err := s.NotificationRecords(ctx, 0)
			want := 1
			if recoverOnRetry {
				want = 0
			}
			if err != nil || requests.Load() != 3 || len(records) != want {
				t.Fatalf("requests=%d records=%+v err=%v", requests.Load(), records, err)
			}
		})
	}
}
