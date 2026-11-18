package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ai-gateway/internal/secure"
	"ai-gateway/internal/store"
)

type providerInput struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	BaseURL      string            `json:"base_url"`
	Keys         *[]string         `json:"keys"`
	ExtraHeaders map[string]string `json:"extra_headers"`
	TimeoutSec   int               `json:"timeout_sec"`
	Priority     int               `json:"priority"`
	Weight       int               `json:"weight"`
	Status       string            `json:"status"`
}

func (in *providerInput) validate() string {
	in.Name = strings.TrimSpace(in.Name)
	in.BaseURL = strings.TrimSpace(strings.TrimRight(in.BaseURL, "/"))
	if in.Name == "" {
		return "请填写供应商名称"
	}
	u, err := url.Parse(in.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "Base URL 必须是 http:// 或 https:// 开头的完整地址"
	}
	if in.Type == "" {
		in.Type = "openai"
	}
	if in.Type != "openai" {
		return "目前仅支持 OpenAI 兼容协议"
	}
	if in.TimeoutSec <= 0 {
		in.TimeoutSec = 120
	}
	if in.TimeoutSec > 3600 {
		return "超时时间不能超过 3600 秒"
	}
	if in.Priority < 0 || in.Priority > 1000 {
		return "优先级需在 0–1000 之间"
	}
	if in.Weight <= 0 {
		in.Weight = 1
	}
	if in.Weight > 100 {
		return "权重需在 1–100 之间"
	}
	if in.Status == "" {
		in.Status = "enabled"
	}
	if in.Status != "enabled" && in.Status != "disabled" {
		return "状态只能是 enabled 或 disabled"
	}
	return ""
}

func cleanKeys(keys []string) []string {
	out := []string{}
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}

func (s *Server) providerDTO(p store.Provider, routeCount int) map[string]any {
	keys, err := s.Vault.DecryptStrings(p.KeysEnc)
	masked := []string{}
	if err == nil {
		for _, k := range keys {
			masked = append(masked, secure.MaskSecret(k))
		}
	}
	headers := map[string]string{}
	_ = json.Unmarshal([]byte(p.ExtraHeaders), &headers)
	return map[string]any{
		"id":            p.ID,
		"name":          p.Name,
		"type":          p.Type,
		"base_url":      p.BaseURL,
		"timeout_sec":   p.TimeoutSec,
		"priority":      p.Priority,
		"weight":        p.Weight,
		"status":        p.Status,
		"health":        p.Health,
		"health_msg":    p.HealthMsg,
		"checked_at":    p.CheckedAt,
		"created_at":    p.CreatedAt,
		"updated_at":    p.UpdatedAt,
		"keys_count":    len(keys),
		"keys_masked":   masked,
		"extra_headers": headers,
		"route_count":   routeCount,
	}
}

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request, u *store.User) {
	providers, err := s.Store.ListProviders()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "查询供应商失败")
		return
	}
	routes, _ := s.Store.AllRoutes()
	counts := map[int64]int{}
	for _, rt := range routes {
		counts[rt.ProviderID]++
	}
	out := make([]map[string]any, 0, len(providers))
	for _, p := range providers {
		out = append(out, s.providerDTO(p, counts[p.ID]))
	}
	jsonOK(w, map[string]any{"items": out})
}

