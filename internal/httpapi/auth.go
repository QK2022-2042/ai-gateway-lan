package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-gateway/internal/secure"
	"ai-gateway/internal/store"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if ok, retry := s.LoginLimiter.Allowed(ip); !ok {
		w.Header().Set("Retry-After", intString(retry))
		jsonErr(w, http.StatusTooManyRequests, "登录尝试过于频繁，请稍后再试")
		return
	}
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		jsonErr(w, http.StatusBadRequest, "请输入用户名和密码")
		return
	}
	u, err := s.Store.UserByUsername(req.Username)
	if err != nil || !secure.VerifyPassword(req.Password, u.PasswordHash) {
		s.LoginLimiter.Fail(ip)
		_ = s.Store.AddAudit(nil, req.Username, "login_failed", "", "密码错误或用户不存在", ip)
		jsonErr(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if u.Status != "active" {
		s.LoginLimiter.Fail(ip)
		jsonErr(w, http.StatusForbidden, "该账号已被禁用")
		return
	}
	s.LoginLimiter.Reset(ip)
	s.issueSession(w, r, u)
	_ = s.Store.TouchLogin(u.ID)
	s.audit(r, u, "login", "", "")
	jsonOK(w, map[string]any{"user": u, "is_admin": u.Role == "admin"})
}

func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, u *store.User) {
	plain, hash := secure.NewSessionToken()
	expires := time.Now().Add(s.Cfg.SessionTTL)
	_ = s.Store.CreateSession(&store.Session{
		TokenHash: hash,
		UserID:    u.ID,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: expires.Unix(),
		IP:        s.clientIP(r),
		UserAgent: truncate(r.UserAgent(), 300),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "gw_session",
		Value:    plain,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		Expires:  expires,
		MaxAge:   int(s.Cfg.SessionTTL.Seconds()),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("gw_session"); err == nil && c.Value != "" {
		_ = s.Store.DeleteSession(secure.HashToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name: "gw_session", Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(0, 0),
	})
	jsonOK(w, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, u *store.User) {
	name, _ := s.Store.GetSetting("site_name")
	if name == "" {
		name = s.Cfg.SiteName
	}
	jsonOK(w, map[string]any{
		"user":      u,
		"is_admin":  u.Role == "admin",
		"site_name": name,
		"version":   s.Cfg.Version,
	})
}

type passwordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, u *store.User) {
	var req passwordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !secure.VerifyPassword(req.OldPassword, u.PasswordHash) {
		jsonErr(w, http.StatusBadRequest, "原密码不正确")
		return
	}
	hash, err := secure.HashPassword(req.NewPassword)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.UpdatePassword(u.ID, hash); err != nil {
		jsonErr(w, http.StatusInternalServerError, "修改密码失败")
		return
	}
	_ = s.Store.DeleteSessionsForUser(u.ID)
	http.SetCookie(w, &http.Cookie{
		Name: "gw_session", Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(0, 0),
	})
	s.audit(r, u, "change_password", "self", "")
	jsonOK(w, map[string]any{"ok": true, "relogin": true})
}

func intString(n int) string {
	if n <= 0 {
		n = 1
	}
	return strconv.Itoa(n)
}
