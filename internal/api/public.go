package api

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

// bootstrap 只返回访客路由与展示所需配置，不暴露 UA、账户或数据库信息。
func (a *Server) bootstrap(w http.ResponseWriter, r *http.Request) {
	v, err := a.store.Settings(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	panels, err := a.store.Panels(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	type item struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	items := []item{}
	defaultSlug := ""
	for _, p := range panels {
		items = append(items, item{Name: p.Name, Slug: p.Slug})
		if p.IsDefault {
			defaultSlug = p.Slug
		}
	}
	writeJSON(w, 200, map[string]any{"timezone": v.Timezone, "language": v.Language, "entryPage": v.EntryPage, "defaultPanel": defaultSlug, "panels": items})
}

type publicMonitor struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	Address     string        `json:"address,omitempty"`
	Protocol    string        `json:"protocol"`
	Description string        `json:"description"`
	Tags        []string      `json:"tags"`
	Summary     model.Summary `json:"summary"`
}

// publicPanel 使用白名单结构，仅公开允许展示的主机地址，不返回完整目标及内部错误消息。
func (a *Server) publicPanel(w http.ResponseWriter, r *http.Request) {
	panels, err := a.store.Panels(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	var panel *model.Panel
	for i := range panels {
		if panels[i].Slug == r.PathValue("slug") {
			panel = &panels[i]
			break
		}
	}
	if panel == nil {
		writeError(w, 404, "状态面板不存在")
		return
	}
	items, err := a.store.Monitors(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	index := map[int64]model.Monitor{}
	for _, m := range items {
		index[m.ID] = m
	}
	views := []publicMonitor{}
	count := heartbeatCount(r, 90)
	for _, id := range panel.MonitorIDs {
		m, exists := index[id]
		if !exists {
			continue
		}
		summary, err := a.store.Summary(r.Context(), m, count)
		if err != nil {
			fail(w, err)
			return
		}
		for i := range summary.Heartbeats {
			summary.Heartbeats[i].Message = ""
			summary.Heartbeats[i].MonitorID = 0
		}
		if summary.Latest != nil {
			summary.Latest.Message = ""
			summary.Latest.MonitorID = 0
		}
		address, protocol := publicEndpoint(m)
		views = append(views, publicMonitor{ID: m.ID, Name: m.Name, Type: m.Type, Address: address, Protocol: protocol, Description: m.Description, Tags: m.Tags, Summary: summary})
	}
	settings, err := a.store.Settings(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"panel": map[string]any{"name": panel.Name, "slug": panel.Slug, "description": panel.Description}, "monitors": views, "groups": panel.Groups, "timezone": settings.Timezone})
}

// publicEndpoint 只公开主机和端口，隐藏项不返回地址；URL 路径、查询参数等始终保留在后台。
func publicEndpoint(m model.Monitor) (address, protocol string) {
	protocol = strings.ToUpper(m.Type)
	switch m.Type {
	case "http":
		u, err := url.Parse(m.Target)
		if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			address = u.Host
			protocol = strings.ToUpper(u.Scheme)
		}
	case "tcp":
		address = net.JoinHostPort(m.Target, strconv.Itoa(m.Port))
	default:
		address = m.Target
	}
	if m.HideAddress {
		address = ""
	}
	return address, protocol
}
