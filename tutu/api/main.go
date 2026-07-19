// media-api: 兔兔听书屋媒体库后端
//
// 职责:专辑元数据(扫描 /opt/media 下的 album.json)+ 跨设备播放进度。
// 媒体文件本身由 OpenResty 静态伺服,不经过本服务。
//
//	GET  /api/media/library            专辑列表(摘要,不含曲目)
//	GET  /api/media/album/{id}         专辑详情(含曲目)
//	GET  /api/media/progress/{album}   读取播放进度(按设备)
//	PUT  /api/media/progress/{album}   写入播放进度(body 为任意 JSON,64KB 上限)
//	POST /api/media/stats/heartbeat    上报收听时长 {"seconds": N}
//	GET  /api/media/stats              使用统计(设备数/总收听时长/设备明细)[管理]
//	POST /api/media/refresh            重扫媒体目录(导入新专辑后调用)[管理]
//	GET  /api/media/healthz            健康检查(免 token)
//
// 两层鉴权:普通端点校验 X-Media-Token(内嵌前端包,仅算门帘);
// [管理] 端点校验 X-Admin-Token(只存服务器 env,前端拿不到,保护统计隐私)。
// 无登录体系:进度与统计按 X-Device-Id 头区分用户(前端 localStorage 里的 UUID)。
// 环境变量:LISTEN、MEDIA_ROOT、MEDIA_TOKEN、MYSQL_DSN
package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var (
	db         *sql.DB
	mediaRoot  string
	token      string
	adminToken string
)

var schemas = []string{`
CREATE TABLE IF NOT EXISTS media_progress (
	user_id    VARCHAR(64)  NOT NULL,
	album_id   VARCHAR(64)  NOT NULL,
	data       JSON         NOT NULL,
	updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (user_id, album_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`, `
CREATE TABLE IF NOT EXISTS media_user_stats (
	user_id          VARCHAR(64) NOT NULL,
	listened_seconds INT UNSIGNED NOT NULL DEFAULT 0,
	first_seen       TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen        TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	last_ip          VARCHAR(45) NOT NULL DEFAULT '',
	PRIMARY KEY (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`}

// ---------- 专辑缓存 ----------

// Album 是 album.json 的完整结构;列表接口只回摘要字段。
type Album struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Title         string          `json:"title"`
	Artist        string          `json:"artist"`
	Cover         *string         `json:"cover"`
	TrackCount    int             `json:"trackCount"`
	TotalDuration int             `json:"totalDuration"`
	UpdatedAt     string          `json:"updatedAt"`
	Tracks        json.RawMessage `json:"tracks"`
}

type library struct {
	mu     sync.RWMutex
	albums map[string]*Album
}

var lib = &library{albums: map[string]*Album{}}

// scan 重扫 MEDIA_ROOT/{audio,video,image}/*/album.json
func (l *library) scan() error {
	found := map[string]*Album{}
	for _, typ := range []string{"audio", "video", "image"} {
		matches, err := filepath.Glob(filepath.Join(mediaRoot, typ, "*", "album.json"))
		if err != nil {
			return err
		}
		for _, m := range matches {
			raw, err := os.ReadFile(m)
			if err != nil {
				log.Printf("scan: 读取 %s 失败: %v", m, err)
				continue
			}
			var a Album
			if err := json.Unmarshal(raw, &a); err != nil {
				log.Printf("scan: 解析 %s 失败: %v", m, err)
				continue
			}
			if a.ID == "" {
				a.ID = filepath.Base(filepath.Dir(m))
			}
			if a.Type == "" {
				a.Type = typ
			}
			found[a.ID] = &a
		}
	}
	l.mu.Lock()
	l.albums = found
	l.mu.Unlock()
	log.Printf("scan: 载入 %d 个专辑", len(found))
	return nil
}

func (l *library) list() []map[string]any {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]map[string]any, 0, len(l.albums))
	for _, a := range l.albums {
		out = append(out, map[string]any{
			"id": a.ID, "type": a.Type, "title": a.Title, "artist": a.Artist,
			"cover": a.Cover, "trackCount": a.TrackCount,
			"totalDuration": a.TotalDuration, "updatedAt": a.UpdatedAt,
		})
	}
	// 先导入的排前面;updatedAt 为 ISO 日期/时间戳,字符串序即时间序,同刻按 id 兜底
	sort.Slice(out, func(i, j int) bool {
		ti, tj := out[i]["updatedAt"].(string), out[j]["updatedAt"].(string)
		if ti != tj {
			return ti < tj
		}
		return out[i]["id"].(string) < out[j]["id"].(string)
	})
	return out
}

