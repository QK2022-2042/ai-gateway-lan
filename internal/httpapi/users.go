package httpapi

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"ai-gateway/internal/secure"
	"ai-gateway/internal/store"
)

var usernameRE = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,64}$`)

type userInput struct {
	Username          string   `json:"username"`
	Password          string   `json:"password"`
	DisplayName       string   `json:"display_name"`
	Role              string   `json:"role"`
	Status            string   `json:"status"`
	Note              string   `json:"note"`
	AllowedModels     []string `json:"allowed_models"`
	RPMLimit          int      `json:"rpm_limit"`
	TPMLimit          int      `json:"tpm_limit"`
	DailyRequestLimit int      `json:"daily_request_limit"`
	DailyTokenLimit   int      `json:"daily_token_limit"`
	MonthlyTokenLimit int      `json:"monthly_token_limit"`
	ConcurrencyLimit  int      `json:"concurrency_limit"`
}

func (in *userInput) validateLimits() string {
	if in.RPMLimit < 0 || in.TPMLimit < 0 || in.DailyRequestLimit < 0 ||
		in.DailyTokenLimit < 0 || in.MonthlyTokenLimit < 0 || in.ConcurrencyLimit < 0 {
		return "限额不能为负数"
	}
	return ""
}

func cleanModels(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range in {
		m = strings.TrimSpace(m)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request, u *store.User) {
	users, err := s.Store.ListUsers(r.URL.Query().Get("q"))
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "查询用户失败")
		return
	}
	usage, _ := s.Store.UsageByUser(time.Now().Format("2006-01-02"))
	type row struct {
		store.User
		UsageToday *store.UserUsageRow `json:"usage_today,omitempty"`
	}
	out := make([]row, 0, len(users))
	for _, item := range users {
		rw := row{User: item}
		if us, ok := usage[item.ID]; ok {
			rw.UsageToday = &us
		}
		out = append(out, rw)
	}
	jsonOK(w, map[string]any{"items": out})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request, admin *store.User) {
	var in userInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if !usernameRE.MatchString(in.Username) {
		jsonErr(w, http.StatusBadRequest, "用户名需为 3–64 位字母、数字或 _ . -")
		return
	}
	if in.Role == "" {
		in.Role = "user"
	}
	if in.Role != "admin" && in.Role != "user" {
		jsonErr(w, http.StatusBadRequest, "角色只能是 admin 或 user")
		return
	}
	if in.Status == "" {
		in.Status = "active"
	}
	if in.Status != "active" && in.Status != "disabled" {
		jsonErr(w, http.StatusBadRequest, "状态只能是 active 或 disabled")
		return
	}
	if msg := in.validateLimits(); msg != "" {
		jsonErr(w, http.StatusBadRequest, msg)
		return
	}
	hash, err := secure.HashPassword(in.Password)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	models := cleanModels(in.AllowedModels)
	if len(models) == 0 {
		models = []string{"*"}
	}
	user := &store.User{
		Username:          in.Username,
		PasswordHash:      hash,
		DisplayName:       in.DisplayName,
		Role:              in.Role,
		Status:            in.Status,
		Note:              strings.TrimSpace(in.Note),
		AllowedModels:     models,
		RPMLimit:          in.RPMLimit,
		TPMLimit:          in.TPMLimit,
		DailyRequestLimit: in.DailyRequestLimit,
		DailyTokenLimit:   in.DailyTokenLimit,
		MonthlyTokenLimit: in.MonthlyTokenLimit,
		ConcurrencyLimit:  in.ConcurrencyLimit,
	}
	if err := s.Store.CreateUser(user); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonErr(w, http.StatusConflict, "用户名已存在")
			return
		}
		jsonErr(w, http.StatusInternalServerError, "创建用户失败: "+err.Error())
		return
	}
	s.audit(r, admin, "create_user", "user:"+itoa(user.ID), user.Username)
	jsonOK(w, map[string]any{"user": user})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request, admin *store.User) {
	id, ok := pathID(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	user, err := s.Store.UserByID(id)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "用户不存在")
		return
	}
	var in userInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Role == "" {
		in.Role = user.Role
	}
	if in.Status == "" {
		in.Status = user.Status
	}
	if in.Role != "admin" && in.Role != "user" {
		jsonErr(w, http.StatusBadRequest, "角色只能是 admin 或 user")
		return
	}
	if in.Status != "active" && in.Status != "disabled" {
		jsonErr(w, http.StatusBadRequest, "状态只能是 active 或 disabled")
		return
	}
	if msg := in.validateLimits(); msg != "" {
		jsonErr(w, http.StatusBadRequest, msg)
		return
	}
	losingAdmin := user.Role == "admin" && (in.Role != "admin" || in.Status != "active")
	if losingAdmin {
		admins, _ := s.Store.CountAdmins()
		if admins <= 1 {
			jsonErr(w, http.StatusBadRequest, "系统必须保留至少一个可用的管理员")
			return
		}
	}
	if id == admin.ID && (in.Role != "admin" || in.Status != "active") {
		jsonErr(w, http.StatusBadRequest, "不能降级或禁用当前登录的管理员账号")
		return
	}
	user.DisplayName = strings.TrimSpace(in.DisplayName)
	user.Role = in.Role
	user.Status = in.Status
	user.Note = strings.TrimSpace(in.Note)
	user.AllowedModels = cleanModels(in.AllowedModels)
	if len(user.AllowedModels) == 0 {
		user.AllowedModels = []string{"*"}
	}
	user.RPMLimit = in.RPMLimit
	user.TPMLimit = in.TPMLimit
	user.DailyRequestLimit = in.DailyRequestLimit
	user.DailyTokenLimit = in.DailyTokenLimit
	user.MonthlyTokenLimit = in.MonthlyTokenLimit
	user.ConcurrencyLimit = in.ConcurrencyLimit
	if err := s.Store.UpdateUser(user); err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}
	if in.Status == "disabled" {
		_ = s.Store.DeleteSessionsForUser(id)
	}
	s.audit(r, admin, "update_user", "user:"+itoa(id), user.Username+" role="+user.Role+" status="+user.Status)
	jsonOK(w, map[string]any{"user": user})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request, admin *store.User) {
	id, ok := pathID(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	if id == admin.ID {
		jsonErr(w, http.StatusBadRequest, "不能删除当前登录的账号")
		return
	}
	user, err := s.Store.UserByID(id)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "用户不存在")
		return
	}
	if user.Role == "admin" {
		admins, _ := s.Store.CountAdmins()
		if admins <= 1 {
			jsonErr(w, http.StatusBadRequest, "系统必须保留至少一个可用的管理员")
			return
		}
	}
	if err := s.Store.DeleteUser(id); err != nil {
		jsonErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	s.audit(r, admin, "delete_user", "user:"+itoa(id), user.Username)
	jsonOK(w, map[string]any{"ok": true})
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request, admin *store.User) {
	id, ok := pathID(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	user, err := s.Store.UserByID(id)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "用户不存在")
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	hash, err := secure.HashPassword(in.Password)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.UpdatePassword(id, hash); err != nil {
		jsonErr(w, http.StatusInternalServerError, "重置密码失败")
		return
	}
	_ = s.Store.DeleteSessionsForUser(id)
	s.audit(r, admin, "reset_password", "user:"+itoa(id), user.Username)
	jsonOK(w, map[string]any{"ok": true})
}
