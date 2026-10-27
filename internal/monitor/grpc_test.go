package monitor

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	health "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type grpcTestHealth struct {
	health.UnimplementedHealthServer
	check func(context.Context, *health.HealthCheckRequest) (*health.HealthCheckResponse, error)
}

func (s *grpcTestHealth) Check(ctx context.Context, request *health.HealthCheckRequest) (*health.HealthCheckResponse, error) {
	return s.check(ctx, request)
}

func startGRPCServer(t *testing.T, config *tls.Config, service health.HealthServer) model.Monitor {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	options := []grpc.ServerOption{}
	if config != nil {
		options = append(options, grpc.Creds(credentials.NewTLS(config)))
	}
	server := grpc.NewServer(options...)
	if service != nil {
		health.RegisterHealthServer(server, service)
	}
	go server.Serve(listener)
	t.Cleanup(server.Stop)
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	return model.Monitor{ID: 1, Type: "grpc", Target: host, Port: port, Timeout: 2, GRPC: &model.GRPCOptions{Security: "none"}}
}

func TestGRPCHealthAndMetadata(t *testing.T) {
	service := &grpcTestHealth{check: func(ctx context.Context, request *health.HealthCheckRequest) (*health.HealthCheckResponse, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		if request.Service != "private.Service" || strings.Join(md.Get("authorization"), "") != "Bearer private-token" || !strings.HasPrefix(strings.Join(md.Get("user-agent"), ""), "gRPC-test") {
			t.Error("service name, Metadata or user agent was not forwarded")
		}
		return &health.HealthCheckResponse{Status: health.HealthCheckResponse_SERVING}, nil
	}}
	m := startGRPCServer(t, nil, service)
	m.GRPC.ServiceName = "private.Service"
	m.GRPC.Metadata = `{"Authorization":"Bearer private-token"}`
	h := Probe(context.Background(), m, "gRPC-test")
	if h.Status != "up" || h.HTTPVersion != "HTTP/2.0" || h.Latency <= 0 || h.TLSVersion != "" || h.CertExpiry != 0 {
		t.Fatalf("bad plaintext health result: %+v", h)
	}
}

func TestGRPCUnhealthyStatusesAndErrors(t *testing.T) {
	for _, state := range []health.HealthCheckResponse_ServingStatus{health.HealthCheckResponse_UNKNOWN, health.HealthCheckResponse_NOT_SERVING, health.HealthCheckResponse_SERVICE_UNKNOWN} {
		t.Run(state.String(), func(t *testing.T) {
			m := startGRPCServer(t, nil, &grpcTestHealth{check: func(context.Context, *health.HealthCheckRequest) (*health.HealthCheckResponse, error) {
				return &health.HealthCheckResponse{Status: state}, nil
			}})
			h := Probe(context.Background(), m, "")
			if h.Status != "down" || h.Message != "gRPC 健康状态："+state.String() || h.HTTPVersion == "" {
				t.Fatalf("unhealthy result: %+v", h)
			}
		})
	}
	for _, code := range []codes.Code{codes.NotFound, codes.Unauthenticated, codes.PermissionDenied, codes.Internal} {
		t.Run(code.String(), func(t *testing.T) {
			m := startGRPCServer(t, nil, &grpcTestHealth{check: func(context.Context, *health.HealthCheckRequest) (*health.HealthCheckResponse, error) {
				return nil, status.Error(code, "private-token / private.Service")
			}})
			h := Probe(context.Background(), m, "")
			if h.Status != "down" || strings.Contains(h.Message, "private") {
				t.Fatalf("RPC status or privacy failure: %+v", h)
			}
		})
	}
	m := startGRPCServer(t, nil, nil)
	if h := Probe(context.Background(), m, ""); h.Status != "down" || !strings.Contains(h.Message, "未实现标准健康检查") {
		t.Fatalf("missing health interface treated as healthy: %+v", h)
	}
}