func (l *library) get(id string) *Album {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.albums[id]
}

// ---------- 工具 ----------

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// auth 校验共享 token;失败直接写响应并返回 false。
func auth(w http.ResponseWriter, r *http.Request) bool {
	got := r.Header.Get("X-Media-Token")
	if got == "" {
		got = r.URL.Query().Get("token")
	}
	if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
		fail(w, http.StatusUnauthorized, "invalid token")
		return false
	}
	return true
}

// authAdmin 校验管理 token(不接受前端那个共享 token)
func authAdmin(w http.ResponseWriter, r *http.Request) bool {
	got := r.Header.Get("X-Admin-Token")
	if subtle.ConstantTimeCompare([]byte(got), []byte(adminToken)) != 1 {
		fail(w, http.StatusUnauthorized, "admin token required")
		return false
	}
	return true
}

// albumIDValid 防路径注入:与导入脚本约定一致
func albumIDValid(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// userID 取设备标识(前端生成的 UUID);缺失或格式非法时写 400 并返回 false。
func userID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.Header.Get("X-Device-Id")
	if len(id) < 8 || len(id) > 64 {
		fail(w, http.StatusBadRequest, "missing device id")
		return "", false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			fail(w, http.StatusBadRequest, "bad device id")
			return "", false
		}
	}
	return id, true
}

// clientIP 优先取 OpenResty 传来的 X-Real-IP
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// ---------- 处理器 ----------

func handleLibrary(w http.ResponseWriter, r *http.Request) {
	if !auth(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"albums": lib.list()})
}

