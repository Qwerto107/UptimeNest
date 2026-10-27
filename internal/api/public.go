package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/Qwerto107/UptimeNest/internal/store"
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
	writePublicJSON(w, r, map[string]any{"siteName": v.SiteName, "timezone": v.Timezone, "language": v.Language, "entryPage": v.EntryPage, "statusPageView": v.StatusPageView, "defaultPanel": defaultSlug, "panels": items, "tlsLatencyWarningMs": v.TLSLatencyWarningMS, "certExpiryWarningDays": v.CertExpiryWarningDays})
}

// publicHeartbeatPoint 保留曲线所需的轻量数据，仅为可识别的故障补充脱敏分类。
type publicHeartbeatPoint struct {
	Time       int64   `json:"time"`
	Status     string  `json:"status"`
	Latency    float64 `json:"latency"`
	Failure    string  `json:"failure,omitempty"`
	StatusCode int     `json:"statusCode,omitempty"`
}

// publicFailure 只识别明确的标准诊断并返回白名单值，不公开原始错误中的地址或凭据。
func publicFailure(beat model.Heartbeat) (string, int) {
	if beat.Status != "down" {
		return "", 0
	}
	message := beat.Message
	if value, ok := strings.CutPrefix(message, "HTTP 状态不在有效状态码中："); ok {
		code, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil && code >= 100 && code <= 599 {
			return "response", code
		}
	}
	for _, suffix := range []string{
		"context deadline exceeded", "i/o timeout", "net/http: TLS handshake timeout",
		"(Client.Timeout exceeded while awaiting headers)",
		"timeout: no recent network activity", "timeout: handshake did not complete in time",
	} {
		if strings.HasSuffix(message, suffix) {
			return "timeout", 0
		}
	}
	for _, suffix := range []string{
		"connection refused", "No connection could be made because the target machine actively refused it.",
		"由于目标计算机积极拒绝，无法连接。",
	} {
		if strings.HasSuffix(message, suffix) {
			return "refused", 0
		}
	}
	switch message {
	case "反转模式：服务可访问", "DNS 返回结果与期望值不一致", "DNS 未返回匹配记录",
		"gRPC 健康状态：NOT_SERVING", "gRPC 健康状态：UNKNOWN", "gRPC 健康状态：SERVICE_UNKNOWN":
		return "response", 0
	}
	return "", 0
}

type publicSummary struct {
	Status     string                 `json:"status"`
	Uptime     *float64               `json:"uptime"`
	Average    *float64               `json:"average"`
	Latest     *model.Heartbeat       `json:"latest"`
	Heartbeats []publicHeartbeatPoint `json:"heartbeats"`
}

type publicMonitor struct {
	ID                    int64         `json:"id"`
	Name                  string        `json:"name"`
	Type                  string        `json:"type"`
	Address               string        `json:"address,omitempty"`
	Protocol              string        `json:"protocol"`
	Description           string        `json:"description"`
	Tags                  []string      `json:"tags"`
	CertExpiryWarningDays *int          `json:"certExpiryWarningDays,omitempty"`
	Summary               publicSummary `json:"summary"`
}

// publicPanel 使用白名单结构，仅公开允许展示的主机地址，不返回完整目标及内部错误消息。
func (a *Server) publicPanel(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !slugPattern.MatchString(slug) {
		writeError(w, 404, "状态面板不存在")
		return
	}
	count := heartbeatCount(r, 90)
	key := panelCacheKey{slug: slug, count: publicHeartbeatBucket(count), version: a.store.PublicVersion()}
	panel, err := a.publicCache.get(r.Context(), key, func(ctx context.Context) (*cachedPanel, error) {
		value, err := a.publicPanelData(ctx, slug, key.count)
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(value)
		return &cachedPanel{body: append(data, '\n'), value: value}, err
	})
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, 404, "状态面板不存在")
		return
	}
	if errors.Is(err, errPanelBusy) {
		writeError(w, 503, "服务繁忙，请稍后重试")
		return
	}
	if err != nil {
		if r.Context().Err() == nil {
			fail(w, err)
		}
		return
	}
	body, err := panel.response(count, key.count)
	if err != nil {
		fail(w, err)
		return
	}
	writePublicResponse(w, r, body)
}

