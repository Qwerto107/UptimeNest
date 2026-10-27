package api

import (
	"net/http"
	"slices"
	"strings"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

var monitorSecretFields = []string{
	"http.auth.password", "http.auth.clientSecret", "http.auth.clientKey",
	"http.headers", "http.body", "websocket.headers", "grpc.metadata",
}

type monitorConfigView struct {
	model.Monitor
	Configured []string `json:"configured"`
}

type monitorInput struct {
	monitorConfigView
	ClearSecrets []string `json:"clearSecrets"`
	CopyFrom     int64    `json:"copyFrom"`
}

func monitorSecret(m *model.Monitor, field string) *string {
	switch field {
	case "http.auth.password":
		if m.HTTP != nil {
			return &m.HTTP.Auth.Password
		}
	case "http.auth.clientSecret":
		if m.HTTP != nil {
			return &m.HTTP.Auth.ClientSecret
		}
	case "http.auth.clientKey":
		if m.HTTP != nil {
			return &m.HTTP.Auth.ClientKey
		}
	case "http.headers":
		if m.HTTP != nil {
			return &m.HTTP.Headers
		}
	case "http.body":
		if m.HTTP != nil {
			return &m.HTTP.Body
		}
	case "websocket.headers":
		if m.WebSocket != nil {
			return &m.WebSocket.Headers
		}
	case "grpc.metadata":
		if m.GRPC != nil {
			return &m.GRPC.Metadata
		}
	}
	return nil
}

func redactMonitor(m model.Monitor) monitorConfigView {
	// 只复制将要清空的结构，避免修改保存结果或检测任务共用的配置指针。
	if m.HTTP != nil {
		value := *m.HTTP
		m.HTTP = &value
	}
	if m.WebSocket != nil {
		value := *m.WebSocket
		m.WebSocket = &value
	}
	if m.GRPC != nil {
		value := *m.GRPC
		m.GRPC = &value
	}
	view := monitorConfigView{Monitor: m, Configured: []string{}}
	for _, field := range monitorSecretFields {
		if value := monitorSecret(&view.Monitor, field); value != nil {
			if *value != "" {
				view.Configured = append(view.Configured, field)
			}
			*value = ""
		}
	}
	return view
}

// monitorDisplay 保留列表和详情使用的公共配置，协议与告警配置按需读取。
func monitorDisplay(m model.Monitor) model.Monitor {
	m.HTTP, m.WebSocket, m.GRPC, m.Alerts = nil, nil, nil, nil
	return m
}

func (a *Server) monitorConfig(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	m, err := a.store.Monitor(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, redactMonitor(m))
}

func (a *Server) mergeMonitorSecrets(w http.ResponseWriter, r *http.Request, m *model.Monitor, input monitorInput) (*model.Monitor, bool) {
	for _, field := range input.ClearSecrets {
		if !slices.Contains(monitorSecretFields, field) {
			writeError(w, 400, "无效的凭据字段")
			return nil, false
		}
	}
	if input.CopyFrom < 0 || (m.ID > 0 && input.CopyFrom != 0) {
		writeError(w, 400, "复制来源仅可用于创建监控")
		return nil, false
	}
	sourceID := m.ID
	if sourceID == 0 {
		sourceID = input.CopyFrom
	}
	var old model.Monitor
	if sourceID > 0 {
		var err error
		old, err = a.store.Monitor(r.Context(), sourceID)
		if err != nil {
			fail(w, err)
			return nil, false
		}
	}
	for _, field := range monitorSecretFields {
		value := monitorSecret(m, field)
		if value == nil {
			continue
		}
		if slices.Contains(input.ClearSecrets, field) {
			*value = ""
			continue
		}
		if old.Type != m.Type || *value != "" {
			continue
		}
		// 切换认证方式时不继承旧方式的密码或私钥。
		if strings.HasPrefix(field, "http.auth.") && (old.HTTP == nil || m.HTTP.Auth.Type != old.HTTP.Auth.Type) {
			continue
		}
		if previous := monitorSecret(&old, field); previous != nil {
			*value = *previous
		}
	}
	if m.ID > 0 {
		return &old, true
	}
	return nil, true
}
