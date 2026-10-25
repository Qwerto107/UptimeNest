package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

// Append 验证配置快照后写入结果，阻止已停用、删除或编辑的旧检测写回数据。
func (s *Store) Append(ctx context.Context, snapshot model.Monitor, h model.Heartbeat) (string, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	q := "SELECT data FROM monitors WHERE id=?"
	if s.dialect == "postgres" {
		q += " FOR UPDATE"
	}
	var data string
	if err = tx.QueryRowContext(ctx, s.Q(q), snapshot.ID).Scan(&data); errors.Is(err, sql.ErrNoRows) {
		return "", ErrStale
	}
	if err != nil {
		return "", err
	}
	var current model.Monitor
	if err = json.Unmarshal([]byte(data), &current); err != nil {
		return "", err
	}
	current.ID = snapshot.ID
	if !current.Enabled || !reflect.DeepEqual(current, snapshot) {
		return "", ErrStale
	}
	previous := "unknown"
	err = tx.QueryRowContext(ctx, s.Q("SELECT status FROM heartbeats WHERE monitor_id=? ORDER BY time DESC,id DESC LIMIT 1"), h.MonitorID).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, s.Q("INSERT INTO heartbeats(monitor_id,time,status,latency,data) VALUES(?,?,?,?,?)"), h.MonitorID, h.Time, h.Status, h.Latency, encode(h)); err != nil {
		return "", err
	}
	// 状态改变或故障时保留日志，正常且未变的心跳不重复生成日志。
	if previous != h.Status || h.Status == "down" {
		if _, err = tx.ExecContext(ctx, s.Q("INSERT INTO monitor_logs(monitor_id,time,status,message) VALUES(?,?,?,?)"), h.MonitorID, h.Time, h.Status, h.Message); err != nil {
			return "", err
		}
	}
	return previous, tx.Commit()
}

// Summary 按页面所需条数返回最近心跳，近 24 小时统计独立计算。
func (s *Store) Summary(ctx context.Context, m model.Monitor, heartbeatLimit int) (model.Summary, error) {
	items, err := s.Summaries(ctx, []model.Monitor{m}, heartbeatLimit)
	if err != nil {
		return model.Summary{}, err
	}
	return items[m.ID], nil
}

// Summaries 批量统计同一时间范围，近期心跳仍按监控使用索引和 LIMIT 读取。
func (s *Store) Summaries(ctx context.Context, monitors []model.Monitor, heartbeatLimit int) (map[int64]model.Summary, error) {
	type statistics struct {
		total, up int64
		average   sql.NullFloat64
	}
	result := make(map[int64]model.Summary, len(monitors))
	cutoff := time.Now().Add(-24 * time.Hour).UnixMilli()
	aggregate := "COUNT(*), COALESCE(SUM(CASE WHEN status='up' THEN 1 ELSE 0 END),0), AVG(CASE WHEN status='up' THEN latency ELSE NULL END)"
	// 小批次交替读取统计与近期心跳，缩短连接占用并复用刚访问的数据页。
	const batchSize = 8
	for start := 0; start < len(monitors); start += batchSize {
		batch := monitors[start:min(start+batchSize, len(monitors))]
		stats := make(map[int64]statistics, len(batch))
		args := make([]any, 0, len(batch)+1)
		for _, m := range batch {
			args = append(args, m.ID)
		}
		args = append(args, cutoff)
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		query := "SELECT monitor_id, " + aggregate + " FROM heartbeats WHERE monitor_id IN (" + placeholders + ") AND time>=? GROUP BY monitor_id"
		if s.dialect == "postgres" {
			// 分监控聚合保留索引访问，避免大批次并行扫描在多请求下放大资源开销。
			values := strings.TrimSuffix(strings.Repeat("(CAST(? AS BIGINT)),", len(batch)), ",")
			query = "SELECT selected.id, stats.* FROM (VALUES " + values + ") AS selected(id) CROSS JOIN LATERAL (SELECT " + aggregate + " FROM heartbeats WHERE monitor_id=selected.id AND time>=?) AS stats"
		}
		rows, err := s.DB.QueryContext(ctx, s.Q(query), args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			var v statistics
			if err = rows.Scan(&id, &v.total, &v.up, &v.average); err != nil {
				rows.Close()
				return nil, err
			}
			stats[id] = v
		}
		err = rows.Err()
		// SQLite 仅有一个连接，必须先归还统计查询的连接，再读取近期心跳。
		rows.Close()
		if err != nil {
			return nil, err
		}
		recent, err := s.recentHeartbeats(ctx, batch, heartbeatLimit)
		if err != nil {
			return nil, err
		}
		for _, m := range batch {
			v := model.Summary{Status: "unknown", Heartbeats: []model.Heartbeat{}}
			items := recent[m.ID]
			if len(items) > 0 {
				latest := items[0]
				v.Latest = &latest
				v.Status = latest.Status
			}
			// 展示按时间从左向右排列，分页接口则保持最新优先。
			for i := len(items) - 1; i >= 0; i-- {
				v.Heartbeats = append(v.Heartbeats, items[i])
			}
			if !m.Enabled {
				v.Status = "paused"
			}
			stat := stats[m.ID]
			if stat.total > 0 {
				ratio := float64(stat.up) * 100 / float64(stat.total)
				v.Uptime = &ratio
			}
			if stat.average.Valid && !m.Invert {
				v.Average = &stat.average.Float64
			}
			result[m.ID] = v
		}
	}
	return result, nil
}

