// UptimeNest 同时运行管理接口、访客接口、检测任务与前端静态页面。
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/Qwerto107/UptimeNest/internal/api"
	"github.com/Qwerto107/UptimeNest/internal/config"
	"github.com/Qwerto107/UptimeNest/internal/monitor"
	"github.com/Qwerto107/UptimeNest/internal/notify"
	"github.com/Qwerto107/UptimeNest/internal/store"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			slog.Error("健康检查失败", "error", err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("UptimeNest 已停止", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	openCtx, openCancel := context.WithTimeout(ctx, 30*time.Second)
	s, err := store.Open(openCtx, cfg.DBType, cfg.DSN)
	openCancel()
	if err != nil {
		return err
	}
	defer s.Close()
	engine := monitor.New(ctx, s, cfg.Workers)
	notificationsDone := make(chan struct{})
	go func() { defer close(notificationsDone); notify.Run(ctx, s) }()
	defer func() { cancel(); <-notificationsDone }()
	if err = engine.Reload(ctx); err != nil {
		return err
	}
	cleanupDone := make(chan struct{})
	go func() { defer close(cleanupDone); monitor.RunCleanup(ctx, s) }()
	server := &http.Server{Addr: cfg.Address, Handler: api.New(s, engine, cfg), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan error, 1)
	go func() {
		slog.Info("UptimeNest 已启动", "address", cfg.Address, "database", cfg.DBType)
		done <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
	case err = <-done:
		if err == http.ErrServerClosed {
			err = nil
		}
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
		slog.Error("关闭 HTTP 服务失败", "error", shutdownErr)
	}
	engine.Wait()
	<-cleanupDone
	return err
}
