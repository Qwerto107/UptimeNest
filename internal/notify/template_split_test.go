package notify

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

func TestServiceAndCertificateTemplatesRouteIndependently(t *testing.T) {
	for kind, defaults := range ChannelTemplates() {
		t.Run(kind, func(t *testing.T) {
			c := model.NotificationChannel{Type: kind, Subject: "service {{alert_type}}", Body: "service {{alert_type}}", CertificateTemplate: &model.NotificationTemplate{Subject: "certificate {{alert_type}}", Body: "certificate {{alert_type}}"}}
			if kind == "webhook" {
				c.Body = `{"template":"service","event":"{{alert_type}}"}`
				c.CertificateTemplate.Body = `{"template":"certificate","event":"{{alert_type}}"}`
			}
			for _, eventType := range []string{"service_down", "service_recovered", "certificate_expiring", "certificate_expired", "certificate_recovered", "test"} {
				event, err := SampleEventFor(eventType)
				if err != nil {
					t.Fatal(err)
				}
				subject, body, err := Preview(c, event, model.DefaultSettings())
				marker := "service"
				if strings.HasPrefix(eventType, "certificate_") {
					marker = "certificate"
				}
				if err != nil || !strings.Contains(body, marker) || !strings.Contains(subject, marker) || !strings.Contains(body, eventType) {
					t.Fatalf("wrong template for %s: %s %s %v", eventType, subject, body, err)
				}
				if kind == "webhook" && !json.Valid([]byte(body)) {
					t.Fatal("invalid rendered JSON")
				}
			}
			if defaults.Body == defaults.CertificateTemplate.Body || !strings.Contains(defaults.CertificateTemplate.Body, "{{cert_expiry}}") {
				t.Fatal("defaults do not distinguish service and certificate content")
			}
			// 升级前的自定义内容继续用于证书事件，不能替换成新默认值。
			c.CertificateTemplate = nil
			event, _ := SampleEventFor("certificate_expiring")
			_, body, err := Preview(c, event, model.DefaultSettings())
			if err != nil || !strings.Contains(body, "service") {
				t.Fatalf("legacy template changed: %s %v", body, err)
			}
		})
	}
	if _, err := SampleEventFor("unknown"); err == nil {
		t.Fatal("unknown test event accepted")
	}
}

func TestCertificateTemplateValidation(t *testing.T) {
	c := model.NotificationChannel{Name: "hook", Type: "webhook", Body: `{}`, Config: model.NotificationConfig{URL: "https://example.com"}}
	for _, body := range []string{`{"message":"{{unknown}}"}`, `{"message":"{{monitor_name"}`, `not json`, strings.Repeat("x", 12001)} {
		c.CertificateTemplate = &model.NotificationTemplate{Body: body}
		if err := Validate(&c); err == nil {
			t.Fatalf("invalid certificate template accepted: %s", body[:min(len(body), 50)])
		}
	}
	c.CertificateTemplate = nil
	c.Body = `{"legacy":"{{alert_type}}"}`
	if err := Validate(&c); err != nil || c.CertificateTemplate.Body != c.Body {
		t.Fatalf("legacy content was not preserved: %+v %v", c.CertificateTemplate, err)
	}
}
