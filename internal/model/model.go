// Package model 定义数据库、监控引擎和接口共用的数据结构。
package model

import "time"

const DefaultUserAgent = "UptimeNest/0.1"

// Monitor 保存检测配置；访客仅能看到未隐藏的主机地址，不返回完整目标配置。
type Monitor struct {
	ID            int64        `json:"id"`
	Name          string       `json:"name"`
	Type          string       `json:"type"`
	Target        string       `json:"target"`
	HideAddress   bool         `json:"hideAddress"`
	Port          int          `json:"port"`
	DNSRecord     string       `json:"dnsRecord"`
	DNSServer     string       `json:"dnsServer"`
	Expected      string       `json:"expected"`
	Interval      int          `json:"interval"`
	Retries       int          `json:"retries"`
	RetryInterval int          `json:"retryInterval"`
	Timeout       int          `json:"timeout"`
	Invert        bool         `json:"invert"`
	Description   string       `json:"description"`
	Tags          []string     `json:"tags"`
	GroupID       int64        `json:"groupId"`
	Enabled       bool         `json:"enabled"`
	HTTP          *HTTPOptions `json:"http,omitempty"`
}

// HTTPOptions 保存单项 HTTP 检测配置；省略时保持旧监控的默认行为。
type HTTPOptions struct {
	Version          string   `json:"version"`
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
		v.Version = "standard"
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
	Slug        string       `json:"slug"`
	Description string       `json:"description"`
	MonitorIDs  []int64      `json:"monitorIds"`
	Groups      []PanelGroup `json:"groups"`
	IsDefault   bool         `json:"isDefault"`
}

type Settings struct {
	Timezone      string `json:"timezone"`
	Language      string `json:"language"`
	EntryPage     string `json:"entryPage"`
	UserAgent     string `json:"userAgent"`
	RetentionDays int    `json:"retentionDays"`
}

func DefaultSettings() Settings {
	return Settings{Timezone: "Asia/Shanghai", Language: "zh-CN", EntryPage: "status", RetentionDays: 90}
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

type MonitorView struct {
	Monitor
	Summary Summary `json:"summary"`
}

// StateChange 将状态事件与检测逻辑解耦，当前版本只提供订阅入口。
type StateChange struct {
	MonitorID int64
	Previous  string
	Current   string
	Time      time.Time
}
