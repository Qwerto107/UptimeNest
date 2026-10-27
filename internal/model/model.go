// Package model 定义数据库、监控引擎和接口共用的数据结构。
package model

import "time"

const DefaultUserAgent = "UptimeNest"

// Monitor 保存检测配置；访客仅能看到未隐藏的主机地址，不返回完整目标配置。
type Monitor struct {
	ID            int64             `json:"id"`
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Target        string            `json:"target"`
	HideAddress   bool              `json:"hideAddress"`
	Port          int               `json:"port"`
	DNSRecord     string            `json:"dnsRecord"`
	DNSServer     string            `json:"dnsServer"`
	Expected      string            `json:"expected"`
	Interval      int               `json:"interval"`
	Retries       int               `json:"retries"`
	RetryInterval int               `json:"retryInterval"`
	Timeout       int               `json:"timeout"`
	Invert        bool              `json:"invert"`
	Description   string            `json:"description"`
	Tags          []string          `json:"tags"`
	GroupID       int64             `json:"groupId"`
	Enabled       bool              `json:"enabled"`
	HTTP          *HTTPOptions      `json:"http,omitempty"`
	WebSocket     *WebSocketOptions `json:"websocket,omitempty"`
	GRPC          *GRPCOptions      `json:"grpc,omitempty"`
}

// GRPCOptions 的 Metadata 和服务名称仅供后台使用，不进入公开状态页。
type GRPCOptions struct {
	Security    string `json:"security"`
	ServiceName string `json:"serviceName"`
	Metadata    string `json:"metadata"`
	CACert      string `json:"caCert"`
}

// GRPCConfig 为缺省连接选项补齐 TLS，不将省略配置解释为明文。
func (m Monitor) GRPCConfig() GRPCOptions {
	v := GRPCOptions{Security: "tls"}
	if m.GRPC != nil {
		v = *m.GRPC
	}
	if v.Security == "" {
		v.Security = "tls"
	}
	return v
}

// WebSocketOptions 的请求头和目标路径仅供后台使用，不进入公开状态页。
type WebSocketOptions struct {
	Mode         string   `json:"mode"`
	ConnectIP    string   `json:"connectIP,omitempty"`
	Origin       string   `json:"origin"`
	Subprotocols []string `json:"subprotocols"`
	Headers      string   `json:"headers"`
}

// WebSocketConfig 为未填写的检测方式补齐握手默认值，不改变已明确选择的模式。
func (m Monitor) WebSocketConfig() WebSocketOptions {
	v := WebSocketOptions{Mode: "handshake", Subprotocols: []string{}}
	if m.WebSocket != nil {
		v = *m.WebSocket
	}
	if v.Mode == "" {
		v.Mode = "handshake"
	}
	if v.Subprotocols == nil {
		v.Subprotocols = []string{}
	}
	return v
}

// HTTPOptions 保存单项 HTTP 检测配置；省略的字段由 HTTPConfig 补齐默认值。
type HTTPOptions struct {
	Version          string   `json:"version"`
	ConnectIP        string   `json:"connectIP,omitempty"`
	IgnoreTLS        bool     `json:"ignoreTLS"`
	MaxRedirects     *int     `json:"maxRedirects,omitempty"`
	ValidStatusCodes []string `json:"validStatusCodes"`
	Method           string   `json:"method"`
	BodyEncoding     string   `json:"bodyEncoding"`
	Body             string   `json:"body"`
	Headers          string   `json:"headers"`
	Auth             HTTPAuth `json:"auth"`
}

// HTTPAuth 的凭据仅属于管理员配置，不能进入公开面板或心跳数据。
type HTTPAuth struct {
	Type            string `json:"type"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	TokenURL        string `json:"tokenURL"`
	ClientID        string `json:"clientID"`
	ClientSecret    string `json:"clientSecret"`
	Scope           string `json:"scope"`
	TokenAuthMethod string `json:"tokenAuthMethod"`
	ClientCert      string `json:"clientCert"`
	ClientKey       string `json:"clientKey"`
	CACert          string `json:"caCert"`
}

// HTTPConfig 补齐旧配置未包含的选项，明确设置的零次重定向不会被默认值覆盖。
func (m Monitor) HTTPConfig() HTTPOptions {
	var v HTTPOptions
	if m.HTTP != nil {
		v = *m.HTTP
	}
	if v.Version == "" {
		v.Version = "auto"
	}
	if v.Method == "" {
		v.Method = "GET"
	}
	if v.MaxRedirects == nil {
		redirects := 10
		v.MaxRedirects = &redirects
	}
	if v.ValidStatusCodes == nil {
		v.ValidStatusCodes = []string{"200-299", "300-399"}
	}
	if v.BodyEncoding == "" {
		v.BodyEncoding = "json"
	}
	if v.Auth.Type == "" {
		v.Auth.Type = "none"
	}
	if v.Auth.TokenAuthMethod == "" {
		v.Auth.TokenAuthMethod = "header"
	}
	return v
}

type Group struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ParentID int64  `json:"parentId"`
}

// PanelGroup 仅属于一个状态面板，与后台监控分组没有关联。
type PanelGroup struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	MonitorIDs []int64 `json:"monitorIds"`
}

// Panel 保存独立展示分组；MonitorIDs 为兼容索引，实际展示顺序以 Groups 为准。
type Panel struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name"`
	PageTitle   string       `json:"pageTitle"`
	Slug        string       `json:"slug"`
	Description string       `json:"description"`
	MonitorIDs  []int64      `json:"monitorIds"`
	Groups      []PanelGroup `json:"groups"`
	IsDefault   bool         `json:"isDefault"`
}

