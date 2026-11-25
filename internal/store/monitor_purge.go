package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"
)

// migrateMonitorDeletion 在版本 6 的加密迁移完成后添加待删除状态。
func (s *Store) migrateMonitorDeletion(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if s.dialect == "postgres" {
		if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(78542169)"); err != nil {
			return err
		}
	}
	var migrated bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schema_version WHERE version=7)").Scan(&migrated); err != nil || migrated {
		return err
	}
	if _, err = tx.ExecContext(ctx, "ALTER TABLE monitors ADD COLUMN deleting INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO schema_version(version) VALUES(7)"); err != nil {
		return err
	}
	return tx.Commit()
}

// RunMonitorPurge 启动时恢复待删除任务，提交新任务后唤醒，失败后定期重试。
func (s *Store) RunMonitorPurge(ctx context.Context) {
	retry := time.NewTicker(time.Minute)
	defer retry.Stop()
	for ctx.Err() == nil {
		if err := s.purgeDeletedMonitors(ctx); err != nil && ctx.Err() == nil {
			slog.Error("清理已删除监控的历史失败", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-s.purgeWake:
		case <-retry.C:
		}
	}
}

func (s *Store) purgeDeletedMonitors(ctx context.Context) error {
	for {
		var id int64
		err := s.readDB.QueryRowContext(ctx, "SELECT id FROM monitors WHERE deleting=1 ORDER BY id LIMIT 1").Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, table := range []string{"heartbeats", "monitor_logs"} {
			for {
				n, err := s.purgeMonitorHistoryBatch(ctx, table, id)
				if err != nil {
					return err
				}
				if n == 0 {
					break
				}
				// 每批独立提交，短暂停顿让其他任务有机会写入。
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Millisecond):
				}
			}
		}
		// 确认历史已清空，避免最后一步触发大量级联删除。
		query := `DELETE FROM monitors WHERE id=? AND deleting=1
			AND NOT EXISTS(SELECT 1 FROM heartbeats WHERE monitor_id=monitors.id)
			AND NOT EXISTS(SELECT 1 FROM monitor_logs WHERE monitor_id=monitors.id)`
		if _, err = s.DB.ExecContext(ctx, s.Q(query), id); err != nil {
			return err
		}
	}
}

// purgeMonitorHistoryBatch 的表名由调用方固定，按现有监控历史索引选取最多 500 行。
func (s *Store) purgeMonitorHistoryBatch(ctx context.Context, table string, id int64) (int64, error) {
	query := "DELETE FROM " + table + " WHERE id IN (SELECT id FROM " + table + " WHERE monitor_id=? ORDER BY time DESC,id DESC LIMIT 500)"
	result, err := s.DB.ExecContext(ctx, s.Q(query), id)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
