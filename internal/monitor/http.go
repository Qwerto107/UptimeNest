package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// probeHTTP 执行配置的请求，身份验证和重定向均受本次检测的整体超时控制。
func probeHTTP(ctx context.Context, m model.Monitor, ua string, h *model.Heartbeat) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(m.Timeout)*time.Second)
	defer cancel()
	options := m.HTTPConfig()
	body, contentType, err := httpBody(options)
	if err != nil {
		return err
	}
	headers, err := httpHeaders(options.Headers)
	if err != nil {
		return err
	}
	tlsConfig, err := httpTLSConfig(options.Auth)
	if err != nil {
		return err
	}
	// 仅按单项配置跳过检测目标的证书校验；OAuth2 令牌请求仍单独校验证书。
	tlsConfig.InsecureSkipVerify = options.IgnoreTLS
	protocols := new(http.Protocols)
	protocols.SetHTTP1(options.Version != "http2")
	protocols.SetHTTP2(options.Version != "http1")
	if options.Version == "http2" {
		protocols.SetUnencryptedHTTP2(true)
		// 服务端未返回 ALPN 时也必须拒绝，避免仅 HTTP/2 模式悄然发送 HTTP/1.1 请求。
		tlsConfig.VerifyConnection = func(state tls.ConnectionState) error {
			if state.NegotiatedProtocol != "h2" {
				return fmt.Errorf("服务器未协商 HTTP/2，无法使用仅 HTTP/2 模式")
			}
			return nil
		}
	}
	var tlsStart time.Time
	trace := &httptrace.ClientTrace{
		TLSHandshakeStart: func() { resetTLS(h); tlsStart = time.Now() },
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			if !tlsStart.IsZero() {
				h.TLSLatency = float64(time.Since(tlsStart).Microseconds()) / 1000
			}
			if err == nil {
				readTLS(state, h)
			}
		},
	}
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), options.Method, m.Target, strings.NewReader(body))
	if err != nil {
		return err
	}
	if ua == "" {
		ua = model.DefaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	if body != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, values := range headers {
		req.Header[key] = values
	}
	if host := headers.Get("Host"); host != "" {
		req.Host = host
		req.Header.Del("Host")
	}
	switch options.Auth.Type {
	case "basic":
		req.SetBasicAuth(options.Auth.Username, options.Auth.Password)
	case "oauth2":
		if err := authorizeOAuth2(ctx, req, options.Auth); err != nil {
			return err
		}
	}
	tcp := &http.Transport{
		Protocols:             protocols,
		DisableKeepAlives:     true,
		TLSClientConfig:       tlsConfig,
		ResponseHeaderTimeout: time.Duration(m.Timeout) * time.Second,
	}
	defer tcp.CloseIdleConnections()
	transport := &probeTransport{
		tcp:     tcp,
		version: options.Version,
		timeout: time.Duration(m.Timeout) * time.Second,
		beat:    h,
		host:    req.URL.Hostname(),
		ip:      options.ConnectIP,
	}
	tcp.Proxy = transport.proxy
	if options.ConnectIP != "" {
		dialer := &net.Dialer{}
		// 只替换连接地址，URL、Host、TLS SNI 和证书校验继续使用原域名。
		tcp.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, transport.connectAddress(address))
		}
	}
	client := &http.Client{Transport: transport}
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if *options.MaxRedirects == 0 {
			return http.ErrUseLastResponse
		}
		if len(via) > *options.MaxRedirects {
			return fmt.Errorf("重定向超过配置的 %d 次", *options.MaxRedirects)
		}
		if next.URL.Scheme != "http" && next.URL.Scheme != "https" {
			return fmt.Errorf("仅允许 HTTP/HTTPS 重定向")
		}
		if sameHTTPOrigin(next.URL, via[0].URL) {
			if host := headers.Get("Host"); host != "" {
				next.Host = host
			}
			return nil
		}
		if options.Auth.Type == "mtls" {
			return fmt.Errorf("mTLS 不允许重定向到其他来源")
		}
		// 任意自定义头都可能携带凭据，跨来源时不继续转发。
		for key := range headers {
			next.Header.Del(key)
		}
		for _, key := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Referer"} {
			next.Header.Del(key)
		}
		next.Header.Set("User-Agent", ua)
		next.Host = ""
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	h.HTTPVersion = resp.Proto
	if resp.TLS != nil {
		readTLS(*resp.TLS, h)
	} else {
		resetTLS(h)
	}
	if _, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024)); err != nil {
		return fmt.Errorf("读取响应失败：%w", err)
	}
	h.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	if !successfulHTTPStatus(resp.StatusCode, options.ValidStatusCodes) {
		return fmt.Errorf("HTTP 状态不在有效状态码中：%d", resp.StatusCode)
	}
	return nil
}

// sameHTTPOrigin 同时比较协议与端口，防止凭据经重定向发送到其他服务或降级连接。
func sameHTTPOrigin(a, b *url.URL) bool {
	port := func(u *url.URL) string {
		if value := u.Port(); value != "" {
			return value
		}
		if u.Scheme == "https" {
			return "443"
		}
		return "80"
	}
	return a.Scheme == b.Scheme && strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}

// authorizeOAuth2 每次检测通过客户端凭据取得令牌，不记录令牌响应或客户端密钥。
func authorizeOAuth2(ctx context.Context, req *http.Request, auth model.HTTPAuth) error {
	tokenTransport := &http.Transport{Proxy: http.ProxyFromEnvironment, ForceAttemptHTTP2: true}
	defer tokenTransport.CloseIdleConnections()
	tokenClient := &http.Client{
		Transport:     tokenTransport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	tokenCtx := context.WithValue(ctx, oauth2.HTTPClient, tokenClient)
	style := oauth2.AuthStyleInHeader
	if auth.TokenAuthMethod == "body" {
		style = oauth2.AuthStyleInParams
	}
	config := clientcredentials.Config{
		ClientID:     auth.ClientID,
		ClientSecret: auth.ClientSecret,
		TokenURL:     auth.TokenURL,
		Scopes:       strings.Fields(auth.Scope),
		AuthStyle:    style,
	}
	token, err := config.Token(tokenCtx)
	if err != nil || !token.Valid() {
		return fmt.Errorf("OAuth2 获取访问令牌失败，请检查 Token URL 和客户端凭据")
	}
	token.SetAuthHeader(req)
	return nil
}
