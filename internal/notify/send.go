package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

func signature(key, value string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(value))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func Send(ctx context.Context, c model.NotificationChannel, e model.NotificationEvent, settings model.Settings) error {
	if err := Validate(&c); err != nil {
		return err
	}
	subject, body, err := Preview(c, e, settings)
	if err != nil {
		return err
	}
	if c.Type == "email" {
		return sendMail(ctx, c.Config, subject, body)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	if c.Config.ProxyURL != "" {
		proxy, err := url.Parse(c.Config.ProxyURL)
		if err != nil {
			return fmt.Errorf("通知代理地址无效")
		}
		transport.Proxy = http.ProxyURL(proxy)
	}
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return sendHTTP(ctx, client, c, e, subject, body)
}

func sendHTTP(ctx context.Context, client *http.Client, c model.NotificationChannel, e model.NotificationEvent, subject, body string) error {
	endpoint := c.Config.URL
	var payload any
	switch c.Type {
	case "webhook":
		payload = json.RawMessage(body)
	case "feishu":
		color := "orange"
		if strings.HasSuffix(e.AlertType, "recovered") {
			color = "green"
		}
		if e.AlertType == "service_down" || e.AlertType == "certificate_expired" {
			color = "red"
		}
		// 卡片标题保持纯文本，正文使用飞书 Markdown，保留模板中的富文本语法。
		message := map[string]any{"msg_type": "interactive", "card": map[string]any{"config": map[string]bool{"wide_screen_mode": true}, "header": map[string]any{"template": color, "title": map[string]string{"tag": "plain_text", "content": subject}}, "elements": []any{map[string]any{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": body}}}}}
		if c.Config.Secret != "" {
			ts := strconv.FormatInt(time.Now().Unix(), 10)
			message["timestamp"] = ts
			message["sign"] = signature(ts+"\n"+c.Config.Secret, "")
		}
		payload = message
	case "dingtalk":
		if c.Config.Secret != "" {
			u, err := url.Parse(endpoint)
			if err != nil {
				return fmt.Errorf("机器人地址无效")
			}
			ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
			q := u.Query()
			q.Set("timestamp", ts)
			q.Set("sign", signature(c.Config.Secret, ts+"\n"+c.Config.Secret))
			u.RawQuery = q.Encode()
			endpoint = u.String()
		}
		payload = map[string]any{"msgtype": "markdown", "markdown": map[string]string{"title": subject, "text": "### " + subject + "\n\n" + body}}
	case "telegram":
		endpoint = "https://api.telegram.org/bot" + c.Config.Token + "/sendMessage"
		message := map[string]any{"chat_id": c.Config.ChatID, "text": subject + "\n\n" + body}
		if c.Config.ThreadID > 0 {
			message["message_thread_id"] = c.Config.ThreadID
		}
		payload = message
	default:
		return fmt.Errorf("不支持的通知渠道类型")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("通知内容无法编码")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("通知请求地址无效")
	}
	if c.Type == "webhook" {
		for name, value := range c.Config.Headers {
			req.Header.Set(name, value)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", model.DefaultUserAgent)
	resp, err := client.Do(req)
	// 不保留原始网络错误，它通常包含带令牌的 URL 或代理凭据。
	if err != nil {
		return fmt.Errorf("通知网络请求失败，请检查地址、网络和代理设置")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("通知服务返回 HTTP %d", resp.StatusCode)
	}
	if c.Type == "webhook" {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil
	}
	var result struct {
		Code       *int  `json:"code"`
		StatusCode *int  `json:"StatusCode"`
		ErrCode    *int  `json:"errcode"`
		OK         *bool `json:"ok"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&result); err != nil {
		return fmt.Errorf("机器人返回了无效响应")
	}
	switch c.Type {
	case "telegram":
		if result.OK == nil || !*result.OK {
			return fmt.Errorf("Telegram 拒绝了通知，请检查 Bot 权限、Chat ID 和消息长度")
		}
	case "dingtalk":
		if result.ErrCode == nil || *result.ErrCode != 0 {
			return fmt.Errorf("钉钉拒绝了通知，请检查签名、关键词和限流设置")
		}
	case "feishu":
		code := result.Code
		if code == nil {
			code = result.StatusCode
		}
		if code == nil || *code != 0 {
			return fmt.Errorf("飞书拒绝了通知，请检查签名、关键词和限流设置")
		}
	}
	return nil
}

func sendMail(ctx context.Context, v model.NotificationConfig, subject, body string) error {
	address := smtpAddress(v)
	tlsConfig := &tls.Config{ServerName: strings.Trim(v.Host, "[]"), MinVersion: tls.VersionTLS12}
	var conn net.Conn
	var err error
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if v.Security == "tls" {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("SMTP 连接失败，请检查主机、端口和 TLS 设置")
	}
	defer conn.Close()
	deadline := time.Now().Add(15 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	client, err := smtp.NewClient(conn, strings.Trim(v.Host, "[]"))
	if err != nil {
		return fmt.Errorf("SMTP 握手失败")
	}
	defer client.Close()
	if v.Security == "starttls" {
		if err = client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("SMTP STARTTLS 失败")
		}
	}
	if v.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", v.Username, v.Password, strings.Trim(v.Host, "[]"))); err != nil {
			return fmt.Errorf("SMTP 认证失败，请检查账户和授权码")
		}
	}
	from, _ := mail.ParseAddress(v.From)
	if err = client.Mail(from.Address); err != nil {
		return fmt.Errorf("SMTP 不接受发件人")
	}
	for _, recipient := range v.To {
		to, _ := mail.ParseAddress(recipient)
		if err = client.Rcpt(to.Address); err != nil {
			return fmt.Errorf("SMTP 不接受收件人")
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP 不接受邮件内容")
	}
	subject = strings.NewReplacer("\r", " ", "\n", " ").Replace(subject)
	message := "From: " + from.String() + "\r\nTo: " + strings.Join(v.To, ", ") + "\r\nSubject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\nDate: " + time.Now().Format(time.RFC1123Z) + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	for len(encoded) > 76 {
		message += encoded[:76] + "\r\n"
		encoded = encoded[76:]
	}
	message += encoded + "\r\n"
	if _, err = io.WriteString(writer, message); err != nil {
		return fmt.Errorf("SMTP 邮件发送失败")
	}
	if err = writer.Close(); err != nil {
		return fmt.Errorf("SMTP 未确认接收邮件")
	}
	// DATA 已被接收后即认为成功；QUIT 的网络错误不能触发重复投递。
	_ = client.Quit()
	return nil
}
