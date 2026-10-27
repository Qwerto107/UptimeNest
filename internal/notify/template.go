// Package notify 渲染通知模板并发送消息，不参与监控状态判定。
package notify

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

var Variables = []string{"alert_type", "site_name", "monitor_id", "monitor_name", "monitor_target", "monitor_type", "status", "error_message", "event_time", "downtime", "cert_expiry", "cert_days_remaining", "latency_ms"}
var variablePattern = regexp.MustCompile(`\{\{\s*([a-z_]+)\s*\}\}`)

const DefaultSubject = "[UptimeNest] {{alert_type}} · {{monitor_name}}"
const DefaultBody = "{{alert_type}}\nMonitor: {{monitor_name}}\nTarget: {{monitor_target}}\nStatus: {{status}}\nTime: {{event_time}}\nDetails: {{error_message}}\nDuration: {{downtime}}"
const DefaultCertificateBody = "{{alert_type}}\nMonitor: {{monitor_name}}\nTarget: {{monitor_target}}\nTime: {{event_time}}\nCertificate expiry: {{cert_expiry}}\nDays remaining: {{cert_days_remaining}}"
const DefaultWebhookBody = `{
  "type": "{{alert_type}}",
  "monitor_id": "{{monitor_id}}",
  "monitor_name": "{{monitor_name}}",
  "target": "{{monitor_target}}",
  "status": "{{status}}",
  "message": "{{error_message}}",
  "time": "{{event_time}}",
  "downtime": "{{downtime}}"
}`
const DefaultCertificateWebhookBody = `{
  "type": "{{alert_type}}",
  "monitor_id": "{{monitor_id}}",
  "monitor_name": "{{monitor_name}}",
  "target": "{{monitor_target}}",
  "time": "{{event_time}}",
  "cert_expiry": "{{cert_expiry}}",
  "cert_days_remaining": "{{cert_days_remaining}}"
}`

type ChannelTemplate struct {
	Subject             string                     `json:"subject"`
	Body                string                     `json:"body"`
	Format              string                     `json:"format"`
	CertificateTemplate model.NotificationTemplate `json:"certificateTemplate"`
}

// 渠道消息格式保持一致，服务和证书事件分别使用独立模板。
func ChannelTemplates() map[string]ChannelTemplate {
	templates := map[string]ChannelTemplate{
		"email":    {Subject: "[{{site_name}}] {{alert_type}} · {{monitor_name}}", Body: DefaultBody, Format: "email"},
		"webhook":  {Body: DefaultWebhookBody, Format: "json"},
		"feishu":   {Subject: "{{alert_type}} · {{monitor_name}}", Body: "**Monitor:** {{monitor_name}}\n**Target:** {{monitor_target}}\n**Status:** {{status}}\n\n**Details:** {{error_message}}\n**Time:** {{event_time}}\n**Duration:** {{downtime}}", Format: "card"},
		"dingtalk": {Subject: "{{site_name}} · {{alert_type}}", Body: "**Monitor:** {{monitor_name}}\n\n**Target:** {{monitor_target}}\n\n**Status:** {{status}}\n\n**Details:** {{error_message}}\n\n**Time:** {{event_time}}\n\n**Duration:** {{downtime}}", Format: "markdown"},
		"telegram": {Subject: "[{{site_name}}] {{alert_type}}", Body: "{{monitor_name}}\n{{monitor_target}}\n\nStatus: {{status}}\nDetails: {{error_message}}\nTime: {{event_time}}\nDuration: {{downtime}}", Format: "text"},
	}
	certificateBodies := map[string]string{
		"email":    DefaultCertificateBody,
		"webhook":  DefaultCertificateWebhookBody,
		"feishu":   "**Monitor:** {{monitor_name}}\n**Target:** {{monitor_target}}\n\n**Certificate expiry:** {{cert_expiry}}\n**Days remaining:** {{cert_days_remaining}}\n**Time:** {{event_time}}",
		"dingtalk": "**Monitor:** {{monitor_name}}\n\n**Target:** {{monitor_target}}\n\n**Certificate expiry:** {{cert_expiry}}\n\n**Days remaining:** {{cert_days_remaining}}\n\n**Time:** {{event_time}}",
		"telegram": "{{monitor_name}}\n{{monitor_target}}\n\nCertificate expiry: {{cert_expiry}}\nDays remaining: {{cert_days_remaining}}\nTime: {{event_time}}",
	}
	for kind, template := range templates {
		template.CertificateTemplate = model.NotificationTemplate{Subject: template.Subject, Body: certificateBodies[kind]}
		templates[kind] = template
	}
	return templates
}

