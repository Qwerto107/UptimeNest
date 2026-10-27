package store

import (
	"context"
	"errors"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

func latencyStore(t *testing.T) (*Store, int64) {
	t.Helper()
	s, err := Open(context.Background(), "sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	m, err := s.SaveMonitor(context.Background(), model.Monitor{Name: "latency"})
	if err != nil {
		t.Fatal(err)
	}
	return s, m.ID
}

func TestLatencyHistoryBoundaries(t *testing.T) {
	s, id := latencyStore(t)
	ctx := context.Background()
	start := int64(1_000_000)
	end := start + time.Hour.Milliseconds()
	other, err := s.SaveMonitor(ctx, model.Monitor{Name: "other"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id, at int64
		status string
		value  float64
	}{
		{id, start - 1, "up", 10000}, {id, start, "up", 0},
		{id, start + 1000, "down", 9000}, {id, start + 2000, "up", -1},
		{id, start + 3000, "paused", 5000}, {id, end, "up", 100},
		{id, end + 1, "up", 10000}, {other.ID, start, "up", 10000},
	} {
		if _, err := s.DB.Exec("INSERT INTO heartbeats(monitor_id,time,status,latency,data) VALUES(?,?,?,?,?)", row.id, row.at, row.status, row.value, "{}"); err != nil {
			t.Fatal(err)
		}
	}
	h, err := s.LatencyHistory(ctx, id, start, end, start-100)
	if err != nil {
		t.Fatal(err)
	}
	if h.Total != 5 || h.Failures != 1 || h.Statistics == nil || *h.Statistics != (model.LatencyStatistics{Average: 50, P95: 100, Maximum: 100}) {
		t.Fatalf("unexpected history: %+v statistics: %+v", h, h.Statistics)
	}
	if h.Start != start || h.End != end || h.AvailableStart != start || len(h.Buckets) != 2 {
		t.Fatalf("wrong bounds: %+v", h)
	}
	if h.Buckets[0].Count != 4 || h.Buckets[0].Successes != 1 || *h.Buckets[0].Average != 0 {
		t.Fatalf("invalid checks affected average: %+v", h.Buckets[0])
	}
	retained, err := s.LatencyHistory(ctx, id, start, end, start+1)
	if err != nil {
		t.Fatal(err)
	}
	if retained.Total != 4 || retained.Statistics.Average != 100 || retained.AvailableStart != start+1 {
		t.Fatalf("retention ignored: %+v", retained)
	}
	for _, period := range [][2]int64{{end, start}, {start, start}, {start, start + (8 * 24 * time.Hour).Milliseconds()}} {
		if _, err := s.LatencyHistory(ctx, id, period[0], period[1], start); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid range: %v", err)
		}
	}
}

func TestLatencyHistoryDenseSeries(t *testing.T) {
	s, id := latencyStore(t)
	start := int64(1_000_000)
	end := start + (7 * 24 * time.Hour).Milliseconds()
	tx, err := s.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	values := []float64{}
	var sum float64
	for i := 0; i < 6000; i++ {
		value, status := float64(i%100), "up"
		if i == 17 {
			value = 12345
		}
		if i%10 == 0 {
			status = "down"
		} else {
			values = append(values, value)
			sum += value
		}
		if _, err := tx.Exec("INSERT INTO heartbeats(monitor_id,time,status,latency,data) VALUES(?,?,?,?,?)", id, start+int64(i)*5000, status, value, "{}"); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	h, err := s.LatencyHistory(context.Background(), id, start, end, start)
	if err != nil {
		t.Fatal(err)
	}
	sort.Float64s(values)
	if h.Total != 6000 || h.Failures != 600 || len(h.Buckets) > 480 || len(h.Buckets) >= h.Total {
		t.Fatalf("series not bounded: %+v", h)
	}
	if h.Statistics == nil || math.Abs(h.Statistics.Average-sum/float64(len(values))) > 1e-9 || h.Statistics.P95 != values[int(math.Ceil(float64(len(values))*0.95))-1] || h.Statistics.Maximum != 12345 {
		t.Fatalf("raw statistics lost: %+v", h.Statistics)
	}
	count, failures := 0, 0
	maximum := float64(0)
	for i, bucket := range h.Buckets {
		count += bucket.Count
		failures += bucket.Failures
		if bucket.Start < start || bucket.End > end || bucket.Start > bucket.End || (i > 0 && bucket.Start < h.Buckets[i-1].End) {
			t.Fatalf("invalid bucket bounds: %+v", bucket)
		}
		if bucket.Minimum == nil || bucket.Maximum == nil || bucket.Average == nil || *bucket.Minimum > *bucket.Average || *bucket.Average > *bucket.Maximum {
			t.Fatalf("invalid envelope: %+v", bucket)
		}
		maximum = max(maximum, *bucket.Maximum)
	}
	if count != h.Total || failures != h.Failures || maximum != h.Statistics.Maximum {
		t.Fatal("bucket counts or spike lost")
	}
}

func TestLatencyHistoryEmptyAndFailures(t *testing.T) {
	s, id := latencyStore(t)
	ctx := context.Background()
	h, err := s.LatencyHistory(ctx, id, 1000, 2000, 1000)
	if err != nil || h.Statistics != nil || h.Buckets == nil || len(h.Buckets) != 0 || h.Total != 0 {
		t.Fatalf("wrong empty history: %+v, %v", h, err)
	}
	if _, err := s.DB.Exec("INSERT INTO heartbeats(monitor_id,time,status,latency,data) VALUES(?,?,?,?,?)", id, 1500, "down", 10000, "{}"); err != nil {
		t.Fatal(err)
	}
	h, err = s.LatencyHistory(ctx, id, 1000, 2000, 1000)
	if err != nil || h.Total != 1 || h.Failures != 1 || h.Statistics != nil || len(h.Buckets) != 1 {
		t.Fatalf("wrong failure history: %+v, %v", h, err)
	}
	bucket := h.Buckets[0]
	if bucket.Average != nil || bucket.Minimum != nil || bucket.Maximum != nil || bucket.Successes != 0 {
		t.Fatalf("failure counted as success: %+v", bucket)
	}
}
