package notify

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/store"
)

// Run 使用数据库队列实现有限重试；网络调用发生在事务外，不占用检测线程。
func Run(ctx context.Context, s *store.Store) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	nextTimers := time.Time{}
	for ctx.Err() == nil {
		if time.Now().After(nextTimers) {
			if err := s.EvaluateAlertTimers(ctx); err != nil && ctx.Err() == nil {
				slog.Error("检查告警提醒失败", "error", err)
			}
			nextTimers = time.Now().Add(time.Minute)
		}
		for i := 0; i < 20 && ctx.Err() == nil; i++ {
			d, err := s.ClaimNotification(ctx, time.Now().UnixMilli())
			if err != nil {
				if ctx.Err() == nil {
					slog.Error("读取通知队列失败", "error", err)
				}
				break
			}
			if d == nil {
				break
			}
			status, message := "sent", ""
			next := int64(0)
			channel, err := s.NotificationChannel(ctx, d.ChannelID)
			if errors.Is(err, store.ErrNotFound) || (err == nil && !channel.Enabled) {
				status = "cancelled"
				err = nil
			} else if err == nil {
				var allowed bool
				allowed, err = s.NotificationAllowed(ctx, *d)
				if err == nil && !allowed {
					status = "cancelled"
				}
				if err == nil && allowed {
					settings, settingsErr := s.Settings(ctx)
					err = settingsErr
					if err == nil {
						sendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
						err = Send(sendCtx, channel, d.Event, settings)
						cancel()
						if err != nil {
							message = err.Error()
						}
					}
				}
			}
			if err != nil {
				status = "failed"
				if message == "" {
					message = "通知配置读取失败"
				}
				if d.Attempts < 4 {
					status = "pending"
					next = time.Now().Add(time.Duration([]int{30, 120, 600}[d.Attempts-1]) * time.Second).UnixMilli()
				}
			}
			if err = s.FinishNotification(ctx, *d, status, message, next); err != nil && ctx.Err() == nil {
				slog.Error("保存通知结果失败", "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
