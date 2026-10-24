package monitor

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"golang.org/x/net/http/httpguts"
)

// ValidateHTTP 校验单项 HTTP 配置，旧监控缺少选项时补齐兼容默认值。
func ValidateHTTP(m *model.Monitor) error {
	v := m.HTTPConfig()
	switch v.Version {
	case "standard", "auto", "http3":
	default:
		return fmt.Errorf("HTTP 版本选项无效")
	}
	if v.Version == "http3" && !strings.HasPrefix(m.Target, "https://") {
		return fmt.Errorf("仅 HTTP/3 模式需要 HTTPS 地址")
	}
	if *v.MaxRedirects < 0 || *v.MaxRedirects > 100 {
		return fmt.Errorf("最大重定向次数应为 0 至 100")
	}
	switch v.Method {
	case "GET", "POST", "HEAD", "PUT", "PATCH", "DELETE", "OPTIONS", "TRACE", "CONNECT":
	default:
		return fmt.Errorf("请选择有效的 HTTP 请求方法")
	}
	if len(v.ValidStatusCodes) == 0 || len(v.ValidStatusCodes) > 100 {
		return fmt.Errorf("请选择至少一种有效状态码，最多 100 项")
	}
	for i, value := range v.ValidStatusCodes {
		v.ValidStatusCodes[i] = strings.TrimSpace(value)
		if _, _, err := statusRange(v.ValidStatusCodes[i]); err != nil {
			return err
		}
	}
	if len(v.Body) > 256<<10 || len(v.Headers) > 64<<10 {
		return fmt.Errorf("请求体最多 256 KiB，请求头最多 64 KiB")
	}
	if _, _, err := httpBody(v); err != nil {
		return err
	}
	headers, err := httpHeaders(v.Headers)
	if err != nil {
		return err
	}
	if (v.Auth.Type == "basic" || v.Auth.Type == "oauth2") && headers.Get("Authorization") != "" {
		return fmt.Errorf("使用身份验证时，请勿重复设置 Authorization 请求头")
	}
	a := v.Auth
	if len(a.Username) > 1024 || len(a.Password) > 8192 || len(a.ClientID) > 1024 || len(a.ClientSecret) > 8192 || len(a.Scope) > 4096 {
		return fmt.Errorf("身份验证参数过长")
	}
	switch a.Type {
	case "none":
	case "basic":
		if a.Username == "" || strings.Contains(a.Username, ":") {
			return fmt.Errorf("Basic 用户名不能为空且不能包含冒号")
		}
	case "oauth2":
		u, err := url.Parse(a.TokenURL)
		if err != nil || len(a.TokenURL) > 2048 || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return fmt.Errorf("OAuth2 Token URL 应为有效的 HTTP/HTTPS 地址")
		}
		if a.ClientID == "" || a.ClientSecret == "" {
			return fmt.Errorf("OAuth2 Client ID 和 Secret 不能为空")
		}
		if a.TokenAuthMethod != "header" && a.TokenAuthMethod != "body" {
			return fmt.Errorf("OAuth2 客户端凭据发送方式无效")
		}
	case "mtls":
		if !strings.HasPrefix(m.Target, "https://") {
			return fmt.Errorf("mTLS 需要 HTTPS 地址")
		}
		if _, err := httpTLSConfig(a); err != nil {
			return err
		}
	default:
		return fmt.Errorf("身份验证方式无效")
	}
	m.HTTP = &v
	return nil
}

// statusRange 同时接受单个状态码和闭区间，统一限制在 HTTP 状态码范围内。
func statusRange(value string) (int, int, error) {
	parts := strings.Split(value, "-")
	if len(parts) < 1 || len(parts) > 2 {
		return 0, 0, fmt.Errorf("有效状态码格式应为 200 或 200-299")
	}
	lo, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("有效状态码格式应为 200 或 200-299")
	}
	hi := lo
	if len(parts) == 2 {
		hi, err = strconv.Atoi(parts[1])
	}
	if err != nil || lo < 100 || hi > 599 || hi < lo {
		return 0, 0, fmt.Errorf("有效状态码应在 100 至 599 之间，区间起点不能大于终点")
	}
	return lo, hi, nil
}

func successfulHTTPStatus(code int, values []string) bool {
	for _, value := range values {
		lo, hi, err := statusRange(value)
		if err == nil && code >= lo && code <= hi {
			return true
		}
	}
	return false
}

// httpBody 保留原始文本；表单按 URL 编码规范解析，JSON 只在存在内容时验证。
func httpBody(v model.HTTPOptions) (string, string, error) {
	switch v.BodyEncoding {
	case "json":
		if v.Body != "" && !json.Valid([]byte(v.Body)) {
			return "", "", fmt.Errorf("请求体不是有效 JSON")
		}
		return v.Body, "application/json", nil
	case "form":
		values, err := url.ParseQuery(v.Body)
		if err != nil {
			return "", "", fmt.Errorf("请求体不是有效的 URL 编码表单")
		}
		return values.Encode(), "application/x-www-form-urlencoded", nil
	case "xml":
		return v.Body, "application/xml", nil
	case "text":
		return v.Body, "text/plain; charset=utf-8", nil
	default:
		return "", "", fmt.Errorf("请求体编码无效")
	}
}

// httpHeaders 仅接受字符串值的 JSON 对象，Host 在构造请求时单独处理。
func httpHeaders(value string) (http.Header, error) {
	result := make(http.Header)
	if strings.TrimSpace(value) == "" {
		return result, nil
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(value), &values); err != nil || values == nil {
		return nil, fmt.Errorf("请求头应为 JSON 对象，名称和值均为字符串")
	}
	if len(values) > 64 {
		return nil, fmt.Errorf("请求头最多 64 项")
	}
	for name, value := range values {
		if !httpguts.ValidHeaderFieldName(name) || !httpguts.ValidHeaderFieldValue(value) {
			return nil, fmt.Errorf("请求头名称或内容包含无效字符")
		}
		key := http.CanonicalHeaderKey(name)
		switch key {
		case "Content-Length", "Transfer-Encoding", "Trailer":
			return nil, fmt.Errorf("%s 由系统自动管理", key)
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("请求头名称重复：%s", key)
		}
		result.Set(key, value)
	}
	return result, nil
}

// httpTLSConfig 解析 PEM 客户端证书；附加 CA 与系统信任库合并，始终校验服务端证书。
func httpTLSConfig(a model.HTTPAuth) (*tls.Config, error) {
	config := &tls.Config{MinVersion: tls.VersionTLS12}
	if a.Type != "mtls" {
		return config, nil
	}
	if len(a.ClientCert) > 64<<10 || len(a.ClientKey) > 64<<10 || len(a.CACert) > 64<<10 {
		return nil, fmt.Errorf("每项 PEM 内容最多 64 KiB")
	}
	cert, err := tls.X509KeyPair([]byte(a.ClientCert), []byte(a.ClientKey))
	if err != nil {
		return nil, fmt.Errorf("mTLS 证书或私钥无效，请检查 PEM 内容及是否匹配")
	}
	config.Certificates = []tls.Certificate{cert}
	if a.CACert != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(a.CACert)) {
			return nil, fmt.Errorf("mTLS CA 证书不是有效 PEM")
		}
		config.RootCAs = pool
	}
	return config, nil
}
