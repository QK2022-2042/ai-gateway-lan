package httpapi

import (
	"net/http"
	"strings"
	"time"

	"ai-gateway/internal/secure"
	"ai-gateway/internal/store"
)

type keyInput struct {
	UserID            int64    `json:"user_id"`
	Name              string   `json:"name"`
	Status            string   `json:"status"`
	AllowedModels     []string `json:"allowed_models"`
	RPMLimit          int      `json:"rpm_limit"`
	TPMLimit          int      `json:"tpm_limit"`
	DailyTokenLimit   int      `json:"daily_token_limit"`
	MonthlyTokenLimit int      `json:"monthly_token_limit"`
	DailyRequestLimit int      `json:"daily_request_limit"`
	ConcurrencyLimit  int      `json:"concurrency_limit"`
	ExpiresAt         *int64   `json:"expires_at"`
}

func (in *keyInput) validate() string {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return "请填写密钥名称"
	}
	if len(in.Name) > 80 {
		return "密钥名称过长"
	}
	if in.Status != "" && in.Status != "active" && in.Status != "disabled" {
		return "状态只能是 active 或 disabled"
	}
	if in.RPMLimit < 0 || in.TPMLimit < 0 || in.DailyTokenLimit < 0 ||
		in.MonthlyTokenLimit < 0 || in.DailyRequestLimit < 0 || in.ConcurrencyLimit < 0 {
		return "限额不能为负数"
	}
	for _, m := range in.AllowedModels {
		if strings.TrimSpace(m) == "" {
			return "模型权限中包含空值"
		}
	}
	return ""
}

func (s *Server) canManageKey(u *store.User, k *store.APIKey) bool {
	return u.Role == "admin" || u.ID == k.UserID
}

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request, u *store.User) {
	var uid *int64
	if u.Role == "admin" {
		uid = queryInt64(r, "user_id")
	} else {
		uid = &u.ID
	}
	keys, err := s.Store.ListKeys(uid, r.URL.Query().Get("q"))
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "查询密钥失败")
		return
	}
	usage, _ := s.Store.KeyUsageByDay(time.Now().Format("2006-01-02"))
	type row struct {
		store.APIKey
		UsageToday *store.KeyUsage `json:"usage_today,omitempty"`
	}
	out := make([]row, 0, len(keys))
	for _, k := range keys {
		item := row{APIKey: k}
		if us, ok := usage[k.ID]; ok {
			item.UsageToday = &us
		}
		out = append(out, item)
	}
	jsonOK(w, map[string]any{"items": out})
}

func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request, u *store.User) {
	var in keyInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		jsonErr(w, http.StatusBadRequest, msg)
		return
	}
	targetID := u.ID
	if u.Role == "admin" && in.UserID > 0 {
		targetID = in.UserID
	}
	target, err := s.Store.UserByID(targetID)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "目标用户不存在")
		return
	}
	if target.Status != "active" {
		jsonErr(w, http.StatusBadRequest, "目标用户已被禁用")
		return
	}
	plain, hash, prefix := secure.NewAPIKey()
	status := in.Status
	if status == "" {
		status = "active"
	}
	k := &store.APIKey{
		UserID:            targetID,
		Name:              in.Name,
		KeyHash:           hash,
		KeyPrefix:         prefix,
		Status:            status,
		AllowedModels:     in.AllowedModels,
		RPMLimit:          in.RPMLimit,
		TPMLimit:          in.TPMLimit,
		DailyTokenLimit:   in.DailyTokenLimit,
		MonthlyTokenLimit: in.MonthlyTokenLimit,
		DailyRequestLimit: in.DailyRequestLimit,
		ConcurrencyLimit:  in.ConcurrencyLimit,
		ExpiresAt:         in.ExpiresAt,
	}
	if k.AllowedModels == nil {
		k.AllowedModels = []string{}
	}
	if err := s.Store.CreateKey(k); err != nil {
		jsonErr(w, http.StatusInternalServerError, "创建密钥失败: "+err.Error())
		return
	}
	s.audit(r, u, "create_key", "key:"+itoa(k.ID), "user="+target.Username+" name="+k.Name)
	jsonOK(w, map[string]any{"key": k, "plaintext_key": plain})
}

func (s *Server) handleUpdateKey(w http.ResponseWriter, r *http.Request, u *store.User) {
	id, ok := pathID(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	k, err := s.Store.KeyByID(id)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "密钥不存在")
		return
	}
	if !s.canManageKey(u, k) {
		jsonErr(w, http.StatusForbidden, "无权操作该密钥")
		return
	}
	var in keyInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Name == "" {
		in.Name = k.Name
	}
	if in.Status == "" {
		in.Status = k.Status
	}
	if msg := in.validate(); msg != "" {
		jsonErr(w, http.StatusBadRequest, msg)
		return
	}
	k.Name = in.Name
	k.Status = in.Status
	k.AllowedModels = in.AllowedModels
	if k.AllowedModels == nil {
		k.AllowedModels = []string{}
	}
	k.RPMLimit = in.RPMLimit
	k.TPMLimit = in.TPMLimit
	k.DailyTokenLimit = in.DailyTokenLimit
	k.MonthlyTokenLimit = in.MonthlyTokenLimit
	k.DailyRequestLimit = in.DailyRequestLimit
	k.ConcurrencyLimit = in.ConcurrencyLimit
	k.ExpiresAt = in.ExpiresAt
	if err := s.Store.UpdateKey(k); err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}
	s.audit(r, u, "update_key", "key:"+itoa(k.ID), k.Name)
	jsonOK(w, map[string]any{"key": k})
}

func (s *Server) handleDeleteKey(w http.ResponseWriter, r *http.Request, u *store.User) {
	id, ok := pathID(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	k, err := s.Store.KeyByID(id)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "密钥不存在")
		return
	}
	if !s.canManageKey(u, k) {
		jsonErr(w, http.StatusForbidden, "无权操作该密钥")
		return
	}
	if err := s.Store.DeleteKey(id); err != nil {
		jsonErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	s.audit(r, u, "delete_key", "key:"+itoa(id), k.Name)
	jsonOK(w, map[string]any{"ok": true})
}

func (s *Server) handleRotateKey(w http.ResponseWriter, r *http.Request, u *store.User) {
	id, ok := pathID(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	k, err := s.Store.KeyByID(id)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "密钥不存在")
		return
	}
	if !s.canManageKey(u, k) {
		jsonErr(w, http.StatusForbidden, "无权操作该密钥")
		return
	}
	plain, hash, prefix := secure.NewAPIKey()
	if err := s.Store.UpdateKeySecret(id, hash, prefix); err != nil {
		jsonErr(w, http.StatusInternalServerError, "轮换失败")
		return
	}
	s.audit(r, u, "rotate_key", "key:"+itoa(id), k.Name)
	jsonOK(w, map[string]any{"plaintext_key": plain, "key_prefix": prefix})
}
