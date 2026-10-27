// Package monitor 实现检测与任务调度，不依赖通知渠道。
package monitor

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

// Probe 执行单次检测；反转只改变可用性判断，不修改测量信息。
func Probe(ctx context.Context, m model.Monitor, ua string) model.Heartbeat {
	h := model.Heartbeat{MonitorID: m.ID, Status: "down"}
	start := time.Now()
	var err error
	switch m.Type {
	case "http":
		err = probeHTTP(ctx, m, ua, &h)
	case "websocket":
		err = probeWebSocket(ctx, m, ua, &h)
	case "grpc":
		err = probeGRPC(ctx, m, ua, &h)
	case "tcp":
		var conn net.Conn
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(m.Target, strconv.Itoa(m.Port)))
		if err == nil {
			conn.Close()
			h.Message = "TCP 端口连接成功"
		}
	case "ping":
		err = probePing(ctx, m, &h)
	case "dns":
		err = probeDNS(ctx, m, &h)
	default:
		err = fmt.Errorf("不支持的监控类型")
	}
	h.Latency = float64(time.Since(start).Microseconds()) / 1000
	h.Time = time.Now().UnixMilli()
	if err == nil {
		h.Status = "up"
	} else {
		h.Message = err.Error()
	}
	if m.Invert {
		if h.Status == "up" {
			h.Status = "down"
			h.Message = "反转模式：服务可访问"
		} else {
			h.Status = "up"
			h.Message = "反转模式：服务不可访问；" + h.Message
		}
	}
	return h
}

// readTLS 读取实际协商参数，颁发者名称使用站点证书 Issuer 中的组织（O）字段。
func readTLS(state tls.ConnectionState, h *model.Heartbeat) {
	h.TLSVersion = tls.VersionName(state.Version)
	h.TLSCipherSuite = tls.CipherSuiteName(state.CipherSuite)
	h.TLSKeyExchange = state.CurveID.String()
	switch state.CurveID {
	case tls.CurveP256, tls.CurveP384, tls.CurveP521, tls.X25519:
		h.TLSKeyExchange = fmt.Sprintf("ECDHE (%s)", state.CurveID)
	case 0:
		h.TLSKeyExchange = ""
		if strings.HasPrefix(h.TLSCipherSuite, "TLS_RSA_WITH_") {
			h.TLSKeyExchange = "RSA"
		}
	}
	h.CertIssuerOrg = ""
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		h.CertExpiry = cert.NotAfter.UnixMilli()
		// O 可以有多个值；缺失时不拿通用名或根证书名称代替。
		h.CertIssuerOrg = strings.Join(cert.Issuer.Organization, " / ")
	}
}

// 校验失败仍可读取对端证书的到期时间；检测继续失败，绝不绕过证书校验。
func readFailedCertificate(err error, h *model.Heartbeat) {
	var verification *tls.CertificateVerificationError
	if errors.As(err, &verification) && len(verification.UnverifiedCertificates) > 0 {
		cert := verification.UnverifiedCertificates[0]
		h.CertExpiry = cert.NotAfter.UnixMilli()
		h.CertIssuerOrg = strings.Join(cert.Issuer.Organization, " / ")
	}
}

// resetTLS 清除上一跳的连接信息，避免重定向或握手失败时混用不同连接的参数。
func resetTLS(h *model.Heartbeat) {
	h.QUICVersion = ""
	h.ZeroRTT = nil
	h.TLSVersion = ""
	h.TLSLatency = 0
	h.CertExpiry = 0
	h.CertIssuerOrg = ""
	h.TLSKeyExchange = ""
	h.TLSCipherSuite = ""
}

func probeDNS(ctx context.Context, m model.Monitor, h *model.Heartbeat) error {
	resolver := &net.Resolver{PreferGo: true}
	if m.DNSServer != "" {
		resolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, m.DNSServer)
		}
	}
	results := []string{}
	var err error
	switch m.DNSRecord {
	case "A", "AAAA":
		network := "ip4"
		if m.DNSRecord == "AAAA" {
			network = "ip6"
		}
		var ips []net.IP
		ips, err = resolver.LookupIP(ctx, network, m.Target)
		for _, ip := range ips {
			results = append(results, ip.String())
		}
	case "CNAME":
		var name string
		name, err = resolver.LookupCNAME(ctx, m.Target)
		if name != "" {
			results = append(results, name)
		}
	case "MX":
		var records []*net.MX
		records, err = resolver.LookupMX(ctx, m.Target)
		for _, r := range records {
			results = append(results, r.Host)
		}
	case "NS":
		var records []*net.NS
		records, err = resolver.LookupNS(ctx, m.Target)
		for _, r := range records {
			results = append(results, r.Host)
		}
	case "TXT":
		results, err = resolver.LookupTXT(ctx, m.Target)
	default:
		return fmt.Errorf("不支持的 DNS 记录类型")
	}
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("DNS 未返回匹配记录")
	}
	if m.Expected != "" {
		matched := false
		for _, result := range results {
			if strings.EqualFold(strings.TrimSuffix(result, "."), strings.TrimSuffix(m.Expected, ".")) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("DNS 返回结果与期望值不一致")
		}
	}
	h.Message = "DNS 解析成功：" + strings.Join(results, ", ")
	if len(h.Message) > 2000 {
		h.Message = h.Message[:2000]
	}
	return nil
}
