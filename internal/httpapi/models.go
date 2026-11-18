package httpapi

import (
	"net/http"
	"regexp"
	"strings"

	"ai-gateway/internal/store"
)

var modelAliasRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,119}$`)

type modelInput struct {
	Alias       string  `json:"alias"`
	DisplayName string  `json:"display_name"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
	Enabled     *bool   `json:"enabled"`
	Note        string  `json:"note"`
}

// handleMyModels lists enabled model aliases visible to the signed-in user.
func (s *Server) handleMyModels(w http.ResponseWriter, r *http.Request, u *store.User) {
	aliases, err := s.Store.ListAliases()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "查询模型失败")
		return
	}
	routes, _ := s.Store.AllRoutes()
	counts := map[string]int{}
	for _, rt := range routes {
		counts[rt.Alias]++
	}
	type item struct {
		Alias       string  `json:"alias"`
		DisplayName string  `json:"display_name"`
		InputPrice  float64 `json:"input_price"`
		OutputPrice float64 `json:"output_price"`
		RouteCount  int     `json:"route_count"`
	}
	out := []item{}
	for _, a := range aliases {
		if !a.Enabled || !userModelAllowed(u, a.Alias) {
			continue
		}
		out = append(out, item{
			Alias: a.Alias, DisplayName: a.DisplayName,
			InputPrice: a.InputPrice, OutputPrice: a.OutputPrice,
			RouteCount: counts[a.Alias],
		})
	}
	jsonOK(w, map[string]any{"items": out})
}

func userModelAllowed(u *store.User, model string) bool {
	if len(u.AllowedModels) == 0 {
		return true
	}
	for _, v := range u.AllowedModels {
		v = strings.TrimSpace(v)
		if v == "*" || v == model {
			return true
		}
		if strings.HasSuffix(v, "*") && strings.HasPrefix(model, strings.TrimSuffix(v, "*")) {
			return true
		}
	}
	return false
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request, u *store.User) {
	aliases, err := s.Store.ListAliases()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "查询模型失败")
		return
	}
	routes, err := s.Store.AllRoutes()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "查询路由失败")
		return
	}
	byAlias := map[string][]store.ModelRoute{}
	for _, rt := range routes {
		byAlias[rt.Alias] = append(byAlias[rt.Alias], rt)
	}
	type item struct {
		store.ModelAlias
		Routes []store.ModelRoute `json:"routes"`
	}
	out := make([]item, 0, len(aliases))
	for _, a := range aliases {
		rs := byAlias[a.Alias]
		if rs == nil {
			rs = []store.ModelRoute{}
		}
		out = append(out, item{ModelAlias: a, Routes: rs})
	}
	jsonOK(w, map[string]any{"items": out})
}

func (s *Server) handleSaveModel(w http.ResponseWriter, r *http.Request, u *store.User) {
	pathAlias := strings.TrimSpace(r.PathValue("alias"))
	isCreate := r.Method == http.MethodPost
	var in modelInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if isCreate {
		in.Alias = strings.TrimSpace(in.Alias)
		if !modelAliasRE.MatchString(in.Alias) {
			jsonErr(w, http.StatusBadRequest, "模型标识需为 1–120 位字母、数字或 . _ : / @ -")
			return
		}
		if _, err := s.Store.GetAlias(in.Alias); err == nil {
			jsonErr(w, http.StatusConflict, "该模型标识已存在")
			return
		}
	} else {
		if pathAlias == "" {
			jsonErr(w, http.StatusBadRequest, "参数错误")
			return
		}
		existing, err := s.Store.GetAlias(pathAlias)
		if err != nil {
			jsonErr(w, http.StatusNotFound, "模型不存在")
			return
		}
		in.Alias = pathAlias
		if in.Enabled == nil {
			v := existing.Enabled
			in.Enabled = &v
		}
	}
	if in.InputPrice < 0 || in.OutputPrice < 0 {
		jsonErr(w, http.StatusBadRequest, "价格不能为负数")
		return
	}
	if in.InputPrice > 1e6 || in.OutputPrice > 1e6 {
		jsonErr(w, http.StatusBadRequest, "价格数值过大")
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	m := &store.ModelAlias{
		Alias:       in.Alias,
		DisplayName: strings.TrimSpace(in.DisplayName),
		InputPrice:  in.InputPrice,
		OutputPrice: in.OutputPrice,
		Enabled:     enabled,
		Note:        strings.TrimSpace(in.Note),
	}
	if err := s.Store.UpsertAlias(m); err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}
	action := "update_model"
	if isCreate {
		action = "create_model"
	}
	s.audit(r, u, action, "model:"+m.Alias, m.DisplayName)
	jsonOK(w, map[string]any{"model": m})
}

