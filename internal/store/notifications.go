package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

func (s *Store) migrateNotifications(ctx context.Context, tx *sql.Tx) error {
	id := "INTEGER PRIMARY KEY AUTOINCREMENT"
	if s.dialect == "postgres" {
		id = "BIGSERIAL PRIMARY KEY"
	}
	for _, q := range []string{
		"CREATE TABLE notification_channels (id " + id + ", data TEXT NOT NULL)",
		"CREATE TABLE alert_states (monitor_id BIGINT NOT NULL REFERENCES monitors(id) ON DELETE CASCADE, kind TEXT NOT NULL, data TEXT NOT NULL, PRIMARY KEY(monitor_id,kind))",
		"CREATE TABLE notification_deliveries (id " + id + ", monitor_id BIGINT NOT NULL, channel_id BIGINT NOT NULL, kind TEXT NOT NULL, event TEXT NOT NULL, status TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, next_attempt BIGINT NOT NULL, created BIGINT NOT NULL, error TEXT NOT NULL DEFAULT '')",
		"CREATE INDEX notification_created ON notification_deliveries(created)",
		"CREATE INDEX notification_due ON notification_deliveries(status,next_attempt,id)",
		"CREATE INDEX notification_order ON notification_deliveries(monitor_id,channel_id,id)",
		"INSERT INTO schema_version(version) VALUES(3)",
	} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) NotificationChannels(ctx context.Context) ([]model.NotificationChannel, error) {
	rows, err := s.readDB.QueryContext(ctx, "SELECT id,data FROM notification_channels ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.NotificationChannel{}
	for rows.Next() {
		var c model.NotificationChannel
		var id int64
		var data string
		if err = rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(data), &c); err != nil {
			return nil, err
		}
		c.ID = id
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *Store) NotificationChannel(ctx context.Context, id int64) (model.NotificationChannel, error) {
	var c model.NotificationChannel
	var data string
	err := s.readDB.QueryRowContext(ctx, s.Q("SELECT data FROM notification_channels WHERE id=?"), id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal([]byte(data), &c)
	c.ID = id
	return c, err
}

func (s *Store) SaveNotificationChannel(ctx context.Context, c model.NotificationChannel) (model.NotificationChannel, error) {
	if c.ID == 0 {
		err := s.DB.QueryRowContext(ctx, s.Q("INSERT INTO notification_channels(data) VALUES(?) RETURNING id"), encode(c)).Scan(&c.ID)
		return c, err
	}
	result, err := s.DB.ExecContext(ctx, s.Q("UPDATE notification_channels SET data=? WHERE id=?"), encode(c), c.ID)
	if err != nil {
		return c, err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		err = ErrNotFound
	}
	return c, err
}

func (s *Store) DeleteNotificationChannel(ctx context.Context, id int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := "SELECT id FROM notification_channels WHERE id=?"
	if s.dialect == "postgres" {
		q += " FOR UPDATE"
	}
	var found int64
	if err = tx.QueryRowContext(ctx, s.Q(q), id).Scan(&found); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT data FROM monitors")
	if err != nil {
		return err
	}
	for rows.Next() {
		var data string
		var m model.Monitor
		if err = rows.Scan(&data); err == nil {
			err = json.Unmarshal([]byte(data), &m)
		}
		if err != nil {
			rows.Close()
			return err
		}
		if m.Alerts != nil {
			for _, rule := range []model.AlertRule{m.Alerts.Service, m.Alerts.Certificate} {
				for _, channelID := range rule.ChannelIDs {
					if channelID == id {
						rows.Close()
						return fmt.Errorf("%w：通知渠道仍被监控引用，请先取消关联", ErrInvalid)
					}
				}
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, s.Q("DELETE FROM notification_channels WHERE id=?"), id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, s.Q("UPDATE notification_deliveries SET status='cancelled' WHERE channel_id=? AND status IN ('pending','sending')"), id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) validateAlertChannels(ctx context.Context, tx *sql.Tx, m model.Monitor) error {
	if m.Alerts == nil {
		return nil
	}
	seen := map[int64]bool{}
	for _, rule := range []model.AlertRule{m.Alerts.Service, m.Alerts.Certificate} {
		for _, id := range rule.ChannelIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			var found int64
			q := "SELECT id FROM notification_channels WHERE id=?"
			if s.dialect == "postgres" {
				q += " FOR SHARE"
			}
			err := tx.QueryRowContext(ctx, s.Q(q), id).Scan(&found)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w：通知渠道不存在，请重新选择", ErrInvalid)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type alertState struct {
	Config         string `json:"config"`
	Active         bool   `json:"active"`
	Started        int64  `json:"started"`
	LastQueued     int64  `json:"lastQueued"`
	CertExpiry     int64  `json:"certExpiry"`
	Expired        bool   `json:"expired"`
	FailureCount   int    `json:"failureCount"`
	FailureStarted int64  `json:"failureStarted"`
}

func alertConfig(m model.Monitor, rule model.AlertRule) string {
	return encode(struct {
		Target, Type                   string
		Port                           int
		Invert                         bool
		HTTP                           *model.HTTPOptions
		WebSocket                      *model.WebSocketOptions
		GRPC                           *model.GRPCOptions `json:",omitempty"`
		DNSRecord, DNSServer, Expected string
		Rule                           model.AlertRule
	}{m.Target, m.Type, m.Port, m.Invert, m.HTTP, m.WebSocket, m.GRPC, m.DNSRecord, m.DNSServer, m.Expected, rule})
}

func (s *Store) readAlertState(ctx context.Context, tx *sql.Tx, id int64, kind string) (alertState, error) {
	var state alertState
	var data string
	err := tx.QueryRowContext(ctx, s.Q("SELECT data FROM alert_states WHERE monitor_id=? AND kind=?"), id, kind).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	if err == nil {
		err = json.Unmarshal([]byte(data), &state)
	}
	return state, err
}

func alertRules(m model.Monitor) map[string]model.AlertRule {
	v := model.AlertOptions{}
	if m.Alerts != nil {
		v = *m.Alerts
	}
	if v.Service.FailureThreshold == 0 {
		v.Service.FailureThreshold = 1
	}
	return map[string]model.AlertRule{"service": v.Service, "certificate": v.Certificate}
}

// 编辑目标或规则时使旧队列失效；单纯重启保留状态，避免重复首次告警。
func (s *Store) resetChangedAlerts(ctx context.Context, tx *sql.Tx, m model.Monitor) error {
	for kind, rule := range alertRules(m) {
		state, err := s.readAlertState(ctx, tx, m.ID, kind)
		if err != nil {
			return err
		}
		if m.Enabled && rule.Enabled && state.Config == alertConfig(m, rule) {
			continue
		}
		if _, err = tx.ExecContext(ctx, s.Q("DELETE FROM alert_states WHERE monitor_id=? AND kind=?"), m.ID, kind); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, s.Q("UPDATE notification_deliveries SET status='cancelled' WHERE monitor_id=? AND kind=? AND status IN ('pending','sending')"), m.ID, kind); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) evaluateAlerts(ctx context.Context, tx *sql.Tx, m model.Monitor, h model.Heartbeat) error {
	return s.evaluateAlertRules(ctx, tx, m, h, false)
}

func (s *Store) evaluateAlertRules(ctx context.Context, tx *sql.Tx, m model.Monitor, h model.Heartbeat, timerOnly bool) error {
	for kind, rule := range alertRules(m) {
		if !rule.Enabled {
			continue
		}
		state, err := s.readAlertState(ctx, tx, m.ID, kind)
		if err != nil {
			return err
		}
		stamp := alertConfig(m, rule)
		if timerOnly && state.Config != stamp {
			continue
		}
		if state.Config != stamp {
			state = alertState{Config: stamp}
		}
		event := model.NotificationEvent{MonitorID: m.ID, MonitorName: m.Name, Target: m.Target, MonitorType: m.Type, Status: h.Status, Error: h.Message, Time: h.Time, Latency: h.Latency}
		if h.Status == "up" {
			event.Error = ""
		}
		active := h.Status == "down"
		alertType := "service_down"
		changedCert := false
		if kind == "service" {
			// 只统计真实检测结果，定时提醒不能把一次失败累计成多次失败。
			if !timerOnly {
				if h.Status == "down" {
					if state.FailureCount == 0 {
						state.FailureStarted = h.Time
					}
					state.FailureCount = min(state.FailureCount+1, rule.FailureThreshold)
				} else {
					state.FailureCount = 0
					state.FailureStarted = 0
				}
			}
			active = active && state.FailureCount >= rule.FailureThreshold
		}
		if kind == "certificate" {
			// 失败检测可能没有 TLS 数据，保留上次观测的到期时间继续判断。
			changedCert = h.CertExpiry > 0 && state.CertExpiry != h.CertExpiry
			if h.CertExpiry > 0 {
				state.CertExpiry = h.CertExpiry
			}
			active = state.CertExpiry > 0 && state.CertExpiry-h.Time <= int64(rule.Days)*86400000
			event.CertExpiry = state.CertExpiry
			alertType = "certificate_expiring"
			if state.CertExpiry <= h.Time {
				alertType = "certificate_expired"
			}
		}
		first := active && (!state.Active || changedCert)
		expired := kind == "certificate" && active && alertType == "certificate_expired" && !state.Expired
		repeat := active && rule.RepeatMinutes > 0 && h.Time-state.LastQueued >= int64(rule.RepeatMinutes)*60000
		if first {
			state.Started = h.Time
			if kind == "service" {
				state.Started = state.FailureStarted
			}
		}
		event.Started = state.Started
		event.AlertType = alertType
		send := first || expired || repeat
		event.Repeat = send && !first && !expired
		if !active && state.Active && rule.Recovery {
			send = true
			event.AlertType = kind + "_recovered"
		}
		if send {
			for _, channelID := range rule.ChannelIDs {
				// 渠道故障时合并重复提醒，避免积压成恢复后的消息洪峰。
				if event.Repeat {
					var pending bool
					if err = tx.QueryRowContext(ctx, s.Q("SELECT EXISTS(SELECT 1 FROM notification_deliveries WHERE monitor_id=? AND channel_id=? AND kind=? AND status IN ('pending','sending'))"), m.ID, channelID, kind).Scan(&pending); err != nil {
						return err
					}
					if pending {
						continue
					}
				}
				if _, err = tx.ExecContext(ctx, s.Q("INSERT INTO notification_deliveries(monitor_id,channel_id,kind,event,status,next_attempt,created) VALUES(?,?,?,?,'pending',?,?)"), m.ID, channelID, kind, encode(event), h.Time, h.Time); err != nil {
					return err
				}
			}
			state.LastQueued = h.Time
		}
		state.Active = active
		state.Expired = kind == "certificate" && active && state.CertExpiry <= h.Time
		if _, err = tx.ExecContext(ctx, s.Q("INSERT INTO alert_states(monitor_id,kind,data) VALUES(?,?,?) ON CONFLICT(monitor_id,kind) DO UPDATE SET data=excluded.data"), m.ID, kind, encode(state)); err != nil {
			return err
		}
	}
	return nil
}

// 到期与重复提醒按分钟检查，不依赖下一次检测；从未检测的新配置不会凭空告警。
func (s *Store) EvaluateAlertTimers(ctx context.Context) error {
	monitors, err := s.Monitors(ctx)
	if err != nil {
		return err
	}
	for _, m := range monitors {
		if !m.Enabled || m.Alerts == nil || (!m.Alerts.Service.Enabled && !m.Alerts.Certificate.Enabled) {
			continue
		}
		if err = s.evaluateAlertTimer(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) evaluateAlertTimer(ctx context.Context, m model.Monitor) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := "SELECT data FROM monitors WHERE id=?"
	if s.dialect == "postgres" {
		q += " FOR UPDATE"
	}
	var data string
	if err = tx.QueryRowContext(ctx, s.Q(q), m.ID).Scan(&data); errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var current model.Monitor
	if err = json.Unmarshal([]byte(data), &current); err != nil {
		return err
	}
	current.ID = m.ID
	if !reflect.DeepEqual(current, m) {
		return nil
	}
	if err = tx.QueryRowContext(ctx, s.Q("SELECT data FROM heartbeats WHERE monitor_id=? ORDER BY time DESC,id DESC LIMIT 1"), m.ID).Scan(&data); errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var h model.Heartbeat
	if err = json.Unmarshal([]byte(data), &h); err != nil {
		return err
	}
	// 最近检测的证书可能已经被更新，定时判断只沿用持久化状态中的证书。
	h.Time, h.CertExpiry = time.Now().UnixMilli(), 0
	if err = s.evaluateAlertRules(ctx, tx, m, h, true); err != nil {
		return err
	}
	return tx.Commit()
}

// ClaimNotification 使用租约恢复意外退出中的任务；同一监控/渠道严格按入队顺序发送。
func (s *Store) ClaimNotification(ctx context.Context, now int64) (*model.NotificationDelivery, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	q := "SELECT id,monitor_id,channel_id,kind,event,attempts FROM notification_deliveries d WHERE status IN ('pending','sending') AND next_attempt<=? AND NOT EXISTS (SELECT 1 FROM notification_deliveries p WHERE p.monitor_id=d.monitor_id AND p.channel_id=d.channel_id AND p.id<d.id AND p.status IN ('pending','sending')) ORDER BY id LIMIT 1"
	if s.dialect == "postgres" {
		q += " FOR UPDATE SKIP LOCKED"
	}
	var d model.NotificationDelivery
	var event string
	err = tx.QueryRowContext(ctx, s.Q(q), now).Scan(&d.ID, &d.MonitorID, &d.ChannelID, &d.Kind, &event, &d.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(event), &d.Event); err != nil {
		return nil, err
	}
	d.Attempts++
	if _, err = tx.ExecContext(ctx, s.Q("UPDATE notification_deliveries SET status='sending',attempts=?,next_attempt=? WHERE id=?"), d.Attempts, now+60000, d.ID); err != nil {
		return nil, err
	}
	return &d, tx.Commit()
}

func (s *Store) FinishNotification(ctx context.Context, d model.NotificationDelivery, status, message string, next int64) error {
	_, err := s.DB.ExecContext(ctx, s.Q("UPDATE notification_deliveries SET status=?,error=?,next_attempt=? WHERE id=? AND status='sending' AND attempts=?"), status, message, next, d.ID, d.Attempts)
	return err
}

func (s *Store) NotificationRecords(ctx context.Context, before int64) ([]model.NotificationDelivery, error) {
	query := "SELECT id,monitor_id,channel_id,kind,event,status,attempts,next_attempt,error FROM notification_deliveries"
	args := []any{}
	if before > 0 {
		query += " WHERE id<?"
		args = append(args, before)
	}
	rows, err := s.readDB.QueryContext(ctx, s.Q(query+" ORDER BY id DESC LIMIT 50"), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.NotificationDelivery{}
	for rows.Next() {
		var d model.NotificationDelivery
		var data string
		if err = rows.Scan(&d.ID, &d.MonitorID, &d.ChannelID, &d.Kind, &data, &d.Status, &d.Attempts, &d.NextAttempt, &d.Error); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(data), &d.Event); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

// NotificationAllowed 在发送前重新读取开关，暂停和删除后取消尚未发出的任务。
func (s *Store) NotificationAllowed(ctx context.Context, d model.NotificationDelivery) (bool, error) {
	if d.Kind == "test" {
		return true, nil
	}
	m, err := s.Monitor(ctx, d.MonitorID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !m.Enabled {
		return false, nil
	}
	if m.Target != d.Event.Target || m.Type != d.Event.MonitorType {
		return false, nil
	}
	rule := alertRules(m)[d.Kind]
	if !rule.Enabled {
		return false, nil
	}
	for _, id := range rule.ChannelIDs {
		if id == d.ChannelID {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) QueueNotificationTest(ctx context.Context, channelID int64, event model.NotificationEvent) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := "SELECT id FROM notification_channels WHERE id=?"
	if s.dialect == "postgres" {
		q += " FOR UPDATE"
	}
	var found int64
	if err = tx.QueryRowContext(ctx, s.Q(q), channelID).Scan(&found); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var pending bool
	if err := tx.QueryRowContext(ctx, s.Q("SELECT EXISTS(SELECT 1 FROM notification_deliveries WHERE channel_id=? AND kind='test' AND status IN ('pending','sending'))"), channelID).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return fmt.Errorf("%w：该渠道已有等待中的测试通知", ErrInvalid)
	}
	now := time.Now().UnixMilli()
	if _, err = tx.ExecContext(ctx, s.Q("INSERT INTO notification_deliveries(monitor_id,channel_id,kind,event,status,next_attempt,created) VALUES(0,?,'test',?,'pending',?,?)"), channelID, encode(event), now, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) cleanupNotifications(ctx context.Context, cutoff int64) error {
	for {
		result, err := s.DB.ExecContext(ctx, s.Q("DELETE FROM notification_deliveries WHERE id IN (SELECT id FROM notification_deliveries WHERE created<? AND status IN ('sent','failed','cancelled') ORDER BY created,id LIMIT 500)"), cutoff)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n < 500 {
			return nil
		}
	}
}
