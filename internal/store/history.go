package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
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
	v := model.Summary{Status: "unknown", Heartbeats: []model.Heartbeat{}}
	items, err := s.Heartbeats(ctx, m.ID, 0, heartbeatLimit)
	if err != nil {
		return v, err
	}
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
	var total, up int64
	var average sql.NullFloat64
	err = s.DB.QueryRowContext(ctx, s.Q("SELECT COUNT(*), COALESCE(SUM(CASE WHEN status='up' THEN 1 ELSE 0 END),0), AVG(CASE WHEN status='up' THEN latency ELSE NULL END) FROM heartbeats WHERE monitor_id=? AND time>=?"), m.ID, time.Now().Add(-24*time.Hour).UnixMilli()).Scan(&total, &up, &average)
	if err != nil {
		return v, err
	}
	if total > 0 {
		ratio := float64(up) * 100 / float64(total)
		v.Uptime = &ratio
	}
	if average.Valid && !m.Invert {
		v.Average = &average.Float64
	}
	return v, nil
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
