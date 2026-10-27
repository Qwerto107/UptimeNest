//go:build !windows

package monitor

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"runtime"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func pingNative(ctx context.Context, address net.IPAddr, payload []byte) error {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return fmt.Errorf("当前平台不支持无特权 ICMP 探测")
	}
	network, local, protocol := "udp4", "0.0.0.0", 1
	var request, reply icmp.Type = ipv4.ICMPTypeEcho, ipv4.ICMPTypeEchoReply
	if address.IP.To4() == nil {
		network, local, protocol = "udp6", "::", 58
		request, reply = ipv6.ICMPTypeEchoRequest, ipv6.ICMPTypeEchoReply
	}
	// udp4/udp6 在这里是内核 ICMP datagram socket，不发送普通 UDP 数据。
	conn, err := icmp.ListenPacket(network, local)
	if err != nil {
		return fmt.Errorf("创建无特权 ICMP 套接字失败（Linux 请检查 ping_group_range）：%w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		if err = conn.SetDeadline(deadline); err != nil {
			return err
		}
	}
	id, sequence := int(binary.BigEndian.Uint16(payload[:2])), int(binary.BigEndian.Uint16(payload[2:4]))
	message := icmp.Message{Type: request, Body: &icmp.Echo{ID: id, Seq: sequence, Data: payload}}
	packet, err := message.Marshal(nil)
	if err != nil {
		return err
	}
	if _, err = conn.WriteTo(packet, &net.UDPAddr{IP: address.IP, Zone: address.Zone}); err != nil {
		return err
	}
	// Linux 会把 Echo ID 改成本地端口；载荷和序列号仍用于跨平台关联请求。
	checkID := false
	if local, ok := conn.LocalAddr().(*net.UDPAddr); runtime.GOOS == "linux" && ok && local.Port != 0 {
		id = local.Port
		checkID = true
	}
	buffer := make([]byte, 1500)
	for {
		n, peer, err := conn.ReadFrom(buffer)
		if err != nil {
			return err
		}
		remote, ok := peer.(*net.UDPAddr)
		if !ok || !remote.IP.Equal(address.IP) {
			continue
		}
		message, err := icmp.ParseMessage(protocol, buffer[:n])
		if err != nil || message.Type != reply || message.Code != 0 {
			continue
		}
		echo, ok := message.Body.(*icmp.Echo)
		if ok && (!checkID || echo.ID == id) && echo.Seq == sequence && bytes.Equal(echo.Data, payload) {
			return nil
		}
	}
}