// recentHeartbeats 合并小批次查询，每项仍先使用索引排序及 LIMIT，不扫描全部历史。
func (s *Store) recentHeartbeats(ctx context.Context, monitors []model.Monitor, limit int) (map[int64][]model.Heartbeat, error) {
	result := make(map[int64][]model.Heartbeat, len(monitors))
	if len(monitors) == 0 {
		return result, nil
	}
	parts := make([]string, 0, len(monitors))
	args := []any{}
	for _, m := range monitors {
		parts = append(parts, "SELECT id,monitor_id,time,data FROM (SELECT id,monitor_id,time,data FROM heartbeats WHERE monitor_id=? ORDER BY time DESC,id DESC LIMIT ?)")
		args = append(args, m.ID, limit)
	}
	query := "SELECT id,monitor_id,data FROM (" + strings.Join(parts, " UNION ALL ") + ") ORDER BY monitor_id,time DESC,id DESC"
	if s.dialect == "postgres" {
		values := strings.TrimSuffix(strings.Repeat("(CAST(? AS BIGINT)),", len(monitors)), ",")
		args = []any{}
		for _, m := range monitors {
			args = append(args, m.ID)
		}
		args = append(args, limit)
		query = "SELECT h.id,h.monitor_id,h.data FROM (VALUES " + values + ") AS selected(id) CROSS JOIN LATERAL (SELECT id,monitor_id,time,data FROM heartbeats WHERE monitor_id=selected.id ORDER BY time DESC,id DESC LIMIT ?) AS h ORDER BY h.monitor_id,h.time DESC,h.id DESC"
	}
	rows, err := s.DB.QueryContext(ctx, s.Q(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, monitorID int64
		var data string
		if err = rows.Scan(&id, &monitorID, &data); err != nil {
			return nil, err
		}
		var h model.Heartbeat
		if err = json.Unmarshal([]byte(data), &h); err != nil {
			return nil, err
		}
		h.ID = id
		result[monitorID] = append(result[monitorID], h)
	}
	return result, rows.Err()
}

func (s *Store) Heartbeats(ctx context.Context, id, before int64, limit int) ([]model.Heartbeat, error) {
	q := "SELECT id,data FROM heartbeats WHERE monitor_id=?"
	args := []any{id}
	if before > 0 {
		q += " AND id<?"
		args = append(args, before)
	}
	q += " ORDER BY time DESC,id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, s.Q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.Heartbeat{}
	for rows.Next() {
		var h model.Heartbeat
		var hid int64
		var data string
		if err = rows.Scan(&hid, &data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(data), &h); err != nil {
			return nil, err
		}
		h.ID = hid
		items = append(items, h)
	}
	return items, rows.Err()
}

func (s *Store) Logs(ctx context.Context, id, before int64, limit int) ([]model.Log, error) {
	q := "SELECT id,monitor_id,time,status,message FROM monitor_logs WHERE monitor_id=?"
	args := []any{id}
	if before > 0 {
		q += " AND id<?"
		args = append(args, before)
	}
	q += " ORDER BY time DESC,id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, s.Q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.Log{}
	for rows.Next() {
		var l model.Log
		if err = rows.Scan(&l.ID, &l.MonitorID, &l.Time, &l.Status, &l.Message); err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	return items, rows.Err()
}

// ClearHistory 的表名仅由内部白名单确定，监控配置始终保留。
func (s *Store) ClearHistory(ctx context.Context, id int64, kind string) (int64, error) {
	table := "heartbeats"
	if kind == "logs" {
		table = "monitor_logs"
	}
	if _, err := s.Monitor(ctx, id); err != nil {
		return 0, err
	}
	result, err := s.DB.ExecContext(ctx, s.Q("DELETE FROM "+table+" WHERE monitor_id=?"), id)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