func handleAlbum(w http.ResponseWriter, r *http.Request) {
	if !auth(w, r) {
		return
	}
	id := r.PathValue("id")
	if !albumIDValid(id) {
		fail(w, http.StatusBadRequest, "bad album id")
		return
	}
	a := lib.get(id)
	if a == nil {
		fail(w, http.StatusNotFound, "album not found")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func handleRefresh(w http.ResponseWriter, r *http.Request) {
	if !authAdmin(w, r) {
		return
	}
	if err := lib.scan(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"albums": len(lib.list())})
}

func handleGetProgress(w http.ResponseWriter, r *http.Request) {
	if !auth(w, r) {
		return
	}
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	album := r.PathValue("album")
	if !albumIDValid(album) {
		fail(w, http.StatusBadRequest, "bad album id")
		return
	}
	var data json.RawMessage
	var updated time.Time
	err := db.QueryRow(
		`SELECT data, updated_at FROM media_progress WHERE user_id = ? AND album_id = ?`,
		uid, album).Scan(&data, &updated)
	switch {
	case err == sql.ErrNoRows:
		writeJSON(w, http.StatusOK, map[string]any{"data": nil, "updatedAt": nil})
	case err != nil:
		fail(w, http.StatusInternalServerError, "db error")
	default:
		writeJSON(w, http.StatusOK, map[string]any{
			"data": data, "updatedAt": updated.UTC().Format(time.RFC3339Nano),
		})
	}
}

func handlePutProgress(w http.ResponseWriter, r *http.Request) {
	if !auth(w, r) {
		return
	}
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	album := r.PathValue("album")
	if !albumIDValid(album) {
		fail(w, http.StatusBadRequest, "bad album id")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil || !json.Valid(body) {
		fail(w, http.StatusBadRequest, "body must be valid json")
		return
	}
	_, err = db.Exec(
		`INSERT INTO media_progress (user_id, album_id, data) VALUES (?, ?, ?)
		 ON DUPLICATE KEY UPDATE data = VALUES(data)`,
		uid, album, body)
	if err != nil {
		log.Printf("progress put: %v", err)
		fail(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleHeartbeat 累加设备的真实收听秒数(前端播放中每 30 秒上报一次)
func handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !auth(w, r) {
		return
	}
	uid, ok := userID(w, r)
	if !ok {
		return
	}
	var in struct {
		Seconds int `json:"seconds"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&in); err != nil || in.Seconds <= 0 || in.Seconds > 300 {
		fail(w, http.StatusBadRequest, "seconds must be 1..300")
		return
	}
	_, err := db.Exec(
		`INSERT INTO media_user_stats (user_id, listened_seconds, last_ip) VALUES (?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   listened_seconds = listened_seconds + VALUES(listened_seconds),
		   last_ip = VALUES(last_ip)`,
		uid, in.Seconds, clientIP(r))
	if err != nil {
		log.Printf("heartbeat: %v", err)
		fail(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleStats 汇总:多少台设备用过、总共听了多久,附设备明细
func handleStats(w http.ResponseWriter, r *http.Request) {
	if !authAdmin(w, r) {
		return
	}
	var users, total int
	if err := db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(listened_seconds), 0) FROM media_user_stats`,
	).Scan(&users, &total); err != nil {
		fail(w, http.StatusInternalServerError, "db error")
		return
	}
	rows, err := db.Query(
		`SELECT user_id, listened_seconds, first_seen, last_seen, last_ip
		 FROM media_user_stats ORDER BY last_seen DESC LIMIT 100`)
	if err != nil {
		fail(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()
	devices := []map[string]any{}
	for rows.Next() {
		var id, ip string
		var sec int
		var first, last time.Time
		if err := rows.Scan(&id, &sec, &first, &last, &ip); err != nil {
			continue
		}
		devices = append(devices, map[string]any{
			"deviceId": id, "listenedSeconds": sec, "lastIp": ip,
			"firstSeen": first.UTC().Format(time.RFC3339),
			"lastSeen":  last.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users": users, "totalListenSeconds": total, "devices": devices,
	})
}

// ---------- main ----------

func main() {
	mediaRoot = env("MEDIA_ROOT", "/opt/media")
	token = env("MEDIA_TOKEN", "")
	if token == "" {
		log.Fatal("必须设置 MEDIA_TOKEN")
	}
	adminToken = env("ADMIN_TOKEN", "")
	if adminToken == "" {
		log.Fatal("必须设置 ADMIN_TOKEN")
	}

	var err error
	db, err = sql.Open("mysql", env("MYSQL_DSN", "blog:blog@tcp(127.0.0.1:3306)/blog?parseTime=true"))
	if err != nil {
		log.Fatalf("mysql open: %v", err)
	}
	db.SetMaxOpenConns(5)
	if err = db.Ping(); err != nil {
		log.Fatalf("mysql ping: %v", err)
	}
	// 旧版 media_progress 以共享 token 为主键,所有人共用一份历史,直接废弃重建
	var legacy string
	if err = db.QueryRow(
		`SELECT column_name FROM information_schema.columns
		 WHERE table_schema = DATABASE() AND table_name = 'media_progress' AND column_name = 'token'`,
	).Scan(&legacy); err == nil {
		if _, err = db.Exec(`DROP TABLE media_progress`); err != nil {
			log.Fatalf("删除旧表失败: %v", err)
		}
		log.Print("已删除旧版共享进度表 media_progress")
	}
	for _, s := range schemas {
		if _, err = db.Exec(s); err != nil {
			log.Fatalf("建表失败: %v", err)
		}
	}

	if err = lib.scan(); err != nil {
		log.Printf("初始扫描失败(不致命,可稍后 refresh): %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/media/library", handleLibrary)
	mux.HandleFunc("GET /api/media/album/{id}", handleAlbum)
	mux.HandleFunc("GET /api/media/progress/{album}", handleGetProgress)
	mux.HandleFunc("PUT /api/media/progress/{album}", handlePutProgress)
	mux.HandleFunc("POST /api/media/stats/heartbeat", handleHeartbeat)
	mux.HandleFunc("GET /api/media/stats", handleStats)
	mux.HandleFunc("POST /api/media/refresh", handleRefresh)
	mux.HandleFunc("GET /api/media/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	addr := env("LISTEN", "127.0.0.1:8081")
	log.Printf("media-api 监听 %s,媒体根目录 %s", addr, mediaRoot)
	srv := &http.Server{
		Addr: addr, Handler: mux,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
