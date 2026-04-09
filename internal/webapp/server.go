package webapp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-kratos/kratos/v2/log"

	"owntracks_server/internal/conf"
	"owntracks_server/internal/store"
)

// Run 启动 HTTP：静态页 + /api/*；阻塞至进程退出。
func Run(cfg *conf.WebConfig, lg *log.Helper) error {
	if v := os.Getenv("CLICKHOUSE_DSN"); v != "" {
		cfg.CHDSN = v
	}
	if cfg.CHDSN == "" {
		return fmt.Errorf("clickhouse: 请在 configs/config.yaml 中配置 clickhouse.dsn 或 host，或设置环境变量 CLICKHOUSE_DSN")
	}

	db, err := sql.Open("clickhouse", cfg.CHDSN)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("clickhouse ping: %w", err)
	}
	st := &store.CH{
		DB:        db,
		Database:  cfg.CHDatabase,
		Title:     cfg.Title,
		Members:   cfg.Members,
		MaxPoints: 4000,
	}

	var static http.Handler
	if cfg.StaticDir != "" {
		static = http.FileServer(http.Dir(cfg.StaticDir))
	} else {
		sub, err := fs.Sub(webFS, "web")
		if err != nil {
			return err
		}
		static = http.FileServer(http.FS(sub))
	}

	mux := http.NewServeMux()
	registerPubRoutes(mux, st, cfg, lg)
	mux.Handle("GET /api/meta", jsonHandler(func(w http.ResponseWriter, r *http.Request) error {
		return writeJSON(w, st.Meta(r.Context()))
	}))
	mux.Handle("GET /api/journey", jsonHandler(func(w http.ResponseWriter, r *http.Request) error {
		q := r.URL.Query()
		var from, to *time.Time
		if s := q.Get("from"); s != "" {
			t, err := parseQueryTime(s)
			if err != nil {
				return err
			}
			t = t.UTC()
			from = &t
		}
		if s := q.Get("to"); s != "" {
			t, err := parseQueryTime(s)
			if err != nil {
				return err
			}
			t = t.UTC()
			to = &t
		}
		interval := 300
		if s := q.Get("interval_sec"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				interval = n
			}
		}
		out, err := st.Journey(r.Context(), from, to, interval)
		if err != nil {
			return err
		}
		return writeJSON(w, out)
	}))
	mux.Handle("GET /api/stats", jsonHandler(func(w http.ResponseWriter, r *http.Request) error {
		q := r.URL.Query()
		var from, to *time.Time
		if s := q.Get("from"); s != "" {
			t, err := parseQueryTime(s)
			if err != nil {
				return err
			}
			t = t.UTC()
			from = &t
		}
		if s := q.Get("to"); s != "" {
			t, err := parseQueryTime(s)
			if err != nil {
				return err
			}
			t = t.UTC()
			to = &t
		}
		out, err := st.Stats(r.Context(), from, to)
		if err != nil {
			return err
		}
		return writeJSON(w, out)
	}))
	// 通配静态资源（Go 1.22+）；/api/* 已优先匹配
	mux.Handle("GET /{path...}", static)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 8 * time.Second,
	}
	lg.Infof("HTTP 服务 http://127.0.0.1%s （OwnTracks 上报 POST /pub/... ，控制台 /api/*）", cfg.Listen)
	return srv.ListenAndServe()
}

func jsonHandler(fn func(http.ResponseWriter, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := fn(w, r); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		}
	})
}

func writeJSON(w http.ResponseWriter, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func parseQueryTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
