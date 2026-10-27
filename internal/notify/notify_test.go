package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTemplatesEscapeJSONAndRejectUnknownVariables(t *testing.T) {
	values := Values(SampleEvent(), model.DefaultSettings())
	values["error_message"] = "quotes \"\n中文 {{not_evaluated}}"
	data, err := RenderJSON(`{"message":"{{error_message}}","nested":["{{monitor_name}}",5]}`, values)
	if err != nil {
		t.Fatal(err)
	}
	var output map[string]any
	if err = json.Unmarshal(data, &output); err != nil || output["message"] != values["error_message"] {
		t.Fatalf("JSON injection: %s %v", data, err)
	}
	for _, template := range []string{"{{password}}", "{{ monitor.name }}", "{{monitor_name"} {
		if _, err = Render(template, values); err == nil {
			t.Fatalf("accepted invalid variable: %s", template)
		}
	}
	if _, err = RenderJSON(`{"{{monitor_name}}":"x"}`, values); err == nil {
		t.Fatal("accepted templated JSON key")
	}
}

func TestChannelPayloadsAndSignatures(t *testing.T) {
	for _, kind := range []string{"webhook", "feishu", "dingtalk", "telegram"} {
		t.Run(kind, func(t *testing.T) {
			c := model.NotificationChannel{Type: kind, Config: model.NotificationConfig{URL: "https://hooks.example/send", Secret: "test-secret", Token: "12345:test-token", ChatID: "-100123", ThreadID: 42, Headers: map[string]string{"Authorization": "Bearer test"}}}
			body := `{"text":"safe"}`
			if kind == "feishu" {
				body = "**Monitor:** example\n*Status:* down\n[Open target](https://example.com)"
			}
			called := false
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				called = true
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				response := `{"code":0}`
				switch kind {
				case "webhook":
					if r.Header.Get("Authorization") != "Bearer test" || payload["text"] != "safe" {
						t.Fatal("custom webhook lost body/headers")
					}
					response = ""
				case "feishu":
					ts := payload["timestamp"].(string)
					h := hmac.New(sha256.New, []byte(ts+"\n"+c.Config.Secret))
					if payload["sign"] != base64.StdEncoding.EncodeToString(h.Sum(nil)) || payload["msg_type"] != "interactive" {
						t.Fatal("bad Feishu signature/card")
					}
					card := payload["card"].(map[string]any)
					title := card["header"].(map[string]any)["title"].(map[string]any)
					text := card["elements"].([]any)[0].(map[string]any)["text"].(map[string]any)
					if title["tag"] != "plain_text" || title["content"] != "Title" || text["tag"] != "lark_md" || text["content"] != body {
						t.Fatal("Feishu lost its plain title or Markdown body")
					}
				case "dingtalk":
					q := r.URL.Query()
					h := hmac.New(sha256.New, []byte(c.Config.Secret))
					h.Write([]byte(q.Get("timestamp") + "\n" + c.Config.Secret))
					if q.Get("sign") != base64.StdEncoding.EncodeToString(h.Sum(nil)) || payload["msgtype"] != "markdown" {
						t.Fatal("bad DingTalk signature/payload")
					}
					response = `{"errcode":0}`
				case "telegram":
					if payload["message_thread_id"] != float64(42) || payload["chat_id"] != "-100123" || r.URL.Host != "api.telegram.org" {
						t.Fatal("bad Telegram routing")
					}
					response = `{"ok":true}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: http.Header{}}, nil
			})}
			if err := sendHTTP(context.Background(), client, c, SampleEvent(), "Title", body); err != nil || !called {
				t.Fatalf("send: %v called=%v", err, called)
			}
		})
	}
}

func TestHTTPDoesNotFollowRedirectOrLeakCredentials(t *testing.T) {
	forwarded := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true }))
	defer other.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, 307) }))
	defer origin.Close()
	c := model.NotificationChannel{Name: "webhook", Type: "webhook", Subject: "title", Body: `{}`, Config: model.NotificationConfig{URL: origin.URL + "/?token=private-token"}}
	err := Send(context.Background(), c, SampleEvent(), model.DefaultSettings())
	if err == nil || forwarded || strings.Contains(err.Error(), "private-token") {
		t.Fatalf("redirect/leak: %v %v", err, forwarded)
	}
	c.Config.URL = "http://127.0.0.1:1/?token=private-token"
	if err = Send(context.Background(), c, SampleEvent(), model.DefaultSettings()); err == nil || strings.Contains(err.Error(), "private-token") {
		t.Fatalf("network error leak: %v", err)
	}
}

func TestRobotBusinessErrorsFailDelivery(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":19021,"errcode":310000,"ok":false}`))}, nil
	})}
	for _, kind := range []string{"feishu", "dingtalk", "telegram"} {
		if err := sendHTTP(context.Background(), client, model.NotificationChannel{Type: kind, Config: model.NotificationConfig{URL: "https://example.com", Token: "test-token"}}, SampleEvent(), "title", "body"); err == nil {
			t.Fatalf("%s HTTP 200 treated as success", kind)
		}
	}
}

func TestChannelSpecificTemplatesAndCustomContent(t *testing.T) {
	formats := map[string]string{"email": "email", "webhook": "json", "feishu": "card", "dingtalk": "markdown", "telegram": "text"}
	defaults := ChannelTemplates()
	if len(defaults) != len(formats) {
		t.Fatalf("unexpected template types: %+v", defaults)
	}
	bodies := map[string]bool{}
	for kind, format := range formats {
		c := model.NotificationChannel{Name: "test", Type: kind, Config: model.NotificationConfig{URL: "https://example.com/hook", Host: "smtp.example.com", Port: 587, Security: "starttls", From: "monitor@example.com", To: []string{"ops@example.com"}, Token: "1234:sample-token", ChatID: "-1000"}}
		if err := Validate(&c); err != nil {
			t.Fatalf("%s defaults invalid: %v", kind, err)
		}
		if defaults[kind].Format != format || c.Subject != defaults[kind].Subject || c.Body != defaults[kind].Body {
			t.Fatalf("%s has incorrect defaults: %+v", kind, c)
		}
		bodies[c.Body] = true
		event := SampleEvent()
		event.AlertType = "certificate_recovered"
		subject, body, err := Preview(c, event, model.DefaultSettings())
		if err != nil || !strings.Contains(subject+body, event.AlertType) {
			t.Fatalf("%s does not share event variable: %s %s %v", kind, subject, body, err)
		}
		if kind == "webhook" && !json.Valid([]byte(body)) {
			t.Fatal("Webhook default did not render valid JSON")
		}
		if (kind == "dingtalk" || kind == "feishu") && !strings.Contains(body, "**Monitor:**") {
			t.Fatalf("%s default lost Markdown format", kind)
		}
		c.Subject = "Custom subject {{monitor_name}}"
		c.Body = "Custom body {{alert_type}}"
		if kind == "webhook" {
			c.Body = `{"custom":"{{alert_type}}"}`
		}
		originalSubject, originalBody := c.Subject, c.Body
		if err = Validate(&c); err != nil || c.Subject != originalSubject || c.Body != originalBody {
			t.Fatalf("%s overwrote customized template: %v", kind, err)
		}
	}
	if len(bodies) != 5 {
		t.Fatal("channel defaults share the same body")
	}
}
