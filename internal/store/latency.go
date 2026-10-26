package store

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

// LatencyHistory 仅扫描曲线需要的列，返回至多 480 个时间段，不解码完整心跳 JSON。
func (s *Store) LatencyHistory(ctx context.Context, id, start, end, availableStart int64) (model.LatencyHistory, error) {
	result := model.LatencyHistory{Start: start, End: end, AvailableStart: max(start, availableStart), Buckets: []model.LatencyBucket{}}
	if end <= start || end-start > (7*24*time.Hour).Milliseconds() {
		return result, fmt.Errorf("%w：延迟历史时间范围无效", ErrInvalid)
	}
	const bucketCount = 480
	step := (end - start + bucketCount - 1) / bucketCount
	buckets := make([]model.LatencyBucket, bucketCount)
	sums := make([]float64, bucketCount)
	values := []float64{}
	rows, err := s.DB.QueryContext(ctx, s.Q("SELECT time,status,latency FROM heartbeats WHERE monitor_id=? AND time>=? AND time<=? ORDER BY time,id"), id, result.AvailableStart, end)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	var sum float64
	for rows.Next() {
		var at int64
		var status string
		var latency float64
		if err := rows.Scan(&at, &status, &latency); err != nil {
			return result, err
		}
		index := min(int((at-start)/step), bucketCount-1)
		bucket := &buckets[index]
		if bucket.Count == 0 {
			bucket.Start = max(start+int64(index)*step, result.AvailableStart)
			bucket.End = min(start+int64(index+1)*step, end)
		}
		bucket.Count++
		result.Total++
		if status == "down" {
			bucket.Failures++
			result.Failures++
		}
		if status != "up" || latency < 0 || math.IsNaN(latency) || math.IsInf(latency, 0) {
			continue
		}
		bucket.Successes++
		sums[index] += latency
		sum += latency
		values = append(values, latency)
		if bucket.Minimum == nil {
			minimum, maximum := latency, latency
			bucket.Minimum, bucket.Maximum = &minimum, &maximum
		} else {
			*bucket.Minimum = min(*bucket.Minimum, latency)
			*bucket.Maximum = max(*bucket.Maximum, latency)
		}
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	// 先归还 SQLite 的单连接，再排序计算原始分布的 P95；不使用时间段平均值估算。
	if err := rows.Close(); err != nil {
		return result, err
	}
	for index, bucket := range buckets {
		if bucket.Count == 0 {
			continue
		}
		if bucket.Successes > 0 {
			average := sums[index] / float64(bucket.Successes)
			bucket.Average = &average
		}
		result.Buckets = append(result.Buckets, bucket)
	}
	if len(values) > 0 {
		sort.Float64s(values)
		result.Statistics = &model.LatencyStatistics{
			Average: sum / float64(len(values)),
			P95:     values[int(math.Ceil(float64(len(values))*0.95))-1],
			Maximum: values[len(values)-1],
		}
	}
	return result, nil
}
