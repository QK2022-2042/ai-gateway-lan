package httpapi

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-gateway/internal/gateway"
	"ai-gateway/internal/limiter"
	"ai-gateway/internal/secure"
	"ai-gateway/internal/store"
)

//go:embed web
var webFiles embed.FS

type Config struct {
	SiteName   string
	Version    string
	DataDir    string
	TrustProxy bool
	SessionTTL time.Duration
}

type Server struct {
	Store        *store.Store
	Vault        *secure.Vault
	GW           *gateway.Gateway
	Limiter      *limiter.Limiter
	LoginLimiter *limiter.LoginLimiter
	Cfg          Config
	Logger       *slog.Logger
	StartedAt    time.Time
	mux          *http.ServeMux
}

func New(st *store.Store, vault *secure.Vault, gw *gateway.Gateway, lim *limiter.Limiter,
	cfg Config, logger *slog.Logger) *Server {
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 7 * 24 * time.Hour
	}
	if cfg.SiteName == "" {
		cfg.SiteName = "AI 模型中转站"
	}
	s := &Server{
		Store:        st,
		Vault:        vault,
		GW:           gw,
		Limiter:      lim,
		LoginLimiter: limiter.NewLoginLimiter(10, 5*time.Minute),
		Cfg:          cfg,
		Logger:       logger,
		StartedAt:    time.Now(),
		mux:          http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	m := s.mux

	m.HandleFunc("GET /healthz", s.handleHealth)
	m.HandleFunc("GET /api/meta", s.handleMeta)

	m.HandleFunc("POST /api/auth/login", s.handleLogin)
	m.HandleFunc("POST /api/auth/logout", s.handleLogout)
	m.HandleFunc("GET /api/auth/me", s.withUser(s.handleMe))
	m.HandleFunc("POST /api/auth/password", s.withUser(s.handleChangePassword))

	// Personal (role-scoped) endpoints
	m.HandleFunc("GET /api/keys", s.withUser(s.handleListKeys))
	m.HandleFunc("POST /api/keys", s.withUser(s.handleCreateKey))
	m.HandleFunc("PATCH /api/keys/{id}", s.withUser(s.handleUpdateKey))
	m.HandleFunc("DELETE /api/keys/{id}", s.withUser(s.handleDeleteKey))
	m.HandleFunc("POST /api/keys/{id}/rotate", s.withUser(s.handleRotateKey))
	m.HandleFunc("GET /api/logs", s.withUser(s.handleListLogs))
	m.HandleFunc("GET /api/my/models", s.withUser(s.handleMyModels))

	// Admin: users
	m.HandleFunc("GET /api/admin/users", s.withAdmin(s.handleListUsers))
	m.HandleFunc("POST /api/admin/users", s.withAdmin(s.handleCreateUser))
	m.HandleFunc("PATCH /api/admin/users/{id}", s.withAdmin(s.handleUpdateUser))
	m.HandleFunc("DELETE /api/admin/users/{id}", s.withAdmin(s.handleDeleteUser))
	m.HandleFunc("POST /api/admin/users/{id}/password", s.withAdmin(s.handleResetPassword))

	// Admin: providers
	m.HandleFunc("GET /api/admin/providers", s.withAdmin(s.handleListProviders))
	m.HandleFunc("POST /api/admin/providers", s.withAdmin(s.handleCreateProvider))
	m.HandleFunc("PATCH /api/admin/providers/{id}", s.withAdmin(s.handleUpdateProvider))
	m.HandleFunc("DELETE /api/admin/providers/{id}", s.withAdmin(s.handleDeleteProvider))
	m.HandleFunc("POST /api/admin/providers/{id}/test", s.withAdmin(s.handleTestProvider))
	m.HandleFunc("POST /api/admin/providers/{id}/sync-models", s.withAdmin(s.handleSyncProviderModels))

	// Admin: model aliases and routes
	m.HandleFunc("GET /api/admin/models", s.withAdmin(s.handleListModels))
	m.HandleFunc("POST /api/admin/models", s.withAdmin(s.handleSaveModel))
	m.HandleFunc("PATCH /api/admin/models/{alias}", s.withAdmin(s.handleSaveModel))
	m.HandleFunc("DELETE /api/admin/models/{alias}", s.withAdmin(s.handleDeleteModel))
	m.HandleFunc("POST /api/admin/routes", s.withAdmin(s.handleCreateRoute))
	m.HandleFunc("PATCH /api/admin/routes/{id}", s.withAdmin(s.handleUpdateRoute))
	m.HandleFunc("DELETE /api/admin/routes/{id}", s.withAdmin(s.handleDeleteRoute))

	// Admin: stats / audit / settings
	m.HandleFunc("GET /api/admin/stats/overview", s.withAdmin(s.handleOverview))
	m.HandleFunc("GET /api/admin/logs/export", s.withAdmin(s.handleExportLogs))
	m.HandleFunc("GET /api/admin/audit", s.withAdmin(s.handleListAudit))
	m.HandleFunc("GET /api/admin/settings", s.withAdmin(s.handleGetSettings))
	m.HandleFunc("PATCH /api/admin/settings", s.withAdmin(s.handleSaveSettings))
	m.HandleFunc("GET /api/admin/system", s.withAdmin(s.handleSystemInfo))

	// Inference endpoints (OpenAI-compatible)
	m.HandleFunc("OPTIONS /v1/{path...}", s.handlePreflight)
	m.HandleFunc("GET /v1/models", s.withAPIKey(s.handleModels))
	m.HandleFunc("POST /v1/chat/completions", s.withAPIKey(s.handleChat))
	m.HandleFunc("POST /v1/completions", s.withAPIKey(s.handleCompletions))
	m.HandleFunc("POST /v1/embeddings", s.withAPIKey(s.handleEmbeddings))

	// Static admin UI
	sub, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	m.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/v1/") {
			http.NotFound(w, r)
			return
		}
		// The UI is embedded and tiny; never let a stale JS/CSS bundle survive
		// an upgrade.
		w.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) Handler() http.Handler {
	return s.recoverMW(s.securityMW(s.corsMW(s.logMW(s.mux))))
}

