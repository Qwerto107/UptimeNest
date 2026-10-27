package notify

import (
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"strings"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

// SecretFields 用于脱敏与编辑时保留原凭据，URL 和自定义头也可能包含令牌。
var SecretFields = []string{"url", "secret", "password", "token", "headers", "proxyURL"}

func SecretPresent(c model.NotificationConfig, key string) bool {
	switch key {
	case "url":
		return c.URL != ""
	case "secret":
		return c.Secret != ""
	case "password":
		return c.Password != ""
	case "token":
		return c.Token != ""
	case "headers":
		return len(c.Headers) > 0
	case "proxyURL":
		return c.ProxyURL != ""
	}
	return false
}

func CopySecret(dst *model.NotificationConfig, src model.NotificationConfig, key string) {
	switch key {
	case "url":
		dst.URL = src.URL
	case "secret":
		dst.Secret = src.Secret
	case "password":
		dst.Password = src.Password
	case "token":
		dst.Token = src.Token
	case "headers":
		dst.Headers = src.Headers
	case "proxyURL":
		dst.ProxyURL = src.ProxyURL
	}
}

func Validate(c *model.NotificationChannel) error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" || len(c.Name) > 200 {
		return fmt.Errorf("渠道名称不能为空，且不能超过 200 字节")
	}
	defaults, ok := ChannelTemplates()[c.Type]
	if !ok {
		return fmt.Errorf("不支持的通知渠道类型")
	}
	legacyTemplate := c.Subject != "" || c.Body != ""
	if c.Subject == "" {
		c.Subject = defaults.Subject
	}
	if c.Body == "" {
		c.Body = defaults.Body
	}
	if c.CertificateTemplate == nil {
		template := defaults.CertificateTemplate
		if legacyTemplate {
			template = model.NotificationTemplate{Subject: c.Subject, Body: c.Body}
		}
		c.CertificateTemplate = &template
	}
	certificate := *c.CertificateTemplate
	if certificate.Subject == "" {
		certificate.Subject = defaults.CertificateTemplate.Subject
	}
	if certificate.Body == "" {
		certificate.Body = defaults.CertificateTemplate.Body
	}
	c.CertificateTemplate = &certificate
	if len(c.Subject) > 1000 || len(c.Body) > 12000 || len(c.CertificateTemplate.Subject) > 1000 || len(c.CertificateTemplate.Body) > 12000 {
		return fmt.Errorf("通知标题或正文模板过长")
	}
	v := &c.Config
	switch c.Type {
	case "email":
		if v.Host == "" || strings.ContainsAny(v.Host, "/\r\n\t ") || v.Port < 1 || v.Port > 65535 {
			return fmt.Errorf("SMTP 主机或端口无效")
		}
		if v.Security != "tls" && v.Security != "starttls" && v.Security != "none" {
			return fmt.Errorf("请选择有效的 SMTP 安全方式")
		}
		if _, err := mail.ParseAddress(v.From); err != nil || strings.ContainsAny(v.From, "\r\n") {
			return fmt.Errorf("发件人邮箱无效")
		}
		if len(v.To) == 0 || len(v.To) > 50 {
			return fmt.Errorf("请填写 1 至 50 个收件人")
		}
		for _, to := range v.To {
			if _, err := mail.ParseAddress(to); err != nil || strings.ContainsAny(to, "\r\n") {
				return fmt.Errorf("收件人邮箱无效")
			}
		}
	case "webhook", "feishu", "dingtalk":
		u, err := url.Parse(v.URL)
		if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || len(v.URL) > 4096 {
			return fmt.Errorf("请输入有效的通知 Webhook 地址")
		}
		if c.Type != "webhook" && u.Scheme != "https" {
			return fmt.Errorf("机器人 Webhook 必须使用 HTTPS")
		}
	case "telegram":
		if len(v.Token) < 10 || len(v.Token) > 256 || strings.ContainsAny(v.Token, "/?#\r\n \t") || v.ChatID == "" || len(v.ChatID) > 200 || v.ThreadID < 0 {
			return fmt.Errorf("Telegram Token、Chat ID 或话题编号无效")
		}
	default:
		return fmt.Errorf("不支持的通知渠道类型")
	}
	if v.ProxyURL != "" {
		u, err := url.Parse(v.ProxyURL)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("通知代理应为 HTTP/HTTPS 代理地址")
		}
	}
	if len(v.Headers) > 30 {
		return fmt.Errorf("自定义通知请求头过多")
	}
	for k, value := range v.Headers {
		if !validHeaderName(k) || strings.ContainsAny(value, "\r\n") || len(value) > 4000 {
			return fmt.Errorf("自定义通知请求头无效")
		}
		switch strings.ToLower(k) {
		case "host", "content-length", "connection", "transfer-encoding":
			return fmt.Errorf("不允许覆盖通知传输请求头")
		}
	}
	if len(v.Password) > 4096 || len(v.Secret) > 4096 || len(v.ProxyURL) > 2048 || len(v.Username) > 320 {
		return fmt.Errorf("通知渠道配置过长")
	}
	_, _, err := Preview(*c, SampleEvent(), model.DefaultSettings())
	if err != nil {
		return err
	}
	event, _ := SampleEventFor("certificate_expiring")
	_, _, err = Preview(*c, event, model.DefaultSettings())
	return err
}

func validHeaderName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", c)) {
			return false
		}
	}
	return true
}

func ValidateAlerts(m *model.Monitor) error {
	if m.Alerts == nil {
		return nil
	}
	if m.Alerts.Service.FailureThreshold == 0 {
		m.Alerts.Service.FailureThreshold = 1
	}
	if m.Alerts.Service.FailureThreshold < 1 || m.Alerts.Service.FailureThreshold > 1000 {
		return fmt.Errorf("连续失败告警次数应为 1 至 1000 次")
	}
	for _, rule := range []*model.AlertRule{&m.Alerts.Service, &m.Alerts.Certificate} {
		if len(rule.ChannelIDs) > 50 || rule.RepeatMinutes < 0 || rule.RepeatMinutes > 525600 {
			return fmt.Errorf("告警渠道最多 50 个，重复间隔应为 0 至 525600 分钟")
		}
		seen := map[int64]bool{}
		ids := []int64{}
		for _, id := range rule.ChannelIDs {
			if id <= 0 {
				return fmt.Errorf("无效的通知渠道编号")
			}
			if !seen[id] {
				ids = append(ids, id)
				seen[id] = true
			}
		}
		rule.ChannelIDs = ids
		if rule.Enabled && len(ids) == 0 {
			return fmt.Errorf("开启告警后至少选择一个通知渠道")
		}
	}
	if m.Alerts.Certificate.Enabled {
		if m.Alerts.Certificate.Days < 1 || m.Alerts.Certificate.Days > 3650 {
			return fmt.Errorf("证书提前提醒天数应为 1 至 3650 天")
		}
		u, err := url.Parse(m.Target)
		secureURL := err == nil && ((m.Type == "http" && u.Scheme == "https") || (m.Type == "websocket" && u.Scheme == "wss"))
		secureGRPC := m.Type == "grpc" && m.GRPCConfig().Security == "tls"
		if !secureURL && !secureGRPC {
			return fmt.Errorf("证书告警仅支持 HTTPS/WSS 或启用 TLS 的 gRPC 监控")
		}
	}
	return nil
}

// smtpAddress 在连接前使用标准主机/端口格式，IPv6 主机同样适用。
func smtpAddress(v model.NotificationConfig) string {
	return net.JoinHostPort(strings.Trim(v.Host, "[]"), fmt.Sprint(v.Port))
}
