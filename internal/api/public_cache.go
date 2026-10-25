package api

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var errPanelBusy = errors.New("状态面板请求繁忙")

type panelCacheKey struct {
	slug    string
	count   int
	version uint64
}

type panelCacheEntry struct {
	body    []byte
	expires time.Time
}

type panelFlight struct {
	done chan struct{}
	body []byte
	err  error
}

type panelCache struct {
	mu      sync.Mutex
	entries map[panelCacheKey]panelCacheEntry
	flights map[panelCacheKey]*panelFlight
	bytes   int
	slots   chan struct{}
}

func newPanelCache() *panelCache {
	return &panelCache{entries: map[panelCacheKey]panelCacheEntry{}, flights: map[panelCacheKey]*panelFlight{}, slots: make(chan struct{}, 2)}
}

func (c *panelCache) get(ctx context.Context, key panelCacheKey, load func(context.Context) ([]byte, error)) ([]byte, error) {
	c.mu.Lock()
	for k, entry := range c.entries {
		if k.version != key.version || time.Now().After(entry.expires) {
			delete(c.entries, k)
			c.bytes -= len(entry.body)
		}
	}
	if entry, ok := c.entries[key]; ok {
		c.mu.Unlock()
		return entry.body, nil
	}
	flight, ok := c.flights[key]
	if !ok {
		// 唯一请求和后台计算均有上限；相同请求共享结果，取消等待不取消其他访客。
		if len(c.flights) >= 32 {
			c.mu.Unlock()
			return nil, errPanelBusy
		}
		flight = &panelFlight{done: make(chan struct{})}
		c.flights[key] = flight
		go c.build(ctx, key, flight, load)
	}
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-flight.done:
		return flight.body, flight.err
	}
}

func (c *panelCache) build(parent context.Context, key panelCacheKey, flight *panelFlight, load func(context.Context) ([]byte, error)) {
	var body []byte
	var err error
	defer func() {
		if cause := recover(); cause != nil {
			err = fmt.Errorf("生成状态面板异常：%v", cause)
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		// 仅缓存已脱敏的序列化响应：最多32项、合计16MiB，单项不超过4MiB。
		if err == nil && len(body) <= 4<<20 {
			for len(c.entries) >= 32 || c.bytes+len(body) > 16<<20 {
				var oldest panelCacheKey
				var expiry time.Time
				for k, entry := range c.entries {
					if expiry.IsZero() || entry.expires.Before(expiry) {
						oldest, expiry = k, entry.expires
					}
				}
				c.bytes -= len(c.entries[oldest].body)
				delete(c.entries, oldest)
			}
			c.entries[key] = panelCacheEntry{body: body, expires: time.Now().Add(3 * time.Second)}
			c.bytes += len(body)
		}
		flight.body, flight.err = body, err
		delete(c.flights, key)
		close(flight.done)
	}()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 20*time.Second)
	defer cancel()
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
		body, err = load(ctx)
	case <-ctx.Done():
		err = ctx.Err()
	}
}