func (s *Server) handleDeleteModel(w http.ResponseWriter, r *http.Request, u *store.User) {
	alias := strings.TrimSpace(r.PathValue("alias"))
	if alias == "" {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	if err := s.Store.DeleteAlias(alias); err != nil {
		jsonErr(w, http.StatusNotFound, "模型不存在")
		return
	}
	s.audit(r, u, "delete_model", "model:"+alias, "")
	jsonOK(w, map[string]any{"ok": true})
}

type routeInput struct {
	Alias         string `json:"alias"`
	ProviderID    int64  `json:"provider_id"`
	UpstreamModel string `json:"upstream_model"`
	Priority      int    `json:"priority"`
	Weight        int    `json:"weight"`
	Enabled       *bool  `json:"enabled"`
}

func (in *routeInput) validate() string {
	in.UpstreamModel = strings.TrimSpace(in.UpstreamModel)
	if in.ProviderID <= 0 {
		return "请选择供应商"
	}
	if in.UpstreamModel == "" {
		return "请填写上游模型名称"
	}
	if len(in.UpstreamModel) > 200 {
		return "上游模型名称过长"
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
	return ""
}

func (s *Server) handleCreateRoute(w http.ResponseWriter, r *http.Request, u *store.User) {
	var in routeInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		jsonErr(w, http.StatusBadRequest, msg)
		return
	}
	if _, err := s.Store.GetAlias(in.Alias); err != nil {
		jsonErr(w, http.StatusBadRequest, "模型别名不存在")
		return
	}
	if _, err := s.Store.GetProvider(in.ProviderID); err != nil {
		jsonErr(w, http.StatusBadRequest, "供应商不存在")
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	rt := &store.ModelRoute{
		Alias:         in.Alias,
		ProviderID:    in.ProviderID,
		UpstreamModel: in.UpstreamModel,
		Priority:      in.Priority,
		Weight:        in.Weight,
		Enabled:       enabled,
	}
	if err := s.Store.CreateRoute(rt); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonErr(w, http.StatusConflict, "该供应商与上游模型的组合已存在")
			return
		}
		jsonErr(w, http.StatusInternalServerError, "创建路由失败: "+err.Error())
		return
	}
	s.audit(r, u, "create_route", "route:"+itoa(rt.ID), in.Alias+" -> "+in.UpstreamModel)
	jsonOK(w, map[string]any{"route": rt})
}

func (s *Server) handleUpdateRoute(w http.ResponseWriter, r *http.Request, u *store.User) {
	id, ok := pathID(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	rt, err := s.Store.GetRoute(id)
	if err != nil {
		jsonErr(w, http.StatusNotFound, "路由不存在")
		return
	}
	var in routeInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		jsonErr(w, http.StatusBadRequest, msg)
		return
	}
	if _, err := s.Store.GetProvider(in.ProviderID); err != nil {
		jsonErr(w, http.StatusBadRequest, "供应商不存在")
		return
	}
	enabled := rt.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	rt.ProviderID = in.ProviderID
	rt.UpstreamModel = in.UpstreamModel
	rt.Priority = in.Priority
	rt.Weight = in.Weight
	rt.Enabled = enabled
	if err := s.Store.UpdateRoute(rt); err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}
	s.audit(r, u, "update_route", "route:"+itoa(id), rt.Alias+" -> "+rt.UpstreamModel)
	jsonOK(w, map[string]any{"route": rt})
}

func (s *Server) handleDeleteRoute(w http.ResponseWriter, r *http.Request, u *store.User) {
	id, ok := pathID(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	if err := s.Store.DeleteRoute(id); err != nil {
		jsonErr(w, http.StatusNotFound, "路由不存在")
		return
	}
	s.audit(r, u, "delete_route", "route:"+itoa(id), "")
	jsonOK(w, map[string]any{"ok": true})
}
