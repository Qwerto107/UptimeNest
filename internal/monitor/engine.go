package monitor

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"sync"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
	"github.com/Qwerto107/UptimeNest/internal/store"
)

type worker struct {
	config model.Monitor
	cancel context.CancelFunc
}

// StateHandler 是未来通知模块的唯一接入点，当前版本没有通知实现。
type StateHandler func(context.Context, model.StateChange)

type Engine struct {
	store     *store.Store
	ctx       context.Context
	mu        sync.Mutex
	workers   map[int64]worker
	semaphore chan struct{}
	wg        sync.WaitGroup
	handlers  []StateHandler
}

func New(ctx context.Context, s *store.Store, concurrency int) *Engine {
	return &Engine{store: s, ctx: ctx, workers: map[int64]worker{}, semaphore: make(chan struct{}, concurrency)}
}

// Subscribe 注册进程内状态处理器；应在启动检测任务前调用。
func (e *Engine) Subscribe(handler StateHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers = append(e.handlers, handler)
}

// Reload 只重启配置有变化的监控；未修改的监控保持原来的检测节奏。
func (e *Engine) Reload(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	items, err := e.store.Monitors(ctx)
	if err != nil {
		return err
	}
	active := map[int64]bool{}
	for _, m := range items {
		if !m.Enabled {
			continue
		}
		active[m.ID] = true
		old, found := e.workers[m.ID]
		if found && reflect.DeepEqual(old.config, m) {
			continue
		}
		if found {
			old.cancel()
		}
		workerCtx, cancel := context.WithCancel(e.ctx)
		e.workers[m.ID] = worker{config: m, cancel: cancel}
		e.wg.Add(1)
		go func(config model.Monitor) { defer e.wg.Done(); e.run(workerCtx, config) }(m)
	}
	for id, task := range e.workers {
		if !active[id] {
			task.cancel()
			delete(e.workers, id)
		}
	}
	return nil
}

func (e *Engine) Wait() { e.wg.Wait() }

func (e *Engine) run(ctx context.Context, m model.Monitor) {
cycles:
	for {
		if ctx.Err() != nil {
			return
		}
		var final model.Heartbeat
		for attempt := 0; attempt <= m.Retries; attempt++ {
			select {
			case e.semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			settings, err := e.store.Settings(ctx)
			if err != nil {
				<-e.semaphore
				if ctx.Err() == nil {
					slog.Error("读取监控设置失败", "error", err)
				}
				if !wait(ctx, m.Interval) {
					return
				}
				continue cycles
			}
			probeCtx, cancel := context.WithTimeout(ctx, time.Duration(m.Timeout)*time.Second)
			final = Probe(probeCtx, m, settings.UserAgent)
			cancel()
			<-e.semaphore
			if ctx.Err() != nil {
				return
			}
			if final.Status == "up" || attempt == m.Retries {
				break
			}
			if !wait(ctx, m.RetryInterval) {
				return
			}
		}
		previous, err := e.store.Append(ctx, m, final)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, store.ErrStale) {
				return
			}
			slog.Error("保存心跳失败", "monitor", m.ID, "error", err)
		} else if previous != final.Status {
			e.mu.Lock()
			handlers := append([]StateHandler{}, e.handlers...)
			e.mu.Unlock()
			event := model.StateChange{MonitorID: m.ID, Previous: previous, Current: final.Status, Time: time.UnixMilli(final.Time)}
			for _, handler := range handlers {
				handler(ctx, event)
			}
		}
		if !wait(ctx, m.Interval) {
			return
		}
	}
}

func wait(ctx context.Context, seconds int) bool {
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// RunCleanup 启动时及每小时执行保留策略；与管理员手动清理使用同一实现。
func RunCleanup(ctx context.Context, s *store.Store) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		settings, err := s.Settings(ctx)
		if err == nil {
			_, err = s.Cleanup(ctx, settings.RetentionDays)
		}
		if err != nil && ctx.Err() == nil {
			slog.Error("自动清理历史失败", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
