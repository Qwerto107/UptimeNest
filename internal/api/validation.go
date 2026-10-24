package api

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/Qwerto107/UptimeNest/internal/monitor"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
var hostPattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,252}$`)

func validHost(host string) bool { return net.ParseIP(host) != nil || hostPattern.MatchString(host) }

// validateMonitor 集中校验通用参数和各类型目标，防止非法配置进入调度器。
func validateMonitor(m *model.Monitor) error {
	m.Name = strings.TrimSpace(m.Name)
	m.Target = strings.TrimSpace(m.Target)
	if m.Name == "" || len(m.Name) > 200 {
		return fmt.Errorf("显示名称不能为空，且不能超过 200 字节")
	}
	if len(m.Description) > 4000 || len(m.Target) > 2048 {
		return fmt.Errorf("目标地址或描述过长")
	}
	if m.Interval < 5 || m.Interval > 86400 {
		return fmt.Errorf("心跳间隔应为 5 至 86400 秒")
	}
	if m.Timeout < 1 || m.Timeout > 120 {
		return fmt.Errorf("检测超时应为 1 至 120 秒")
	}
	if m.Retries < 0 || m.Retries > 10 {
		return fmt.Errorf("重试次数应为 0 至 10 次")
	}
	if m.RetryInterval < 1 || m.RetryInterval > 3600 {
		return fmt.Errorf("重试间隔应为 1 至 3600 秒")
	}
	if m.GroupID < 0 {
		return fmt.Errorf("无效的分组")
	}
	if len(m.Tags) > 20 {
		return fmt.Errorf("最多设置 20 个标签")
	}
	tags := []string{}
	seen := map[string]bool{}
	for _, tag := range m.Tags {
		tag = strings.TrimSpace(tag)
		if len(tag) > 100 {
			return fmt.Errorf("单个标签不能超过 100 字节")
		}
		if tag != "" && !seen[tag] {
			tags = append(tags, tag)
			seen[tag] = true
		}
	}
	m.Tags = tags
	switch m.Type {
	case "http":
		u, err := url.Parse(m.Target)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
			return fmt.Errorf("请输入有效的 HTTP/HTTPS 地址，不允许在地址中携带账户密码")
		}
		if err := monitor.ValidateHTTP(m); err != nil {
			return err
		}
	case "tcp":
		if !validHost(m.Target) || m.Port < 1 || m.Port > 65535 {
			return fmt.Errorf("TCP 目标应为主机名或 IP，端口应为 1 至 65535")
		}
	case "ping":
		if !validHost(m.Target) {
			return fmt.Errorf("Ping 目标应为主机名或 IP")
		}
	case "dns":
		if !hostPattern.MatchString(m.Target) {
			return fmt.Errorf("请输入有效的 DNS 域名")
		}
		switch m.DNSRecord {
		case "A", "AAAA", "CNAME", "MX", "NS", "TXT":
		default:
			return fmt.Errorf("请选择有效的 DNS 记录类型")
		}
		if len(m.Expected) > 1000 {
			return fmt.Errorf("DNS 期望值过长")
		}
		if m.DNSServer != "" {
			host, port, err := net.SplitHostPort(m.DNSServer)
			if err != nil || net.ParseIP(host) == nil {
				return fmt.Errorf("DNS 服务器应填写 IP:端口，例如 1.1.1.1:53")
			}
			p, err := net.LookupPort("udp", port)
			if err != nil || p < 1 || p > 65535 {
				return fmt.Errorf("DNS 服务器端口无效")
			}
		}
	default:
		return fmt.Errorf("请选择 HTTP、TCP、Ping 或 DNS 类型")
	}
	if m.Type != "http" {
		m.HTTP = nil
	}
	return nil
}

func validateSettings(v model.Settings) error {
	if v.Timezone == "" || v.Timezone == "Local" {
		return fmt.Errorf("请选择明确的 IANA 时区名称，例如 Asia/Singapore 或 UTC")
	}
	if _, err := time.LoadLocation(v.Timezone); err != nil {
		return fmt.Errorf("时区无效，请使用 IANA 时区名称")
	}
	if v.Language != "zh-CN" {
		return fmt.Errorf("当前版本仅支持简体中文")
	}
	if v.EntryPage != "status" && v.EntryPage != "admin" {
		return fmt.Errorf("入口页面无效")
	}
	if len(v.UserAgent) > 500 || strings.ContainsAny(v.UserAgent, "\r\n") {
		return fmt.Errorf("User-Agent 过长或包含非法换行")
	}
	if v.RetentionDays < 1 || v.RetentionDays > 3650 {
		return fmt.Errorf("历史保留期限应为 1 至 3650 天")
	}
	return nil
}