func (s *Server) handleCreateProvider(w http.ResponseWriter, r *http.Request, u *store.User) {
	var in providerInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		jsonErr(w, http.StatusBadRequest, msg)
		return
	}
	keys := []string{}
	if in.Keys != nil {
		keys = cleanKeys(*in.Keys)
	}
	enc, err := s.Vault.EncryptStrings(keys)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "加密密钥失败")
		return
	}
	headers := map[string]string{}
	for k, v := range in.ExtraHeaders {
		if strings.TrimSpace(k) != "" {
			headers[k] = v
		}
	}
	hj, _ := json.Marshal(headers)
	p := &store.Provider{
		Name:         in.Name,
		Type:         in.Type,
		BaseURL:      in.BaseURL,
		KeysEnc:      enc,
		ExtraHeaders: string(hj),
		TimeoutSec:   in.TimeoutSec,
		Priority:     in.Priority,
		Weight:       in.Weight,
		Status:       in.Status,
	}
	if err := s.Store.CreateProvider(p); err != nil {
		jsonErr(w, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	s.audit(r, u, "create_provider", "provider:"+itoa(p.ID), p.Name+" "+p.BaseURL)
	jsonOK(w, map[string]any{"provider": s.providerDTO(*p, 0)})
}

func (s *Server) handleUpdateProvider(w http.ResponseWriter, r *http.Request, u *store.User) {
	id, ok := pathID(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	p, err := s.Store.GetProvider(id)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "供应商不存在")
		return
	}
	var in providerInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		jsonErr(w, http.StatusBadRequest, msg)
		return
	}
	p.Name = in.Name
	p.Type = in.Type
	p.BaseURL = in.BaseURL
	p.TimeoutSec = in.TimeoutSec
	p.Priority = in.Priority
	p.Weight = in.Weight
	p.Status = in.Status
	if in.Keys != nil {
		enc, err := s.Vault.EncryptStrings(cleanKeys(*in.Keys))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "加密密钥失败")
			return
		}
		p.KeysEnc = enc
	}
	if in.ExtraHeaders != nil {
		headers := map[string]string{}
		for k, v := range in.ExtraHeaders {
			if strings.TrimSpace(k) != "" {
				headers[k] = v
			}
		}
		hj, _ := json.Marshal(headers)
		p.ExtraHeaders = string(hj)
	}
	if err := s.Store.UpdateProvider(p); err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}
	s.audit(r, u, "update_provider", "provider:"+itoa(id), p.Name+" status="+p.Status)
	jsonOK(w, map[string]any{"provider": s.providerDTO(*p, 0)})
}

func (s *Server) handleDeleteProvider(w http.ResponseWriter, r *http.Request, u *store.User) {
	id, ok := pathID(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	p, err := s.Store.GetProvider(id)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "供应商不存在")
		return
	}
	if err := s.Store.DeleteProvider(id); err != nil {
		jsonErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	s.audit(r, u, "delete_provider", "provider:"+itoa(id), p.Name)
	jsonOK(w, map[string]any{"ok": true})
}

func (s *Server) handleTestProvider(w http.ResponseWriter, r *http.Request, u *store.User) {
	id, ok := pathID(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	p, err := s.Store.GetProvider(id)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "供应商不存在")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	check := s.GW.CheckProvider(ctx, p)
	health := "error"
	if check.OK {
		health = "ok"
	}
	msg := check.Message
	if !check.OK {
		msg = fmt.Sprintf("%s", check.Message)
	}
	_ = s.Store.SetProviderHealth(id, health, truncate(msg, 500))
	s.audit(r, u, "test_provider", "provider:"+itoa(id), health+": "+msg)
	jsonOK(w, map[string]any{"check": check, "health": health})
}

func (s *Server) handleSyncProviderModels(w http.ResponseWriter, r *http.Request, u *store.User) {
	id, ok := pathID(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	p, err := s.Store.GetProvider(id)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "供应商不存在")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	check := s.GW.CheckProvider(ctx, p)
	if !check.OK {
		_ = s.Store.SetProviderHealth(id, "error", truncate(check.Message, 500))
		jsonErr(w, http.StatusBadGateway, "拉取模型列表失败: "+check.Message)
		return
	}
	_ = s.Store.SetProviderHealth(id, "ok", truncate(check.Message, 500))
	routes, _ := s.Store.AllRoutes()
	configured := []string{}
	for _, rt := range routes {
		if rt.ProviderID == id {
			configured = append(configured, rt.UpstreamModel)
		}
	}
	s.audit(r, u, "sync_provider_models", "provider:"+itoa(id), fmt.Sprintf("%d models", len(check.ModelIDs)))
	jsonOK(w, map[string]any{"models": check.ModelIDs, "configured": configured, "check": check})
}