func (s *Server) recoverMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.Logger.Error("panic serving request", "path", r.URL.Path, "panic", rec)
				jsonErr(w, http.StatusInternalServerError, "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; "+
				"script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) corsMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", "*")
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Api-Key, Anthropic-Version, Anthropic-Beta, OpenAI-Beta")
			h.Set("Access-Control-Max-Age", "86400")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API keys must never appear in logs. Log only method/path/status.
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/v1/") {
			s.Logger.Info("http", "method", r.Method, "path", r.URL.Path,
				"status", sw.status, "ms", time.Since(start).Milliseconds(), "ip", s.clientIP(r))
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{
		"status":  "ok",
		"version": s.Cfg.Version,
		"uptime":  int64(time.Since(s.StartedAt).Seconds()),
	})
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	name, _ := s.Store.GetSetting("site_name")
	if name == "" {
		name = s.Cfg.SiteName
	}
	jsonOK(w, map[string]any{
		"site_name": name,
		"version":   s.Cfg.Version,
	})
}

func (s *Server) handlePreflight(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// ---- authentication wrappers ----

func (s *Server) withUser(next func(http.ResponseWriter, *http.Request, *store.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.checkCSRF(w, r) {
			return
		}
		u, err := s.sessionUser(r)
		if err != nil {
			jsonErr(w, http.StatusUnauthorized, "登录状态已失效，请重新登录")
			return
		}
		next(w, r, u)
	}
}

func (s *Server) withAdmin(next func(http.ResponseWriter, *http.Request, *store.User)) http.HandlerFunc {
	return s.withUser(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		if u.Role != "admin" {
			jsonErr(w, http.StatusForbidden, "需要管理员权限")
			return
		}
		next(w, r, u)
	})
}

func (s *Server) withAPIKey(next func(http.ResponseWriter, *http.Request, *gateway.Identity)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, status, msg := s.apiIdentity(r)
		if id == nil {
			writeGatewayError(w, status, msg)
			return
		}
		next(w, r, id)
	}
}

func (s *Server) apiIdentity(r *http.Request) (*gateway.Identity, int, string) {
	token := secure.BearerToken(r.Header.Get("Authorization"))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get("X-Api-Key"))
	}
	if token == "" {
		return nil, http.StatusUnauthorized, "缺少 API Key。请在 Authorization: Bearer <key> 中提供"
	}
	key, err := s.Store.KeyByHash(secure.HashToken(token))
	if err != nil {
		return nil, http.StatusUnauthorized, "API Key 无效"
	}
	user, err := s.Store.UserByID(key.UserID)
	if err != nil {
		return nil, http.StatusUnauthorized, "API Key 所属用户不存在"
	}
	if user.Status != "active" {
		return nil, http.StatusForbidden, "该用户已被禁用"
	}
	return &gateway.Identity{
		Key:       key,
		User:      user,
		IP:        s.clientIP(r),
		UserAgent: truncate(r.UserAgent(), 300),
	}, 0, ""
}

func (s *Server) sessionUser(r *http.Request) (*store.User, error) {
	c, err := r.Cookie("gw_session")
	if err != nil || c.Value == "" {
		return nil, errors.New("no session")
	}
	hash := secure.HashToken(c.Value)
	sess, err := s.Store.SessionByHash(hash)
	if err != nil {
		return nil, err
	}
	if sess.ExpiresAt < time.Now().Unix() {
		_ = s.Store.DeleteSession(hash)
		return nil, errors.New("expired")
	}
	u, err := s.Store.UserByID(sess.UserID)
	if err != nil {
		return nil, err
	}
	if u.Status != "active" {
		return nil, errors.New("disabled")
	}
	return u, nil
}

func (s *Server) checkCSRF(w http.ResponseWriter, r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return true
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api/auth/login" {
		return true
	}
	if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
		jsonErr(w, http.StatusForbidden, "请求缺少防跨站校验头")
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		host := r.Host
		if !strings.HasSuffix(origin, "://"+host) {
			jsonErr(w, http.StatusForbidden, "跨站请求被拒绝")
			return false
		}
	}
	return true
}

func (s *Server) clientIP(r *http.Request) string {
	if s.Cfg.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i > 0 {
				xff = xff[:i]
			}
			if ip := strings.TrimSpace(xff); net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---- helpers ----

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

func writeGatewayError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"message": msg, "type": "invalid_request_error"},
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		jsonErr(w, http.StatusBadRequest, "请求数据格式错误: "+err.Error())
		return false
	}
	return true
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func pageParams(r *http.Request) (page, size int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
	if size < 1 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	return page, size
}

func queryInt64(r *http.Request, key string) *int64 {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func (s *Server) audit(r *http.Request, u *store.User, action, target, detail string) {
	var actorID *int64
	name := ""
	if u != nil {
		actorID = &u.ID
		name = u.Username
	}
	_ = s.Store.AddAudit(actorID, name, action, target, detail, s.clientIP(r))
}
