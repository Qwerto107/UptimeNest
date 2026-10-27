package monitor

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

// probePing 共用解析与超时，平台实现只发送一次原生 ICMP Echo，不启动外部进程。
func probePing(ctx context.Context, m model.Monitor, h *model.Heartbeat) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(m.Timeout)*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, m.Target)
	if err != nil {
		return fmt.Errorf("Ping 地址解析失败：%w", err)
	}
	if len(addresses) == 0 {
		return fmt.Errorf("Ping 地址解析未返回 IP")
	}
	// 每个请求携带随机载荷，避免并发探测、迟到报文或其他来源被误判为成功。
	payload := make([]byte, 24)
	if _, err = rand.Read(payload); err != nil {
		return err
	}
	if err = pingNative(ctx, addresses[0], payload); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("Ping 未收到有效响应：%w", err)
	}
	h.Message = "收到 ICMP 应答"
	return nil
}