type Settings struct {
	SiteName            string   `json:"siteName"`
	Timezone            string   `json:"timezone"`
	Language            string   `json:"language"`
	EntryPage           string   `json:"entryPage"`
	UserAgent           string   `json:"userAgent"`
	TLSLatencyWarningMS int      `json:"tlsLatencyWarningMs"`
	RetentionDays       int      `json:"retentionDays"`
	TrustedProxies      []string `json:"trustedProxies"`
}

// DefaultSettings 为首次安装及缺少配置字段时提供默认展示设置。
func DefaultSettings() Settings {
	return Settings{SiteName: "UptimeNest", Timezone: "Asia/Singapore", Language: "en-US", EntryPage: "status", RetentionDays: 30, TrustedProxies: []string{}}
}

// Heartbeat 只记录一次完整检测（含重试）后的最终结果。
type Heartbeat struct {
	ID             int64   `json:"id"`
	MonitorID      int64   `json:"monitorId"`
	Time           int64   `json:"time"`
	Status         string  `json:"status"`
	Latency        float64 `json:"latency"`
	Message        string  `json:"message"`
	TLSVersion     string  `json:"tlsVersion,omitempty"`
	HTTPVersion    string  `json:"httpVersion,omitempty"`
	TLSLatency     float64 `json:"tlsLatency,omitempty"`
	CertExpiry     int64   `json:"certExpiry,omitempty"`
	CertIssuerOrg  string  `json:"certIssuerOrg,omitempty"`
	TLSKeyExchange string  `json:"tlsKeyExchange,omitempty"`
	TLSCipherSuite string  `json:"tlsCipherSuite,omitempty"`
	QUICVersion    string  `json:"quicVersion,omitempty"`
	ZeroRTT        *bool   `json:"zeroRTT,omitempty"`
}

type Log struct {
	ID        int64  `json:"id"`
	MonitorID int64  `json:"monitorId"`
	Time      int64  `json:"time"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// Summary 的统计范围为最近 24 小时；无数据时 Uptime 为 nil。
type Summary struct {
	Status     string      `json:"status"`
	Uptime     *float64    `json:"uptime"`
	Average    *float64    `json:"average"`
	Latest     *Heartbeat  `json:"latest"`
	Heartbeats []Heartbeat `json:"heartbeats"`
}

type LatencyStatistics struct {
	Average float64 `json:"average"`
	P95     float64 `json:"p95"`
	Maximum float64 `json:"maximum"`
}

// LatencyBucket 描述一个时间段的真实记录；延迟只统计有效的成功检测。
type LatencyBucket struct {
	Start     int64    `json:"start"`
	End       int64    `json:"end"`
	Count     int      `json:"count"`
	Successes int      `json:"successes"`
	Failures  int      `json:"failures"`
	Average   *float64 `json:"average"`
	Minimum   *float64 `json:"minimum"`
	Maximum   *float64 `json:"maximum"`
}

type LatencyHistory struct {
	Start          int64              `json:"start"`
	End            int64              `json:"end"`
	AvailableStart int64              `json:"availableStart"`
	Total          int                `json:"total"`
	Failures       int                `json:"failures"`
	Statistics     *LatencyStatistics `json:"statistics"`
	Buckets        []LatencyBucket    `json:"buckets"`
}

type MonitorView struct {
	Monitor
	Summary Summary `json:"summary"`
}

// StateChange 供可选的进程内状态订阅使用，不能代替持久化通知队列。
type StateChange struct {
	MonitorID int64
	Previous  string
	Current   string
	Time      time.Time
}
