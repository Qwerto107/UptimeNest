package monitor

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	pingDLL     = windows.NewLazySystemDLL("iphlpapi.dll")
	icmpCreate  = pingDLL.NewProc("IcmpCreateFile")
	icmp6Create = pingDLL.NewProc("Icmp6CreateFile")
	icmpSend    = pingDLL.NewProc("IcmpSendEcho2")
	icmp6Send   = pingDLL.NewProc("Icmp6SendEcho2")
	icmpClose   = pingDLL.NewProc("IcmpCloseHandle")
	// 系统调用在超时前不能强行释放缓冲区，单独限制尚未完成的调用数量。
	windowsPingSlots = make(chan struct{}, 32)
)

// 仅映射 IPv4 返回结构的固定前缀；Data 按当前进程的指针宽度对齐。
type windowsEchoReply struct {
	Address       [4]byte
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
}

// ICMPV6_ECHO_REPLY 的数据紧随此36字节头，地址结构不包含地址族。
type windowsEcho6Reply struct {
	Port          uint16
	_             uint16
	FlowInfo      uint32
	Address       [16]byte
	ScopeID       uint32
	Status        uint32
	RoundTripTime uint32
}

func pingNative(ctx context.Context, address net.IPAddr, payload []byte) error {
	select {
	case windowsPingSlots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	done := make(chan error, 1)
	go func() {
		defer func() { <-windowsPingSlots }()
		// Windows 同步 ICMP API 自行超时；取消后仍由本协程持有缓冲区并收尾。
		done <- sendWindowsEcho(ctx, address, payload)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func sendWindowsEcho(ctx context.Context, address net.IPAddr, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return fmt.Errorf("ICMP 探测缺少超时设置")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return context.DeadlineExceeded
	}
	timeout := uintptr(max(1, (remaining+time.Millisecond-1)/time.Millisecond))
	create, send := icmpCreate, icmpSend
	if address.IP.To4() == nil {
		create, send = icmp6Create, icmp6Send
	}
	for _, proc := range []*windows.LazyProc{create, send, icmpClose} {
		if err := proc.Find(); err != nil {
			return err
		}
	}
	handle, _, err := create.Call()
	if handle == ^uintptr(0) || handle == 0 {
		return fmt.Errorf("创建 ICMP 句柄失败：%w", err)
	}
	defer icmpClose.Call(handle)
	reply := make([]byte, 512)
	var pinned runtime.Pinner
	pinned.Pin(&payload[0])
	pinned.Pin(&reply[0])
	defer pinned.Unpin()
	var count uintptr
	if ip := address.IP.To4(); ip != nil {
		count, _, err = send.Call(handle, 0, 0, 0,
			uintptr(binary.LittleEndian.Uint32(ip)), uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), 0,
			uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)), timeout)
	} else {
		source := windows.RawSockaddrInet6{Family: windows.AF_INET6}
		target := windows.RawSockaddrInet6{Family: windows.AF_INET6}
		copy(target.Addr[:], address.IP.To16())
		if address.Zone != "" {
			index, zoneErr := strconv.ParseUint(address.Zone, 10, 32)
			if zoneErr != nil {
				iface, err := net.InterfaceByName(address.Zone)
				if err != nil {
					return err
				}
				index = uint64(iface.Index)
			}
			target.Scope_id = uint32(index)
		}
		count, _, err = send.Call(handle, 0, 0, 0, uintptr(unsafe.Pointer(&source)), uintptr(unsafe.Pointer(&target)),
			uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), 0, uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)), timeout)
	}
	runtime.KeepAlive(payload)
	if count == 0 {
		if err == windows.Errno(11010) {
			return context.DeadlineExceeded
		}
		return fmt.Errorf("ICMP 请求失败：%w", err)
	}
	if address.IP.To4() == nil {
		header := (*windowsEcho6Reply)(unsafe.Pointer(&reply[0]))
		if err := windowsPingStatus(header.Status); err != nil {
			return err
		}
		offset := int(unsafe.Sizeof(*header))
		if !net.IP(header.Address[:]).Equal(address.IP) || !bytes.Equal(reply[offset:offset+len(payload)], payload) {
			return fmt.Errorf("ICMP 应答来源或载荷不匹配")
		}
	} else {
		header := (*windowsEchoReply)(unsafe.Pointer(&reply[0]))
		if err := windowsPingStatus(header.Status); err != nil {
			return err
		}
		start := uintptr(unsafe.Pointer(&reply[0]))
		// 先验证系统返回的数据指针位于自有缓冲区，再转换为安全切片索引。
		if header.Data < start || header.Data-start > uintptr(len(reply)-len(payload)) || int(header.DataSize) != len(payload) {
			return fmt.Errorf("ICMP 应答载荷无效")
		}
		offset := int(header.Data - start)
		if !net.IP(header.Address[:]).Equal(address.IP) || !bytes.Equal(reply[offset:offset+len(payload)], payload) {
			return fmt.Errorf("ICMP 应答来源或载荷不匹配")
		}
	}
	return nil
}

func windowsPingStatus(status uint32) error {
	if status == 0 {
		return nil
	}
	if status == 11010 {
		return context.DeadlineExceeded
	}
	return fmt.Errorf("Ping 目标不可达（ICMP 状态 %d）", status)
}
