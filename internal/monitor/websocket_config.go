package monitor

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"golang.org/x/net/http/httpguts"
)

// ValidateWebSocket 规范化专属选项，握手关键头只能由协议库管理。
func ValidateWebSocket(m *model.Monitor) error {
	u, err := url.Parse(m.Target)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Hostname() == "" || u.User != nil || u.Fragment != "" || strings.Contains(m.Target, "#") {
		return fmt.Errorf("请输入有效的 WS/WSS 地址，不允许携带账户密码或片段")
	}
	if port := u.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return fmt.Errorf("WebSocket 端口应为 1 至 65535")
		}
	}
	v := m.WebSocketConfig()
	if v.Mode != "handshake" && v.Mode != "ping" {
		return fmt.Errorf("WebSocket 检测模式无效")
	}
	v.ConnectIP, err = normalizeConnectIP(v.ConnectIP)
	if err != nil {
		return err
	}
	v.Origin = strings.TrimSpace(v.Origin)
	if v.Origin != "" {
		origin, err := url.Parse(v.Origin)
		if err != nil || len(v.Origin) > 2048 || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Hostname() == "" || origin.User != nil || (origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.ForceQuery || origin.Fragment != "" || strings.ContainsAny(v.Origin, "\r\n#") {
			return fmt.Errorf("Origin 应为不含路径、参数或凭据的 HTTP/HTTPS 来源")
		}
		v.Origin = origin.Scheme + "://" + origin.Host
	}
	if len(v.Subprotocols) > 16 {
		return fmt.Errorf("WebSocket 子协议最多 16 项，每项最多 128 字节且不能重复")
	}
	seen := map[string]bool{}
	protocols := []string{}
	for _, protocol := range v.Subprotocols {
		protocol = strings.TrimSpace(protocol)
		if len(protocol) > 128 || !httpguts.ValidHeaderFieldName(protocol) || seen[protocol] {
			return fmt.Errorf("WebSocket 子协议最多 16 项，每项最多 128 字节且不能重复")
		}
		seen[protocol] = true
		protocols = append(protocols, protocol)
	}
	v.Subprotocols = protocols
	if len(v.Headers) > 64<<10 {
		return fmt.Errorf("WebSocket 请求头最多 64 KiB")
	}
	headers, err := httpHeaders(v.Headers)
	if err != nil {
		return err
	}
	for name := range headers {
		if name == "Connection" || name == "Upgrade" || name == "Origin" || strings.HasPrefix(strings.ToLower(name), "sec-websocket-") {
			return fmt.Errorf("WebSocket 握手头由系统管理，Origin 和子协议请使用专属字段")
		}
	}
	m.WebSocket = &v
	return nil
}
