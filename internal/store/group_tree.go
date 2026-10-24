package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

// lockGroups 串行化分组结构写入，避免两个并发请求互相设为父级而形成环。
func (s *Store) lockGroups(ctx context.Context, tx *sql.Tx) error {
	if s.dialect == "postgres" {
		_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(78542171)")
		return err
	}
	return nil
}

func validateGroupParent(ctx context.Context, tx *sql.Tx, g model.Group) error {
	if g.ParentID < 0 {
		return fmt.Errorf("%w：父分组无效", ErrInvalid)
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,parent_id FROM monitor_groups")
	if err != nil {
		return err
	}
	parents := map[int64]int64{}
	for rows.Next() {
		var id, parent int64
		if err = rows.Scan(&id, &parent); err != nil {
			rows.Close()
			return err
		}
		parents[id] = parent
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if _, exists := parents[g.ID]; g.ID != 0 && !exists {
		return ErrNotFound
	}
	seen := map[int64]bool{}
	if g.ID != 0 {
		seen[g.ID] = true
	}
	// 沿候选父节点向根遍历，拒绝自身、子孙以及已有的异常环。
	for id := g.ParentID; id != 0; {
		if seen[id] {
			return fmt.Errorf("%w：不能将分组放入自身或其子分组", ErrInvalid)
		}
		seen[id] = true
		parent, exists := parents[id]
		if !exists {
			return fmt.Errorf("%w：父分组不存在，请刷新后重试", ErrInvalid)
		}
		id = parent
	}
	return nil
}
