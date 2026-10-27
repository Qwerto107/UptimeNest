package api

import (
	"container/list"
	"net/http"
	"net/netip"
	"sync"
	"time"
)

type ipLimitEntry struct {
	key      string
	tokens   float64
	last     time.Time
	failures int
	until    time.Time
}

// ipLimits 使用有界 LRU，表满只淘汰最久未使用的来源，不拒绝所有新 IP。
type ipLimits struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	order   list.List
}

func newIPLimits() *ipLimits {
	return &ipLimits{entries: map[string]*list.Element{}}
}

func (l *ipLimits) entry(key string, now time.Time, burst float64) *ipLimitEntry {
	if element := l.entries[key]; element != nil {
		l.order.MoveToFront(element)
		return element.Value.(*ipLimitEntry)
	}
	if len(l.entries) >= 10000 {
		oldest := l.order.Back()
		delete(l.entries, oldest.Value.(*ipLimitEntry).key)
		l.order.Remove(oldest)
	}
	value := &ipLimitEntry{key: key, tokens: burst, last: now}
	l.entries[key] = l.order.PushFront(value)
	return value
}

func (l *ipLimits) allow(key string, burst, perSecond float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	value := l.entry(key, now, burst)
	value.tokens = min(burst, value.tokens+now.Sub(value.last).Seconds()*perSecond)
	value.last = now
	if value.tokens < 1 {
		return false
	}
	value.tokens--
	return true
}

func (l *ipLimits) blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	element := l.entries[key]
	if element == nil {
		return false
	}
	l.order.MoveToFront(element)
	value := element.Value.(*ipLimitEntry)
	return time.Now().Before(value.until) && value.failures >= 10
}

func (l *ipLimits) failed(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	value := l.entry(key, now, 0)
	if !now.Before(value.until) {
		value.failures = 0
		value.until = now.Add(15 * time.Minute)
	}
	value.failures++
}

func (l *ipLimits) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if element := l.entries[key]; element != nil {
		delete(l.entries, key)
		l.order.Remove(element)
	}
}

func (a *Server) limitKey(r *http.Request) (string, error) {
	settings, err := a.store.Settings(r.Context())
	if err != nil {
		return "", err
	}
	trusted, err := trustedProxyPrefixes(settings.TrustedProxies)
	if err != nil {
		return "", err
	}
	key := clientIP(r, trusted)
	if addr, err := netip.ParseAddr(key); err == nil && addr.Is6() {
		key = netip.PrefixFrom(addr, 64).Masked().String()
	}
	return key, nil
}

func (a *Server) passwordAttempt(w http.ResponseWriter, r *http.Request) (string, bool) {
	key, err := a.limitKey(r)
	if err != nil {
		fail(w, err)
		return "", false
	}
	// 请求频率限制覆盖所有请求；失败次数只在验证失败后累计。
	if !a.authRequests.allow(key, 20, 1) {
		w.Header().Set("Retry-After", "1")
		writeError(w, 429, "请求过于频繁，请稍后重试")
		return "", false
	}
	if a.authFailures.blocked(key) {
		writeError(w, 429, "尝试次数过多，请 15 分钟后重试")
		return "", false
	}
	return key, true
}

func (a *Server) bcryptSlot(w http.ResponseWriter) bool {
	select {
	case a.bcryptSlots <- struct{}{}:
		return true
	default:
		w.Header().Set("Retry-After", "1")
		writeError(w, 429, "登录验证繁忙，请稍后重试")
		return false
	}
}
