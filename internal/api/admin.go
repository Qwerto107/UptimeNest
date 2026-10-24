package api

import (
	"net/http"
	"strings"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

func (a *Server) monitors(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.Monitors(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	views := []model.MonitorView{}
	count := heartbeatCount(r, 60)
	for _, m := range items {
		summary, err := a.store.Summary(r.Context(), m, count)
		if err != nil {
			fail(w, err)
			return
		}
		views = append(views, model.MonitorView{Monitor: m, Summary: summary})
	}
	writeJSON(w, 200, views)
}

func (a *Server) saveMonitor(w http.ResponseWriter, r *http.Request) {
	var m model.Monitor
	if !decode(w, r, &m) {
		return
	}
	m.ID = 0
	if r.Method == http.MethodPut {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		m.ID = id
	}
	if err := validateMonitor(&m); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	m, err := a.store.SaveMonitor(r.Context(), m)
	if err != nil {
		fail(w, err)
		return
	}
	if err = a.engine.Reload(r.Context()); err != nil {
		fail(w, err)
		return
	}
	status := 200
	if r.Method == http.MethodPost {
		status = 201
	}
	writeJSON(w, status, m)
}

func (a *Server) setEnabled(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Enabled == nil {
		writeError(w, 400, "请明确指定启用或停用状态")
		return
	}
	m, err := a.store.Monitor(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	m.Enabled = *body.Enabled
	if _, err = a.store.SaveMonitor(r.Context(), m); err != nil {
		fail(w, err)
		return
	}
	if err = a.engine.Reload(r.Context()); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *Server) deleteMonitor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := a.store.DeleteMonitor(r.Context(), id); err != nil {
		fail(w, err)
		return
	}
	if err := a.engine.Reload(r.Context()); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *Server) heartbeats(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := a.store.Monitor(r.Context(), id); err != nil {
		fail(w, err)
		return
	}
	before, limit := page(r)
	items, err := a.store.Heartbeats(r.Context(), id, before, limit)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, items)
}
func (a *Server) logs(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := a.store.Monitor(r.Context(), id); err != nil {
		fail(w, err)
		return
	}
	before, limit := page(r)
	items, err := a.store.Logs(r.Context(), id, before, limit)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, items)
}
func (a *Server) clearHeartbeats(w http.ResponseWriter, r *http.Request) {
	a.clearHistory(w, r, "heartbeats")
}
func (a *Server) clearLogs(w http.ResponseWriter, r *http.Request) { a.clearHistory(w, r, "logs") }
func (a *Server) clearHistory(w http.ResponseWriter, r *http.Request, kind string) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	count, err := a.store.ClearHistory(r.Context(), id, kind)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]int64{"deleted": count})
}

func (a *Server) groups(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.Groups(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, items)
}
func (a *Server) saveGroup(w http.ResponseWriter, r *http.Request) {
	var g model.Group
	if !decode(w, r, &g) {
		return
	}
	g.ID = 0
	if r.Method == http.MethodPut {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		g.ID = id
	}
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" || len(g.Name) > 200 {
		writeError(w, 400, "分组名称不能为空，且不能超过 200 字节")
		return
	}
	g, err := a.store.SaveGroup(r.Context(), g)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, g)
}
func (a *Server) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := a.store.DeleteGroup(r.Context(), id); err != nil {
		fail(w, err)
		return
	}
	if err := a.engine.Reload(r.Context()); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *Server) panels(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.Panels(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, items)
}
func (a *Server) savePanel(w http.ResponseWriter, r *http.Request) {
	var p model.Panel
	if !decode(w, r, &p) {
		return
	}
	p.ID = 0
	if r.Method == http.MethodPut {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		p.ID = id
	}
	p.Name = strings.TrimSpace(p.Name)
	p.Slug = strings.TrimSpace(p.Slug)
	if p.Name == "" || len(p.Name) > 200 || !slugPattern.MatchString(p.Slug) || len(p.Description) > 4000 {
		writeError(w, 400, "请填写名称和访问路径；路径最多 64 位，仅支持小写字母、数字和连字符")
		return
	}
	if p.Groups != nil {
		p.MonitorIDs = []int64{}
		for _, group := range p.Groups {
			p.MonitorIDs = append(p.MonitorIDs, group.MonitorIDs...)
		}
	}
	if len(p.MonitorIDs) > 1000 {
		writeError(w, 400, "单个面板最多展示 1000 项监控")
		return
	}
	ids := []int64{}
	seen := map[int64]bool{}
	for _, id := range p.MonitorIDs {
		if id <= 0 {
			writeError(w, 400, "无效的监控编号")
			return
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	p.MonitorIDs = ids
	p, err := a.store.SavePanel(r.Context(), p)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, p)
}
func (a *Server) deletePanel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := a.store.DeletePanel(r.Context(), id); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *Server) settings(w http.ResponseWriter, r *http.Request) {
	value, err := a.store.Settings(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, value)
}
func (a *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	var v model.Settings
	if !decode(w, r, &v) {
		return
	}
	if err := validateSettings(v); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if err := a.store.SaveSettings(r.Context(), v); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (a *Server) cleanup(w http.ResponseWriter, r *http.Request) {
	v, err := a.store.Settings(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	counts, err := a.store.Cleanup(r.Context(), v.RetentionDays)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, counts)
}
