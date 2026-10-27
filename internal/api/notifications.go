package api

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/Qwerto107/UptimeNest/internal/notify"
)

type channelView struct {
	model.NotificationChannel
	Configured []string `json:"configured"`
}

type channelInput struct {
	channelView
	ClearSecrets []string `json:"clearSecrets"`
	AlertType    string   `json:"alertType"`
}

func redactChannel(c model.NotificationChannel) channelView {
	v := channelView{NotificationChannel: c, Configured: []string{}}
	if v.CertificateTemplate == nil {
		v.CertificateTemplate = &model.NotificationTemplate{Subject: c.Subject, Body: c.Body}
	}
	for _, field := range notify.SecretFields {
		if notify.SecretPresent(c.Config, field) {
			v.Configured = append(v.Configured, field)
		}
		notify.CopySecret(&v.Config, model.NotificationConfig{}, field)
	}
	if v.Config.To == nil {
		v.Config.To = []string{}
	}
	v.Config.Headers = map[string]string{}
	return v
}

func (a *Server) notificationChannels(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.NotificationChannels(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	views := []channelView{}
	for _, c := range items {
		views = append(views, redactChannel(c))
	}
	writeJSON(w, 200, views)
}

func (a *Server) saveNotificationChannel(w http.ResponseWriter, r *http.Request) {
	var body channelInput
	if !decode(w, r, &body) {
		return
	}
	c := body.NotificationChannel
	c.ID = 0
	if r.Method == http.MethodPut {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		c.ID = id
	}
	if !a.prepareNotificationChannel(w, r, &c, body.ClearSecrets) {
		return
	}
	c, err := a.store.SaveNotificationChannel(r.Context(), c)
	if err != nil {
		fail(w, err)
		return
	}
	status := 200
	if r.Method == http.MethodPost {
		status = 201
	}
	writeJSON(w, status, redactChannel(c))
}

// 保存与草稿测试共用凭据合并和校验，测试不会将未保存的配置写回渠道。
func (a *Server) prepareNotificationChannel(w http.ResponseWriter, r *http.Request, c *model.NotificationChannel, clear []string) bool {
	if c.ID < 0 {
		writeError(w, 400, "无效的记录编号")
		return false
	}
	for _, field := range clear {
		if !slices.Contains(notify.SecretFields, field) {
			writeError(w, 400, "无效的凭据字段")
			return false
		}
	}
	if c.ID > 0 {
		old, err := a.store.NotificationChannel(r.Context(), c.ID)
		if err != nil {
			fail(w, err)
			return false
		}
		if old.Type != c.Type {
			writeError(w, 400, "已保存渠道不能修改类型，请创建新渠道")
			return false
		}
		// 旧客户端未提交新字段时保留已拆分的证书模板。
		if c.CertificateTemplate == nil {
			c.CertificateTemplate = old.CertificateTemplate
		}
		for _, field := range notify.SecretFields {
			if !notify.SecretPresent(c.Config, field) && !slices.Contains(clear, field) {
				notify.CopySecret(&c.Config, old.Config, field)
			}
		}
	}
	if err := notify.Validate(c); err != nil {
		writeError(w, 400, err.Error())
		return false
	}
	return true
}

func (a *Server) testNotificationDraft(w http.ResponseWriter, r *http.Request) {
	var body channelInput
	if !decode(w, r, &body) {
		return
	}
	if body.AlertType == "" {
		body.AlertType = "test"
	}
	event, err := notify.SampleEventFor(body.AlertType)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	c := body.NotificationChannel
	if !a.prepareNotificationChannel(w, r, &c, body.ClearSecrets) {
		return
	}
	settings, err := a.store.Settings(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if err = notify.Send(ctx, c, event, settings); err != nil {
		writeError(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"sent": true})
}

func (a *Server) deleteNotificationChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := a.store.DeleteNotificationChannel(r.Context(), id); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *Server) testNotificationChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := a.store.NotificationChannel(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	if !c.Enabled {
		writeError(w, 400, "请先启用通知渠道")
		return
	}
	if err = a.store.QueueNotificationTest(r.Context(), id, notify.SampleEvent()); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 202, map[string]bool{"queued": true})
}

func (a *Server) notificationPreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type      string `json:"type"`
		Subject   string `json:"subject"`
		Body      string `json:"body"`
		AlertType string `json:"alertType"`
	}
	if !decode(w, r, &body) {
		return
	}
	if len(body.Subject) > 1000 || len(body.Body) > 12000 {
		writeError(w, 400, "通知标题或正文模板过长")
		return
	}
	event, err := notify.SampleEventFor(body.AlertType)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	settings, err := a.store.Settings(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	subject, text, err := notify.Preview(model.NotificationChannel{Type: body.Type, Subject: body.Subject, Body: body.Body}, event, settings)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"subject": subject, "body": text})
}

func (a *Server) notificationTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"variables": notify.Variables, "channels": notify.ChannelTemplates(), "subject": notify.DefaultSubject, "body": notify.DefaultBody, "webhookBody": notify.DefaultWebhookBody})
}

func (a *Server) notificationRecords(w http.ResponseWriter, r *http.Request) {
	before := int64(0)
	if value := r.URL.Query().Get("before"); value != "" {
		var err error
		before, err = strconv.ParseInt(value, 10, 64)
		if err != nil || before < 1 {
			writeError(w, 400, "分页位置无效，请刷新后重试")
			return
		}
	}
	items, err := a.store.NotificationRecords(r.Context(), before)
	if err != nil {
		fail(w, err)
		return
	}
	channels, err := a.store.NotificationChannels(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	names := map[int64]string{}
	for _, c := range channels {
		names[c.ID] = c.Name
	}
	for i := range items {
		items[i].ChannelName = names[items[i].ChannelID]
	}
	writeJSON(w, 200, items)
}
