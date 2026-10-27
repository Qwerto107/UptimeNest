package model

// AlertRule 只控制通知，不改变检测重试、超时或服务状态。
type AlertRule struct {
	Enabled          bool    `json:"enabled"`
	ChannelIDs       []int64 `json:"channelIds"`
	Recovery         bool    `json:"recovery"`
	RepeatMinutes    int     `json:"repeatMinutes"`
	Days             int     `json:"days"`
	FailureThreshold int     `json:"failureThreshold"`
}

type AlertOptions struct {
	Service     AlertRule `json:"service"`
	Certificate AlertRule `json:"certificate"`
}

type NotificationChannel struct {
	ID      int64              `json:"id"`
	Name    string             `json:"name"`
	Type    string             `json:"type"`
	Enabled bool               `json:"enabled"`
	Config  NotificationConfig `json:"config"`
	Subject string             `json:"subject"`
	Body    string             `json:"body"`
	// 旧渠道缺少此字段时继续使用原模板，编辑保存后两套模板独立维护。
	CertificateTemplate *NotificationTemplate `json:"certificateTemplate,omitempty"`
}

type NotificationTemplate struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// NotificationConfig 仅通过管理员接口读写，密码、机器人 URL 等在读取时脱敏。
type NotificationConfig struct {
	URL      string            `json:"url"`
	Secret   string            `json:"secret"`
	Headers  map[string]string `json:"headers"`
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	Security string            `json:"security"`
	Username string            `json:"username"`
	Password string            `json:"password"`
	From     string            `json:"from"`
	To       []string          `json:"to"`
	Token    string            `json:"token"`
	ChatID   string            `json:"chatId"`
	ThreadID int64             `json:"threadId"`
	ProxyURL string            `json:"proxyURL"`
}

// NotificationEvent 是入队时的事实快照，不保存渠道凭据或渲染后的敏感请求。
type NotificationEvent struct {
	AlertType   string  `json:"alertType"`
	MonitorID   int64   `json:"monitorId"`
	MonitorName string  `json:"monitorName"`
	Target      string  `json:"target"`
	MonitorType string  `json:"monitorType"`
	Status      string  `json:"status"`
	Error       string  `json:"error"`
	Time        int64   `json:"time"`
	Started     int64   `json:"started"`
	CertExpiry  int64   `json:"certExpiry"`
	Latency     float64 `json:"latency"`
	Repeat      bool    `json:"repeat"`
}

type NotificationDelivery struct {
	ID          int64             `json:"id"`
	ChannelID   int64             `json:"channelId"`
	ChannelName string            `json:"channelName"`
	MonitorID   int64             `json:"monitorId"`
	Kind        string            `json:"kind"`
	Event       NotificationEvent `json:"event"`
	Status      string            `json:"status"`
	Attempts    int               `json:"attempts"`
	NextAttempt int64             `json:"nextAttempt"`
	Error       string            `json:"error"`
}
