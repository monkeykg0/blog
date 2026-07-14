// blog-stats: 博客真实访问量统计后端
//
// POST /api/view   {"path": "/blog/xxx/"}  前端 beacon 上报一次浏览
// GET  /api/views?path=/blog/xxx/          查询该页面累计 PV/UV
//
// Redis 负责：同一访客当日去重（UV）、IP 限流
// MySQL 负责：按日持久化 PV/UV
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

var (
	db  *sql.DB
	rdb *redis.Client
)

const schema = `
CREATE TABLE IF NOT EXISTS page_views_daily (
	date DATE NOT NULL,
	path VARCHAR(200) NOT NULL,
	pv BIGINT NOT NULL DEFAULT 0,
	uv BIGINT NOT NULL DEFAULT 0,
	PRIMARY KEY (date, path),
	KEY idx_path (path)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	var err error

	db, err = sql.Open("mysql", env("MYSQL_DSN", "blog:blog@tcp(127.0.0.1:3306)/blog?parseTime=true"))
	if err != nil {
		log.Fatalf("mysql open: %v", err)
	}
	db.SetMaxOpenConns(10)
	if err = db.Ping(); err != nil {
		log.Fatalf("mysql ping: %v", err)
	}
	if _, err = db.Exec(schema); err != nil {
		log.Fatalf("mysql schema: %v", err)
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     env("REDIS_ADDR", "127.0.0.1:6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
	})
	if err = rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis ping: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/view", handleView)
	mux.HandleFunc("GET /api/views", handleViews)
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	addr := env("LISTEN_ADDR", "127.0.0.1:8080")
	log.Printf("listening on %s", addr)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

// clientIP 取真实访客 IP：源站在 Cloudflare 之后，优先 CF-Connecting-IP。
// 前提：防火墙/OpenResty 已限制只有 Cloudflare 能访问源站，该头不可伪造。
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

var botMarkers = []string{"bot", "spider", "crawl", "curl", "wget", "python", "go-http", "headless"}

func isBot(ua string) bool {
	ua = strings.ToLower(ua)
	if ua == "" {
		return true
	}
	for _, m := range botMarkers {
		if strings.Contains(ua, m) {
			return true
		}
	}
	return false
}

func validPath(p string) bool {
	return strings.HasPrefix(p, "/") && len(p) <= 200 &&
		!strings.Contains(p, "..") && !strings.ContainsAny(p, " \t\n\r")
}

func handleView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil || !validPath(body.Path) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if isBot(r.UserAgent()) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	ip := clientIP(r)

	// 限流：单 IP 每 10 秒最多 30 次上报
	rlKey := "rl:" + ip
	n, err := rdb.Incr(ctx, rlKey).Result()
	if err == nil && n == 1 {
		rdb.Expire(ctx, rlKey, 10*time.Second)
	}
	if n > 30 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// UV 去重：同一 (IP+UA) 当日对同一页面只算一个访客
	h := sha256.Sum256([]byte(ip + "|" + r.UserAgent()))
	today := time.Now().Format("2006-01-02")
	seenKey := "seen:" + today + ":" + body.Path + ":" + hex.EncodeToString(h[:8])
	isNew, err := rdb.SetNX(ctx, seenKey, 1, 26*time.Hour).Result()
	if err != nil {
		log.Printf("redis setnx: %v", err)
	}
	uvDelta := 0
	if isNew {
		uvDelta = 1
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO page_views_daily (date, path, pv, uv) VALUES (?, ?, 1, ?)
		 ON DUPLICATE KEY UPDATE pv = pv + 1, uv = uv + ?`,
		today, body.Path, uvDelta, uvDelta)
	if err != nil {
		log.Printf("mysql insert: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleViews(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !validPath(path) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var pv, uv int64
	err := db.QueryRowContext(r.Context(),
		`SELECT COALESCE(SUM(pv), 0), COALESCE(SUM(uv), 0) FROM page_views_daily WHERE path = ?`,
		path).Scan(&pv, &uv)
	if err != nil {
		log.Printf("mysql query: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"pv": pv, "uv": uv})
}
