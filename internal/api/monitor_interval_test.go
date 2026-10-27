package api

import (
	"fmt"
	"testing"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

func TestMonitorIntervalLimitsByProtocol(t *testing.T) {
	for _, kind := range []string{"ping", "http", "websocket", "grpc", "tcp", "dns"} {
		for _, interval := range []int{0, 1, 4, 5, 86400, 86401} {
			t.Run(fmt.Sprintf("%s/%d", kind, interval), func(t *testing.T) {
				m := model.Monitor{Name: "interval", Type: kind, Target: "example.com", Port: 443, DNSRecord: "A", Interval: interval, Timeout: 10, RetryInterval: 1}
				if kind == "http" {
					m.Target = "https://example.com"
				}
				if kind == "websocket" {
					m.Target = "wss://example.com"
				}
				minimum := 5
				if kind == "ping" {
					minimum = 1
				}
				err := validateMonitor(&m)
				valid := interval >= minimum && interval <= 86400
				if valid && err != nil {
					t.Fatalf("valid interval rejected: %v", err)
				}
				if !valid && (err == nil || err.Error() != fmt.Sprintf("心跳间隔应为 %d 至 86400 秒", minimum)) {
					t.Fatalf("unexpected validation: %v", err)
				}
			})
		}
	}
}
