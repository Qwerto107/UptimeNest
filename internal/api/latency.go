package api

import (
	"context"
	"net/http"
	"time"
)

func (a *Server) latencyHistory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ranges := map[string]time.Duration{
		"1h": time.Hour, "3h": 3 * time.Hour, "6h": 6 * time.Hour,
		"12h": 12 * time.Hour, "24h": 24 * time.Hour,
		"3d": 72 * time.Hour, "1w": 7 * 24 * time.Hour,
	}
	selected := r.URL.Query().Get("range")
	if selected == "" {
		selected = "24h"
	}
	duration, ok := ranges[selected]
	if !ok {
		writeError(w, 400, "请选择有效的延迟历史时间范围")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if _, err := a.store.Monitor(ctx, id); err != nil {
		fail(w, err)
		return
	}
	settings, err := a.store.Settings(ctx)
	if err != nil {
		fail(w, err)
		return
	}
	end := time.Now()
	start := end.Add(-duration).UnixMilli()
	availableStart := end.Add(-time.Duration(settings.RetentionDays) * 24 * time.Hour).UnixMilli()
	value, err := a.store.LatencyHistory(ctx, id, start, end.UnixMilli(), availableStart)
	if err != nil {
		if r.Context().Err() == nil {
			fail(w, err)
		}
		return
	}
	writeJSON(w, 200, value)
}
