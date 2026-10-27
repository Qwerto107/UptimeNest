// Package store 封装 SQLite 与 PostgreSQL 的持久化差异。
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("记录不存在")
var ErrStale = errors.New("监控配置已经更新")
var ErrConflict = errors.New("名称或访问路径已存在")
var ErrInvalid = errors.New("配置无效")

// sqlBatchSize 为批量查询预留参数余量，兼容两种数据库的参数数量限制。
const sqlBatchSize = 500

type Store struct {
	DB            *sql.DB
	dialect       string
	publicVersion atomic.Uint64
}

func (s *Store) PublicVersion() uint64 { return s.publicVersion.Load() }

// commitChange 使配置变更提交后立即淘汰本进程的公开响应缓存。
func (s *Store) commitChange(tx *sql.Tx) error {
	err := tx.Commit()
	// 取消或连接中断时提交结果可能未知，仍使缓存失效以免保留旧配置。
	s.publicVersion.Add(1)
	return err
}

// Open 初始化连接与版本化表结构；SQLite 使用单连接避免并发写锁冲突。
func Open(ctx context.Context, dialect, dsn string) (*Store, error) {
	driver := "pgx"
	if dialect == "sqlite" {
		driver = "sqlite"
		if !strings.HasPrefix(dsn, "file:") && dsn != ":memory:" {
			if err := os.MkdirAll(filepath.Dir(dsn), 0700); err != nil {
				return nil, err
			}
		}
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if dialect == "sqlite" {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(12)
	}
	s := &Store{DB: db, dialect: dialect}
	if err = db.PingContext(ctx); err == nil {
		err = s.migrate(ctx)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Q 只转换内部固定 SQL 的占位符，用户输入始终通过参数传递。
func (s *Store) Q(query string) string {
	if s.dialect != "postgres" {
		return query
	}
	var b strings.Builder
	n := 0
	for _, c := range query {
		if c == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func (s *Store) migrate(ctx context.Context) error {
	if s.dialect == "sqlite" {
		for _, q := range []string{"PRAGMA foreign_keys=ON", "PRAGMA journal_mode=WAL", "PRAGMA busy_timeout=5000"} {
			if _, err := s.DB.ExecContext(ctx, q); err != nil {
				return err
			}
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// 事务级锁让多个 PostgreSQL 实例启动时不会同时迁移。
	if s.dialect == "postgres" {
		if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(78542169)"); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY)"); err != nil {
		return err
	}
	var version int
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version); err != nil {
		return err
	}
	if version > 3 {
		return errors.New("数据库版本高于当前程序支持的版本")
	}
	if version == 0 {
		id := "INTEGER PRIMARY KEY AUTOINCREMENT"
		if s.dialect == "postgres" {
			id = "BIGSERIAL PRIMARY KEY"
		}
		queries := []string{
			"CREATE TABLE monitors (id " + id + ", data TEXT NOT NULL)",
			"CREATE TABLE monitor_groups (id " + id + ", name TEXT NOT NULL UNIQUE)",
			"CREATE TABLE panels (id " + id + ", slug TEXT NOT NULL UNIQUE, is_default INTEGER NOT NULL DEFAULT 0, data TEXT NOT NULL)",
			"CREATE UNIQUE INDEX panels_one_default ON panels(is_default) WHERE is_default = 1",
			"CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)",
			"CREATE TABLE account (id INTEGER PRIMARY KEY CHECK (id = 1), password_hash TEXT NOT NULL)",
			"CREATE TABLE sessions (token TEXT PRIMARY KEY, expires BIGINT NOT NULL)",
			"CREATE INDEX sessions_expiry ON sessions(expires)",
			"CREATE TABLE heartbeats (id " + id + ", monitor_id BIGINT NOT NULL REFERENCES monitors(id) ON DELETE CASCADE, time BIGINT NOT NULL, status TEXT NOT NULL, latency DOUBLE PRECISION NOT NULL, data TEXT NOT NULL)",
			"CREATE INDEX heartbeats_monitor_time ON heartbeats(monitor_id, time DESC, id DESC)",
			"CREATE INDEX heartbeats_time ON heartbeats(time)",
			"CREATE TABLE monitor_logs (id " + id + ", monitor_id BIGINT NOT NULL REFERENCES monitors(id) ON DELETE CASCADE, time BIGINT NOT NULL, status TEXT NOT NULL, message TEXT NOT NULL)",
			"CREATE INDEX logs_monitor_time ON monitor_logs(monitor_id, time DESC, id DESC)",
			"CREATE INDEX logs_time ON monitor_logs(time)",
			"INSERT INTO schema_version(version) VALUES(1)",
		}
		for _, q := range queries {
			if _, err = tx.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("初始化数据库: %w", err)
			}
		}
	}
	if version < 2 {
		if _, err = tx.ExecContext(ctx, "ALTER TABLE monitor_groups ADD COLUMN parent_id BIGINT NOT NULL DEFAULT 0"); err != nil {
			return err
		}
		// 在后台分组结构改变前，冻结旧面板的分组名称与展示顺序。
		if err = s.migratePanelGroups(ctx, tx); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_version(version) VALUES(2)"); err != nil {
			return err
		}
	}
	if version < 3 {
		if err = s.migrateNotifications(ctx, tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func encode(v any) string { b, _ := json.Marshal(v); return string(b) }

func (s *Store) Monitors(ctx context.Context) ([]model.Monitor, error) {
	return s.monitors(ctx, "SELECT id, data FROM monitors ORDER BY id")
}

// MonitorsByIDs 仅读取面板选中的监控，避免解码无关配置和凭据。
func (s *Store) MonitorsByIDs(ctx context.Context, ids []int64) ([]model.Monitor, error) {
	items := []model.Monitor{}
	for start := 0; start < len(ids); start += sqlBatchSize {
		batch := ids[start:min(start+sqlBatchSize, len(ids))]
		args := make([]any, len(batch))
		for i, id := range batch {
			args[i] = id
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		monitors, err := s.monitors(ctx, "SELECT id, data FROM monitors WHERE id IN ("+placeholders+") ORDER BY id", args...)
		if err != nil {
			return nil, err
		}
		items = append(items, monitors...)
	}
	return items, nil
}

func (s *Store) monitors(ctx context.Context, query string, args ...any) ([]model.Monitor, error) {
	rows, err := s.DB.QueryContext(ctx, s.Q(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.Monitor{}
	for rows.Next() {
		var m model.Monitor
		var data string
		if err := rows.Scan(&m.ID, &data); err != nil {
			return nil, err
		}
		id := m.ID
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			return nil, err
		}
		m.ID = id
		items = append(items, m)
	}
	return items, rows.Err()
}

func (s *Store) Monitor(ctx context.Context, id int64) (model.Monitor, error) {
	var m model.Monitor
	var data string
	err := s.DB.QueryRowContext(ctx, s.Q("SELECT data FROM monitors WHERE id=?"), id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return m, ErrNotFound
	}
	if err != nil {
		return m, err
	}
	err = json.Unmarshal([]byte(data), &m)
	m.ID = id
	return m, err
}

// SaveMonitor 在同一事务内检查所属分组，避免出现悬空分组引用。
func (s *Store) SaveMonitor(ctx context.Context, m model.Monitor) (model.Monitor, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return m, err
	}
	defer tx.Rollback()
	if err = s.validateAlertChannels(ctx, tx, m); err != nil {
		return m, err
	}
	if m.GroupID != 0 {
		var id int64
		q := "SELECT id FROM monitor_groups WHERE id=?"
		if s.dialect == "postgres" {
			q += " FOR SHARE"
		}
		if err = tx.QueryRowContext(ctx, s.Q(q), m.GroupID).Scan(&id); errors.Is(err, sql.ErrNoRows) {
			return m, ErrNotFound
		}
		if err != nil {
			return m, err
		}
	}
	if m.ID == 0 {
		err = tx.QueryRowContext(ctx, s.Q("INSERT INTO monitors(data) VALUES(?) RETURNING id"), encode(m)).Scan(&m.ID)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, s.Q("UPDATE monitors SET data=? WHERE id=?"), encode(m), m.ID)
		if err == nil {
			n, _ := result.RowsAffected()
			if n == 0 {
				return m, ErrNotFound
			}
		}
	}
	if err != nil {
		return m, err
	}
	if err = s.resetChangedAlerts(ctx, tx, m); err != nil {
		return m, err
	}
	return m, s.commitChange(tx)
}

func (s *Store) DeleteMonitor(ctx context.Context, id int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// 与面板保存采用同一锁序，避免删除监控时覆盖并发调整的面板分组。
	if s.dialect == "postgres" {
		if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(78542170)"); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, s.Q("DELETE FROM monitors WHERE id=?"), id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	// 面板配置同步移除已删除的监控 ID，其他监控与历史不受影响。
	rows, err := tx.QueryContext(ctx, "SELECT id, data FROM panels")
	if err != nil {
		return err
	}
	panels := []model.Panel{}
	for rows.Next() {
		var p model.Panel
		var data string
		if err = rows.Scan(&p.ID, &data); err != nil {
			rows.Close()
			return err
		}
		pid := p.ID
		if err = json.Unmarshal([]byte(data), &p); err != nil {
			rows.Close()
			return err
		}
		p.ID = pid
		kept := []int64{}
		for _, mid := range p.MonitorIDs {
			if mid != id {
				kept = append(kept, mid)
			}
		}
		p.MonitorIDs = kept
		for i := range p.Groups {
			ids := []int64{}
			for _, mid := range p.Groups[i].MonitorIDs {
				if mid != id {
					ids = append(ids, mid)
				}
			}
			p.Groups[i].MonitorIDs = ids
		}
		panels = append(panels, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, p := range panels {
		if _, err = tx.ExecContext(ctx, s.Q("UPDATE panels SET data=? WHERE id=?"), encode(p), p.ID); err != nil {
			return err
		}
	}
	return s.commitChange(tx)
}

func (s *Store) Groups(ctx context.Context) ([]model.Group, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, name, parent_id FROM monitor_groups ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.Group{}
	for rows.Next() {
		var g model.Group
		if err = rows.Scan(&g.ID, &g.Name, &g.ParentID); err != nil {
			return nil, err
		}
		items = append(items, g)
	}
	return items, rows.Err()
}

func (s *Store) SaveGroup(ctx context.Context, g model.Group) (model.Group, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return g, err
	}
	defer tx.Rollback()
	if err = s.lockGroups(ctx, tx); err != nil {
		return g, err
	}
	if err = validateGroupParent(ctx, tx, g); err != nil {
		return g, err
	}
	if g.ID == 0 {
		err = tx.QueryRowContext(ctx, s.Q("INSERT INTO monitor_groups(name,parent_id) VALUES(?,?) RETURNING id"), g.Name, g.ParentID).Scan(&g.ID)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, s.Q("UPDATE monitor_groups SET name=?,parent_id=? WHERE id=?"), g.Name, g.ParentID, g.ID)
		if err == nil {
			n, _ := result.RowsAffected()
			if n == 0 {
				return g, ErrNotFound
			}
		}
	}
	if err != nil {
		return g, conflict(err)
	}
	return g, s.commitChange(tx)
}

// DeleteGroup 将组内监控移到未分组，不删除监控或其历史数据。
func (s *Store) DeleteGroup(ctx context.Context, id int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.lockGroups(ctx, tx); err != nil {
		return err
	}
	var parentID int64
	if err = tx.QueryRowContext(ctx, s.Q("SELECT parent_id FROM monitor_groups WHERE id=?"), id).Scan(&parentID); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	// 子分组上移一级，组内直属监控仍移到未分组；独立面板分组不受影响。
	if _, err = tx.ExecContext(ctx, s.Q("UPDATE monitor_groups SET parent_id=? WHERE parent_id=?"), parentID, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, s.Q("DELETE FROM monitor_groups WHERE id=?"), id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	q := "SELECT id, data FROM monitors"
	if s.dialect == "postgres" {
		q += " FOR UPDATE"
	}
	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	changed := []model.Monitor{}
	for rows.Next() {
		var m model.Monitor
		var data string
		if err = rows.Scan(&m.ID, &data); err != nil {
			rows.Close()
			return err
		}
		mid := m.ID
		if err = json.Unmarshal([]byte(data), &m); err != nil {
			rows.Close()
			return err
		}
		m.ID = mid
		if m.GroupID == id {
			m.GroupID = 0
			changed = append(changed, m)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, m := range changed {
		if _, err = tx.ExecContext(ctx, s.Q("UPDATE monitors SET data=? WHERE id=?"), encode(m), m.ID); err != nil {
			return err
		}
	}
	return s.commitChange(tx)
}

func (s *Store) Panels(ctx context.Context) ([]model.Panel, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, slug, is_default, data FROM panels ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.Panel{}
	for rows.Next() {
		var p model.Panel
		var data, slug string
		var id int64
		var d int
		if err = rows.Scan(&id, &slug, &d, &data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(data), &p); err != nil {
			return nil, err
		}
		p.ID = id
		p.Slug = slug
		p.IsDefault = d == 1
		items = append(items, p)
	}
	return items, rows.Err()
}

func (s *Store) PanelBySlug(ctx context.Context, slug string) (model.Panel, error) {
	var p model.Panel
	var data string
	var id int64
	var isDefault int
	err := s.DB.QueryRowContext(ctx, s.Q("SELECT id, is_default, data FROM panels WHERE slug=?"), slug).Scan(&id, &isDefault, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal([]byte(data), &p); err != nil {
		return p, err
	}
	p.ID = id
	p.Slug = slug
	p.IsDefault = isDefault == 1
	return p, nil
}

func (s *Store) SavePanel(ctx context.Context, p model.Panel) (model.Panel, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return p, err
	}
	defer tx.Rollback()
	// 所有面板写入均串行化，保证默认面板唯一且删除后能正确递补。
	if s.dialect == "postgres" {
		if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(78542170)"); err != nil {
			return p, err
		}
	}
	if p.Groups == nil {
		groups, names, err := legacyGroupData(ctx, tx)
		if err != nil {
			return p, err
		}
		for _, id := range p.MonitorIDs {
			if _, exists := groups[id]; !exists {
				return p, ErrNotFound
			}
		}
		p.Groups = legacyPanelGroups(p.MonitorIDs, groups, names)
	}
	if err = normalizePanelGroups(&p); err != nil {
		return p, err
	}
	for _, id := range p.MonitorIDs {
		var found int64
		q := "SELECT id FROM monitors WHERE id=?"
		if s.dialect == "postgres" {
			q += " FOR SHARE"
		}
		if err = tx.QueryRowContext(ctx, s.Q(q), id).Scan(&found); errors.Is(err, sql.ErrNoRows) {
			return p, ErrNotFound
		}
		if err != nil {
			return p, err
		}
	}
	var defaults int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM panels WHERE is_default=1").Scan(&defaults); err != nil {
		return p, err
	}
	if defaults == 0 {
		p.IsDefault = true
	}
	if p.IsDefault {
		if _, err = tx.ExecContext(ctx, "UPDATE panels SET is_default=0"); err != nil {
			return p, err
		}
	}
	d := 0
	if p.IsDefault {
		d = 1
	}
	if p.ID == 0 {
		err = tx.QueryRowContext(ctx, s.Q("INSERT INTO panels(slug,is_default,data) VALUES(?,?,?) RETURNING id"), p.Slug, d, encode(p)).Scan(&p.ID)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, s.Q("UPDATE panels SET slug=?,is_default=?,data=? WHERE id=?"), p.Slug, d, encode(p), p.ID)
		if err == nil {
			n, _ := result.RowsAffected()
			if n == 0 {
				return p, ErrNotFound
			}
		}
	}
	if err != nil {
		return p, conflict(err)
	}
	// 取消当前默认面板时，仍保留一个有效默认入口。
	if _, err = tx.ExecContext(ctx, "UPDATE panels SET is_default=1 WHERE id=(SELECT MIN(id) FROM panels) AND NOT EXISTS (SELECT 1 FROM panels WHERE is_default=1)"); err != nil {
		return p, err
	}
	return p, s.commitChange(tx)
}

func (s *Store) DeletePanel(ctx context.Context, id int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if s.dialect == "postgres" {
		if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(78542170)"); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, s.Q("DELETE FROM panels WHERE id=?"), id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, "UPDATE panels SET is_default=1 WHERE id=(SELECT MIN(id) FROM panels) AND NOT EXISTS (SELECT 1 FROM panels WHERE is_default=1)"); err != nil {
		return err
	}
	return s.commitChange(tx)
}

func (s *Store) Settings(ctx context.Context) (model.Settings, error) {
	v := model.DefaultSettings()
	var data string
	err := s.DB.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='system'").Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	err = json.Unmarshal([]byte(data), &v)
	return v, err
}

func (s *Store) SaveSettings(ctx context.Context, v model.Settings) error {
	_, err := s.DB.ExecContext(ctx, s.Q("INSERT INTO settings(key,value) VALUES('system',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value"), encode(v))
	s.publicVersion.Add(1)
	return err
}

func conflict(err error) error {
	if err == nil {
		return nil
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "unique") || strings.Contains(text, "23505") {
		return ErrConflict
	}
	return err
}

// Cleanup 使用统一截止时间分批清理，批次间释放连接；中断后可再次执行。
func (s *Store) Cleanup(ctx context.Context, days int) (map[string]int64, error) {
	// 最后再次失效，避免并发读取把清理中间状态留在缓存中。
	defer s.publicVersion.Add(1)
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).UnixMilli()
	counts := map[string]int64{"heartbeats": 0, "logs": 0}
	for _, item := range []struct{ table, id, time, key string }{
		{"heartbeats", "id", "time", "heartbeats"},
		{"monitor_logs", "id", "time", "logs"},
		{"sessions", "token", "expires", ""},
	} {
		before := cutoff
		if item.table == "sessions" {
			before = time.Now().UnixMilli()
		}
		for {
			query := "DELETE FROM " + item.table + " WHERE " + item.id + " IN (SELECT " + item.id + " FROM " + item.table + " WHERE " + item.time + "<? ORDER BY " + item.time + "," + item.id + " LIMIT 500)"
			result, err := s.DB.ExecContext(ctx, s.Q(query), before)
			if err != nil {
				if item.key != "" {
					s.publicVersion.Add(1)
				}
				return counts, err
			}
			n, err := result.RowsAffected()
			if err != nil {
				if item.key != "" {
					s.publicVersion.Add(1)
				}
				return counts, err
			}
			if item.key != "" {
				counts[item.key] += n
				if n > 0 {
					s.publicVersion.Add(1)
				}
			}
			if n < 500 {
				// 并发清理可能已经删掉本批选中的行；确认没有剩余过期数据再结束。
				var remaining bool
				query = "SELECT EXISTS(SELECT 1 FROM " + item.table + " WHERE " + item.time + "<?)"
				if err = s.DB.QueryRowContext(ctx, s.Q(query), before).Scan(&remaining); err != nil {
					return counts, err
				}
				if !remaining {
					break
				}
			}
			select {
			case <-ctx.Done():
				return counts, ctx.Err()
			case <-time.After(time.Millisecond):
			}
		}
	}
	if err := s.cleanupNotifications(ctx, cutoff); err != nil {
		return counts, err
	}
	return counts, nil
}
