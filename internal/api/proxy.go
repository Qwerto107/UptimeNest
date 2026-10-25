package api

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func trustedProxyPrefixes(values []string) ([]netip.Prefix, error) {
	if len(values) > 100 {
		return nil, fmt.Errorf("可信代理最多设置 100 项")
	}
	prefixes := make([]netip.Prefix, 0, len(values))
	for i, value := range values {
		value = strings.TrimSpace(value)
		if len(value) > 100 {
			return nil, fmt.Errorf("第 %d 项可信代理应为有效的 IP 或 CIDR", i+1)
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			addr, addrErr := netip.ParseAddr(value)
			if addrErr != nil || addr.Zone() != "" {
				return nil, fmt.Errorf("第 %d 项可信代理应为有效的 IP 或 CIDR", i+1)
			}
			addr = addr.Unmap()
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

func clientIP(r *http.Request, trusted []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return r.RemoteAddr
	}
	peer = peer.WithZone("").Unmap()
	isTrusted := func(addr netip.Addr) bool {
		for _, prefix := range trusted {
			if prefix.Contains(addr) {
				return true
			}
		}
		return false
	}
	if !isTrusted(peer) {
		return peer.String()
	}
	forwarded := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
	if forwarded == "" {
		values := r.Header.Values("X-Real-IP")
		if len(values) == 1 {
			addr, err := netip.ParseAddr(strings.TrimSpace(values[0]))
			if err == nil && addr.Zone() == "" {
				return addr.Unmap().String()
			}
		}
		return peer.String()
	}
	hops := strings.Split(forwarded, ",")
	if len(forwarded) > 8192 || len(hops) > 64 {
		return peer.String()
	}
	// 仅越过可信代理；右侧第一个不可信地址就是客户端，不能信任其左侧内容。
	current := peer
	for i := len(hops) - 1; i >= 0 && isTrusted(current); i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil || addr.Zone() != "" {
			return peer.String()
		}
		current = addr.Unmap()
	}
	return current.String()
}