type publicPanelInfo struct {
	Name        string `json:"name"`
	PageTitle   string `json:"pageTitle"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type publicPanelResponse struct {
	Panel    publicPanelInfo    `json:"panel"`
	Monitors []publicMonitor    `json:"monitors"`
	Groups   []model.PanelGroup `json:"groups"`
	Timezone string             `json:"timezone"`
}

type cachedPanel struct {
	body  []byte
	value *publicPanelResponse
}

func (p *cachedPanel) response(count, bucket int) ([]byte, error) {
	if count == bucket {
		return p.body, nil
	}
	// 复制切片头后裁剪，不修改共享缓存；任意条数只增加序列化成本。
	value := *p.value
	value.Monitors = append([]publicMonitor{}, p.value.Monitors...)
	for i := range value.Monitors {
		points := value.Monitors[i].Summary.Heartbeats
		if len(points) > count {
			value.Monitors[i].Summary.Heartbeats = points[len(points)-count:]
		}
	}
	body, err := json.Marshal(value)
	return append(body, '\n'), err
}

func (a *Server) publicPanelData(ctx context.Context, slug string, count int) (*publicPanelResponse, error) {
	panel, err := a.store.PanelBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	items, err := a.store.MonitorsByIDs(ctx, panel.MonitorIDs)
	if err != nil {
		return nil, err
	}
	index := map[int64]model.Monitor{}
	for _, m := range items {
		index[m.ID] = m
	}
	views := []publicMonitor{}
	summaries, err := a.store.Summaries(ctx, items, count)
	if err != nil {
		return nil, err
	}
	for _, id := range panel.MonitorIDs {
		m, exists := index[id]
		if !exists {
			continue
		}
		summary := summaries[id]
		points := make([]publicHeartbeatPoint, 0, len(summary.Heartbeats))
		for _, beat := range summary.Heartbeats {
			failure, code := publicFailure(beat)
			points = append(points, publicHeartbeatPoint{Time: beat.Time, Status: beat.Status, Latency: beat.Latency, Failure: failure, StatusCode: code})
		}
		if summary.Latest != nil {
			summary.Latest.Message = ""
			summary.Latest.MonitorID = 0
		}
		address, protocol := publicEndpoint(m)
		views = append(views, publicMonitor{
			ID: m.ID, Name: m.Name, Type: m.Type, Address: address, Protocol: protocol, Description: m.Description, Tags: m.Tags,
			CertExpiryWarningDays: m.CertExpiryWarningDays,
			Summary:               publicSummary{Status: summary.Status, Uptime: summary.Uptime, Average: summary.Average, Latest: summary.Latest, Heartbeats: points},
		})
	}
	settings, err := a.store.Settings(ctx)
	if err != nil {
		return nil, err
	}
	return &publicPanelResponse{Panel: publicPanelInfo{Name: panel.Name, PageTitle: panel.PageTitle, Slug: panel.Slug, Description: panel.Description}, Monitors: views, Groups: panel.Groups, Timezone: settings.Timezone}, nil
}

// publicEndpoint 只公开主机和端口，隐藏项不返回地址；URL 路径、查询参数等始终保留在后台。
func publicEndpoint(m model.Monitor) (address, protocol string) {
	protocol = strings.ToUpper(m.Type)
	switch m.Type {
	case "http", "websocket":
		u, err := url.Parse(m.Target)
		if err == nil && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "ws" || u.Scheme == "wss") {
			address = u.Host
			protocol = strings.ToUpper(u.Scheme)
		}
	case "tcp", "grpc":
		address = net.JoinHostPort(m.Target, strconv.Itoa(m.Port))
		if m.Type == "grpc" {
			protocol = "gRPC"
		}
	default:
		address = m.Target
	}
	if m.HideAddress {
		address = ""
	}
	return address, protocol
}
