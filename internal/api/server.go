// Package api 提供访客与管理员隔离的 HTTP 接口。
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/config"
	"github.com/Qwerto107/UptimeNest/internal/monitor"
	"github.com/Qwerto107/UptimeNest/internal/store"
	"github.com/Qwerto107/UptimeNest/internal/webui"
)

type Server struct {
	store    *store.Store
	engine   *monitor.Engine
	config   config.Config
	setupMu  sync.Mutex
	limitMu  sync.Mutex
	attempts map[string]attempt
}

func New(s *store.Store, engine *monitor.Engine, cfg config.Config) http.Handler {
	a := &Server{store: s, engine: engine, config: cfg, attempts: map[string]attempt{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/public/bootstrap", a.bootstrap)
	mux.HandleFunc("GET /api/public/panels/{slug}", a.publicPanel)
	mux.HandleFunc("GET /api/admin/session", a.session)
	mux.HandleFunc("POST /api/admin/setup", a.setup)
	mux.HandleFunc("POST /api/admin/session", a.login)
	mux.Handle("DELETE /api/admin/session", a.auth(a.logout))
	mux.Handle("PUT /api/admin/password", a.auth(a.password))
	mux.Handle("GET /api/admin/monitors", a.auth(a.monitors))
	mux.Handle("POST /api/admin/monitors", a.auth(a.saveMonitor))
	mux.Handle("PUT /api/admin/monitors/{id}", a.auth(a.saveMonitor))
	mux.Handle("DELETE /api/admin/monitors/{id}", a.auth(a.deleteMonitor))
	mux.Handle("PUT /api/admin/monitors/{id}/enabled", a.auth(a.setEnabled))
	mux.Handle("GET /api/admin/monitors/{id}/heartbeats", a.auth(a.heartbeats))
	mux.Handle("DELETE /api/admin/monitors/{id}/heartbeats", a.auth(a.clearHeartbeats))
	mux.Handle("GET /api/admin/monitors/{id}/logs", a.auth(a.logs))
	mux.Handle("DELETE /api/admin/monitors/{id}/logs", a.auth(a.clearLogs))
	mux.Handle("GET /api/admin/groups", a.auth(a.groups))
	mux.Handle("POST /api/admin/groups", a.auth(a.saveGroup))
	mux.Handle("PUT /api/admin/groups/{id}", a.auth(a.saveGroup))
	mux.Handle("DELETE /api/admin/groups/{id}", a.auth(a.deleteGroup))
	mux.Handle("GET /api/admin/panels", a.auth(a.panels))
	mux.Handle("POST /api/admin/panels", a.auth(a.savePanel))
	mux.Handle("PUT /api/admin/panels/{id}", a.auth(a.savePanel))
	mux.Handle("DELETE /api/admin/panels/{id}", a.auth(a.deletePanel))
	mux.Handle("GET /api/admin/settings", a.auth(a.settings))
	mux.Handle("PUT /api/admin/settings", a.auth(a.saveSettings))
	mux.Handle("POST /api/admin/cleanup", a.auth(a.cleanup))
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.DB.PingContext(ctx); err != nil {
			writeError(w, 503, "数据库暂不可用")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, 404, "接口不存在或请求方式不正确")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "请求方式不正确")
			return
		}
		a.static(w, r)
	})
	return a.middleware(mux)
}

// middleware 禁止跨源写入，限制请求体并统一安全响应头。
func (a *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; font-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
					writeError(w, 403, "不允许跨站提交")
					return
				}
			}
			if r.ContentLength != 0 && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				writeError(w, 415, "请求必须使用 JSON")
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		defer func() {
			if err := recover(); err != nil {
				slog.Error("处理请求异常", "error", fmt.Sprint(err))
				writeError(w, 500, "服务暂时不可用")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *Server) static(w http.ResponseWriter, r *http.Request) {
	files := webui.Files()
	if a.config.WebDir != "" {
		files = os.DirFS(a.config.WebDir)
	} else if files == nil {
		// 未启用内嵌构建时，继续支持原有的源码运行方式。
		files = os.DirFS("web/dist")
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	info, err := fs.Stat(files, name)
	if err == nil && !info.IsDir() {
		http.FileServer(http.FS(files)).ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/") || path.Ext(r.URL.Path) != "" {
		http.NotFound(w, r)
		return
	}
	if _, err := fs.Stat(files, "index.html"); err != nil {
		http.Error(w, "前端尚未构建，请先在 web 目录执行 npm run build", 503)
		return
	}
	// 页面路由统一回退到首页，不对访客开放目录列表。
	w.Header().Set("Cache-Control", "no-cache")
	page := r.Clone(r.Context())
	page.URL.Path = "/"
	page.URL.RawPath = ""
	http.FileServer(http.FS(files)).ServeHTTP(w, page)
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		writeError(w, 400, "请求数据格式不正确")
		return false
	}
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, 400, "请求只能包含一个 JSON 对象")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, 404, "记录不存在，请刷新页面")
	case errors.Is(err, store.ErrConflict):
		writeError(w, 409, "名称或访问路径已存在")
	case errors.Is(err, store.ErrInvalid):
		writeError(w, 400, err.Error())
	default:
		slog.Error("接口操作失败", "error", err)
		writeError(w, 500, "操作未完成，请稍后重试")
	}
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 400, "无效的记录编号")
		return 0, false
	}
	return id, true
}

func page(r *http.Request) (int64, int) {
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return before, limit
}
