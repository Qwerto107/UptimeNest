package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

var panelGroupID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// normalizePanelGroups 以展示分组为唯一顺序来源，兼容字段 MonitorIDs 仅保存去重后的扁平索引。
func normalizePanelGroups(p *model.Panel) error {
	if len(p.Groups) > 100 {
		return fmt.Errorf("%w：单个面板最多 100 个展示分组", ErrInvalid)
	}
	groups := map[string]bool{}
	monitors := map[int64]bool{}
	p.MonitorIDs = []int64{}
	for i := range p.Groups {
		g := &p.Groups[i]
		g.Name = strings.TrimSpace(g.Name)
		if !panelGroupID.MatchString(g.ID) || groups[g.ID] {
			return fmt.Errorf("%w：展示分组编号无效或重复", ErrInvalid)
		}
		if g.Name == "" || len(g.Name) > 200 {
			return fmt.Errorf("%w：展示分组名称不能为空，且不能超过 200 字节", ErrInvalid)
		}
		groups[g.ID] = true
		if g.MonitorIDs == nil {
			g.MonitorIDs = []int64{}
		}
		for _, id := range g.MonitorIDs {
			if id <= 0 || monitors[id] {
				return fmt.Errorf("%w：同一面板中的监控不能重复，编号需为正整数", ErrInvalid)
			}
			monitors[id] = true
			p.MonitorIDs = append(p.MonitorIDs, id)
		}
	}
	if len(p.MonitorIDs) > 1000 {
		return fmt.Errorf("%w：单个面板最多展示 1000 项监控", ErrInvalid)
	}
	return nil
}

// legacyGroupData 只用于旧面板首次转为独立分组，后续不再跟随后台分组变化。
func (s *Store) legacyGroupData(ctx context.Context, tx *sql.Tx) (map[int64]int64, map[int64]string, error) {
	monitors := map[int64]int64{}
	rows, err := tx.QueryContext(ctx, "SELECT id,data FROM monitors")
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id int64
		var data string
		if err = rows.Scan(&id, &data); err != nil {
			rows.Close()
			return nil, nil, err
		}
		var m model.Monitor
		// 旧结构迁移早于加密初始化；运行时只接受已经加密的配置。
		if s.secrets == nil {
			err = json.Unmarshal([]byte(data), &m)
		} else {
			err = s.openJSON(secretDomain("monitors", id), data, &m)
		}
		if err != nil {
			rows.Close()
			return nil, nil, err
		}
		monitors[id] = m.GroupID
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	names := map[int64]string{0: "未分组"}
	rows, err = tx.QueryContext(ctx, "SELECT id,name FROM monitor_groups")
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id int64
		var name string
		if err = rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, nil, err
		}
		names[id] = name
	}
	err = rows.Err()
	rows.Close()
	return monitors, names, err
}

func legacyPanelGroups(ids []int64, monitors map[int64]int64, names map[int64]string) []model.PanelGroup {
	result := []model.PanelGroup{}
	indices := map[int64]int{}
	seen := map[int64]bool{}
	for _, id := range ids {
		group, exists := monitors[id]
		if !exists || seen[id] {
			continue
		}
		seen[id] = true
		index, exists := indices[group]
		if !exists {
			name := names[group]
			if name == "" {
				name = "未分组"
			}
			index = len(result)
			indices[group] = index
			result = append(result, model.PanelGroup{ID: "legacy-" + strconv.FormatInt(group, 10), Name: name, MonitorIDs: []int64{}})
		}
		result[index].MonitorIDs = append(result[index].MonitorIDs, id)
	}
	return result
}

// migratePanelGroups 在同一迁移事务中保存旧分组快照，不改变监控或其心跳历史。
func (s *Store) migratePanelGroups(ctx context.Context, tx *sql.Tx) error {
	monitors, names, err := s.legacyGroupData(ctx, tx)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,data FROM panels")
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
		id := p.ID
		if err = json.Unmarshal([]byte(data), &p); err != nil {
			rows.Close()
			return err
		}
		p.ID = id
		if p.Groups == nil {
			p.Groups = legacyPanelGroups(p.MonitorIDs, monitors, names)
			panels = append(panels, p)
		}
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
	return nil
}