func TestGRPCDeadlineAndCancellation(t *testing.T) {
	m := startGRPCServer(t, nil, &grpcTestHealth{check: func(ctx context.Context, _ *health.HealthCheckRequest) (*health.HealthCheckResponse, error) {
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	if h := Probe(ctx, m, ""); h.Status != "down" || !strings.HasSuffix(h.Message, "context deadline exceeded") || time.Since(start) > time.Second {
		t.Fatalf("deadline not respected: %+v", h)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if h := Probe(ctx, m, ""); h.Status != "down" || !strings.HasSuffix(h.Message, "context canceled") {
		t.Fatalf("cancellation not respected: %+v", h)
	}
}

func grpcTestCertificate(t *testing.T, validHost bool, expired bool) (tls.Certificate, string, int64) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(24 * time.Hour)
	if expired {
		expiry = time.Now().Add(-time.Hour)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{Organization: []string{"UptimeNest tests"}}, NotBefore: time.Now().Add(-48 * time.Hour), NotAfter: expiry, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{"wrong.example"}}
	if validHost {
		cert.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, public, private)
	if err != nil {
		t.Fatal(err)
	}
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}, ca, expiry.Unix() * 1000
}

func TestGRPCTLSVerificationAndCertificateDetails(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		host, expired, trust bool
	}{{"trusted", true, false, true}, {"untrusted", true, false, false}, {"wrong hostname", false, false, true}, {"expired", true, true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			cert, ca, expiry := grpcTestCertificate(t, tc.host, tc.expired)
			m := startGRPCServer(t, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, &grpcTestHealth{check: func(context.Context, *health.HealthCheckRequest) (*health.HealthCheckResponse, error) {
				return &health.HealthCheckResponse{Status: health.HealthCheckResponse_SERVING}, nil
			}})
			m.GRPC.Security = "tls"
			if tc.trust {
				m.GRPC.CACert = ca
			}
			h := Probe(context.Background(), m, "")
			if h.CertExpiry != expiry || h.TLSLatency <= 0 {
				t.Fatalf("lost certificate or handshake time: %+v", h)
			}
			if tc.host && tc.trust && !tc.expired {
				if h.Status != "up" || h.TLSVersion == "" || h.TLSCipherSuite == "" || h.CertIssuerOrg != "UptimeNest tests" {
					t.Fatalf("lost negotiated TLS details: %+v", h)
				}
			} else if h.Status != "down" {
				t.Fatalf("invalid certificate accepted: %+v", h)
			}
		})
	}
}

func TestValidateGRPC(t *testing.T) {
	m := model.Monitor{Target: "::1", Port: 443}
	if err := ValidateGRPC(&m); err != nil || m.GRPC.Security != "tls" {
		t.Fatalf("TLS default or IPv6 rejected: %+v, %v", m, err)
	}
	for _, target := range []string{"grpc://example.com", "host/path", "user@host", "host:443", "host\r\n"} {
		m := model.Monitor{Target: target, Port: 443}
		if ValidateGRPC(&m) == nil {
			t.Fatalf("accepted invalid target %q", target)
		}
	}
	for _, text := range []string{`null`, `[]`, `{"token":123}`, `{"x":"a\nInjected"}`, `{"x":"中文"}`, `{"grpc-test":"x"}`, `{"token-bin":"x"}`, `{"content-type":"x"}`, `{"X":"1","x":"2"}`, `{"bad key":"x"}`} {
		m := model.Monitor{Target: "example.com", Port: 443, GRPC: &model.GRPCOptions{Metadata: text}}
		if ValidateGRPC(&m) == nil {
			t.Fatalf("accepted invalid Metadata %q", text)
		}
	}
	for _, v := range []model.GRPCOptions{{Security: "invalid"}, {CACert: "invalid PEM"}, {ServiceName: "bad\nservice"}, {Metadata: strings.Repeat("x", 65537)}} {
		m := model.Monitor{Target: "example.com", Port: 443, GRPC: &v}
		if ValidateGRPC(&m) == nil {
			t.Fatalf("accepted invalid gRPC options: %+v", v)
		}
	}
}