func SampleEvent() model.NotificationEvent {
	now := time.Now()
	return model.NotificationEvent{AlertType: "test", MonitorName: "Example monitor", Target: "https://example.com", MonitorType: "http", Status: "down", Error: "Example connection timeout", Time: now.UnixMilli(), Started: now.Add(-5 * time.Minute).UnixMilli(), CertExpiry: now.Add(7 * 24 * time.Hour).UnixMilli(), Latency: 142}
}

// 测试和预览共用示例事件，确保所选事件与模板类别、证书日期一致。
func SampleEventFor(alertType string) (model.NotificationEvent, error) {
	event := SampleEvent()
	event.AlertType = alertType
	switch alertType {
	case "service_down", "test":
	case "service_recovered", "certificate_expiring", "certificate_expired", "certificate_recovered":
		event.Status, event.Error = "up", ""
	default:
		return event, fmt.Errorf("无效的告警类型")
	}
	if alertType == "certificate_expired" {
		event.CertExpiry = event.Time - 86400000
	}
	if alertType == "certificate_recovered" {
		event.CertExpiry = event.Time + 90*86400000
	}
	return event, nil
}

func Values(e model.NotificationEvent, settings model.Settings) map[string]string {
	location, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		location = time.UTC
	}
	format := func(t int64) string { return time.UnixMilli(t).In(location).Format("2006-01-02 15:04:05 MST") }
	v := map[string]string{"alert_type": e.AlertType, "site_name": settings.SiteName, "monitor_id": strconv.FormatInt(e.MonitorID, 10), "monitor_name": e.MonitorName, "monitor_target": e.Target, "monitor_type": e.MonitorType, "status": e.Status, "error_message": e.Error, "event_time": format(e.Time), "downtime": "-", "cert_expiry": "-", "cert_days_remaining": "-", "latency_ms": strconv.FormatFloat(e.Latency, 'f', 1, 64)}
	if e.Started > 0 {
		v["downtime"] = (time.Duration(max(int64(0), e.Time-e.Started)) * time.Millisecond).Round(time.Second).String()
	}
	if e.CertExpiry > 0 {
		v["cert_expiry"] = format(e.CertExpiry)
		v["cert_days_remaining"] = strconv.FormatInt(int64(math.Ceil(float64(e.CertExpiry-e.Time)/86400000)), 10)
	}
	return v
}

func Render(text string, values map[string]string) (string, error) {
	var invalid string
	result := variablePattern.ReplaceAllStringFunc(text, func(token string) string {
		name := variablePattern.FindStringSubmatch(token)[1]
		value, ok := values[name]
		if !ok {
			invalid = name
		}
		return value
	})
	if invalid != "" {
		return "", fmt.Errorf("未知模板变量: %s", invalid)
	}
	// 只验证原模板中的括号，变量值里的 {{...}} 不会再次解释。
	remainder := variablePattern.ReplaceAllString(text, "")
	if strings.Contains(remainder, "{{") || strings.Contains(remainder, "}}") {
		return "", fmt.Errorf("模板变量格式应为 {{variable_name}}")
	}
	return result, nil
}

// JSON 模板先解析再替换字符串值，变量内的引号和换行不会改变 JSON 结构。
func RenderJSON(text string, values map[string]string) ([]byte, error) {
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, fmt.Errorf("Webhook 模板必须是有效 JSON，变量放在字符串值中")
	}
	var walk func(any) (any, error)
	walk = func(v any) (any, error) {
		switch item := v.(type) {
		case string:
			return Render(item, values)
		case []any:
			for i := range item {
				next, err := walk(item[i])
				if err != nil {
					return nil, err
				}
				item[i] = next
			}
		case map[string]any:
			for key := range item {
				if strings.Contains(key, "{{") {
					return nil, fmt.Errorf("Webhook 变量只能用于 JSON 字符串值")
				}
				next, err := walk(item[key])
				if err != nil {
					return nil, err
				}
				item[key] = next
			}
		}
		return v, nil
	}
	next, err := walk(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(next)
}

func Preview(c model.NotificationChannel, event model.NotificationEvent, settings model.Settings) (string, string, error) {
	if _, ok := ChannelTemplates()[c.Type]; !ok {
		return "", "", fmt.Errorf("不支持的通知渠道类型")
	}
	values := Values(event, settings)
	template := model.NotificationTemplate{Subject: c.Subject, Body: c.Body}
	if strings.HasPrefix(event.AlertType, "certificate_") && c.CertificateTemplate != nil {
		template = *c.CertificateTemplate
	}
	subject, err := Render(template.Subject, values)
	if err != nil {
		return "", "", err
	}
	if c.Type == "webhook" {
		body, err := RenderJSON(template.Body, values)
		return subject, string(body), err
	}
	body, err := Render(template.Body, values)
	return subject, body, err
}
