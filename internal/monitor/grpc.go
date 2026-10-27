package monitor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	health "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var grpcHostPattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,252}$`)
var grpcMetadataKey = regexp.MustCompile(`^[a-z0-9_.-]+$`)

// ValidateGRPC 校验直连目标、Metadata 和 CA，未指定连接安全模式时默认使用 TLS。
func ValidateGRPC(m *model.Monitor) error {
	if (net.ParseIP(m.Target) == nil && !grpcHostPattern.MatchString(m.Target)) || m.Port < 1 || m.Port > 65535 {
		return fmt.Errorf("gRPC 目标应为主机名或 IP，端口应为 1 至 65535")
	}
	v := m.GRPCConfig()
	v.ServiceName = strings.TrimSpace(v.ServiceName)
	if v.Security != "tls" && v.Security != "none" {
		return fmt.Errorf("请选择有效的 gRPC 连接安全模式")
	}
	if len(v.ServiceName) > 1000 || strings.ContainsAny(v.ServiceName, "\r\n\x00") {
		return fmt.Errorf("gRPC 服务名称最多 1000 字节，且不能包含换行或空字符")
	}
	if len(v.Metadata) > 64<<10 || len(v.CACert) > 64<<10 {
		return fmt.Errorf("gRPC Metadata 和 CA 证书各最多 64 KiB")
	}
	if _, err := grpcMetadata(v.Metadata); err != nil {
		return err
	}
	if v.Security == "tls" {
		if _, err := grpcTLSConfig(m.Target, v.CACert); err != nil {
			return err
		}
	}
	m.GRPC = &v
	return nil
}

// 非二进制 Metadata 只接受可打印 ASCII；协议保留字段由 gRPC 客户端管理。
func grpcMetadata(text string) (metadata.MD, error) {
	result := metadata.MD{}
	if strings.TrimSpace(text) == "" {
		return result, nil
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(text), &values); err != nil || values == nil {
		return nil, fmt.Errorf("gRPC Metadata 应为 JSON 对象，名称和值均为字符串")
	}
	if len(values) > 64 {
		return nil, fmt.Errorf("gRPC Metadata 最多 64 项")
	}
	for key, value := range values {
		key = strings.ToLower(key)
		if len(key) > 100 || !grpcMetadataKey.MatchString(key) || strings.HasPrefix(key, "grpc-") || strings.HasSuffix(key, "-bin") {
			return nil, fmt.Errorf("gRPC Metadata 名称无效，不支持 grpc- 保留前缀或 -bin 二进制字段")
		}
		switch key {
		case "content-type", "content-length", "te", "user-agent", "host", "connection", "transfer-encoding":
			return nil, fmt.Errorf("gRPC 协议请求头由系统自动管理")
		}
		for _, c := range value {
			if c < 0x20 || c > 0x7e {
				return nil, fmt.Errorf("gRPC Metadata 值只能包含可打印 ASCII 字符")
			}
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("gRPC Metadata 名称不能重复，忽略大小写")
		}
		result.Set(key, value)
	}
	return result, nil
}

func grpcTLSConfig(host, ca string) (*tls.Config, error) {
	config := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}
	if ca != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(ca)) {
			return nil, fmt.Errorf("gRPC CA 证书不是有效 PEM")
		}
		config.RootCAs = pool
	}
	return config, nil
}

// gRPC 在连接协程中握手；用锁传递 TLS 快照，避免与检测返回后的心跳读取产生竞争。
type grpcTLSResult struct {
	sync.Mutex
	state   *tls.ConnectionState
	latency float64
	err     error
}

type grpcTLSCredentials struct {
	credentials.TransportCredentials
	result *grpcTLSResult
}

func (c *grpcTLSCredentials) ClientHandshake(ctx context.Context, authority string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	start := time.Now()
	secured, info, err := c.TransportCredentials.ClientHandshake(ctx, authority, conn)
	c.result.Lock()
	c.result.latency = float64(time.Since(start).Microseconds()) / 1000
	c.result.err = err
	c.result.state = nil
	if value, ok := info.(credentials.TLSInfo); ok {
		c.result.state = &value.State
	}
	c.result.Unlock()
	return secured, info, err
}

func (c *grpcTLSCredentials) Clone() credentials.TransportCredentials {
	return &grpcTLSCredentials{TransportCredentials: c.TransportCredentials.Clone(), result: c.result}
}

func probeGRPC(ctx context.Context, m model.Monitor, ua string, h *model.Heartbeat) error {
	if err := ValidateGRPC(&m); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(m.Timeout)*time.Second)
	defer cancel()
	v := m.GRPCConfig()
	md, _ := grpcMetadata(v.Metadata)
	ctx = metadata.NewOutgoingContext(ctx, md)
	result := &grpcTLSResult{}
	var transport credentials.TransportCredentials = insecure.NewCredentials()
	if v.Security == "tls" {
		config, err := grpcTLSConfig(m.Target, v.CACert)
		if err != nil {
			return err
		}
		transport = &grpcTLSCredentials{TransportCredentials: credentials.NewTLS(config), result: result}
	}
	if ua == "" {
		ua = model.DefaultUserAgent
	}
	// 固定使用目标主机和端口，禁止解析器服务配置改变检测地址、重试或健康检查行为。
	conn, err := grpc.NewClient("passthrough:///"+net.JoinHostPort(m.Target, strconv.Itoa(m.Port)),
		grpc.WithTransportCredentials(transport), grpc.WithNoProxy(), grpc.WithDisableServiceConfig(),
		grpc.WithDisableRetry(), grpc.WithUserAgent(ua), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(64<<10)))
	if err != nil {
		return fmt.Errorf("无法创建 gRPC 健康检查客户端")
	}
	defer func() {
		conn.Close()
		result.Lock()
		defer result.Unlock()
		h.TLSLatency = result.latency
		if result.state != nil {
			readTLS(*result.state, h)
		} else if result.err != nil {
			readFailedCertificate(result.err, h)
		}
	}()
	var remote peer.Peer
	response, err := health.NewHealthClient(conn).Check(ctx, &health.HealthCheckRequest{Service: v.ServiceName}, grpc.Peer(&remote))
	if remote.Addr != nil {
		h.HTTPVersion = "HTTP/2.0"
	}
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("gRPC 检测中断：%w", ctx.Err())
		}
		// 不保存远端错误详情，防止服务端将 Metadata 凭据回显进日志或通知。
		switch status.Code(err) {
		case codes.Unimplemented:
			return fmt.Errorf("gRPC 服务未实现标准健康检查接口")
		case codes.NotFound:
			return fmt.Errorf("gRPC 健康检查未找到指定服务")
		default:
			result.Lock()
			tlsFailed := result.err != nil
			result.Unlock()
			if tlsFailed {
				return fmt.Errorf("gRPC TLS 握手失败，请检查证书和协议配置")
			}
			return fmt.Errorf("gRPC 健康检查失败（%s）", status.Code(err))
		}
	}
	if response.Status != health.HealthCheckResponse_SERVING {
		return fmt.Errorf("gRPC 健康状态：%s", response.Status)
	}
	h.Message = "gRPC 健康检查成功（SERVING）"
	return nil
}
