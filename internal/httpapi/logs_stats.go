package httpapi

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-gateway/internal/store"
)

func (s *Server) buildLogFilter(r *http.Request, u *store.User) store.LogFilter {
	q := r.URL.Query()
	f := store.LogFilter{
		From:    parseInt(q.Get("from")),
		To:      parseInt(q.Get("to")),
		Model:   strings.TrimSpace(q.Get("model")),
		Status:  strings.TrimSpace(q.Get("status")),
		Keyword: strings.TrimSpace(q.Get("keyword")),
	}
	if u.Role != "admin" {
		f.UserID = &u.ID
	} else {
		f.UserID = queryInt64(r, "user_id")
	}
	if u.Role == "admin" {
		f.KeyID = queryInt64(r, "key_id")
	}
	return f
}

func parseInt(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func (s *Server) handleListLogs(w http.ResponseWriter, r *http.Request, u *store.User) {
	f := s.buildLogFilter(r, u)
	page, size := pageParams(r)
	f.Limit = size
	f.Offset = (page - 1) * size
	items, total, err := s.Store.QueryLogs(&f)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "查询日志失败")
		return
	}
	jsonOK(w, map[string]any{"items": items, "total": total, "page": page, "page_size": size})
}

func (s *Server) handleExportLogs(w http.ResponseWriter, r *http.Request, u *store.User) {
	f := s.buildLogFilter(r, u)
	f.Limit = 500
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=logs-%s.csv", time.Now().Format("20060102-150405")))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM for Excel
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"时间", "请求ID", "用户", "密钥", "模型", "上游模型", "供应商",
		"状态码", "错误", "输入Tokens", "输出Tokens", "总Tokens", "费用(元)",
		"延迟(ms)", "首字(ms)", "流式", "重试", "客户端IP"})
	written := 0
	for written < 20000 {
		f.Offset = written
		items, total, err := s.Store.QueryLogs(&f)
		if err != nil {
			break
		}
		for _, l := range items {
			_ = cw.Write([]string{
				time.Unix(l.Ts, 0).Format("2006-01-02 15:04:05"),
				l.RequestID, l.Username, l.KeyName, l.Model, l.UpstreamModel, l.ProviderName,
				strconv.Itoa(l.Status), l.Error,
				strconv.FormatInt(l.PromptTokens, 10), strconv.FormatInt(l.CompletionTokens, 10),
				strconv.FormatInt(l.TotalTokens, 10), fmt.Sprintf("%.4f", l.Cost),
				strconv.FormatInt(l.LatencyMs, 10), strconv.FormatInt(l.FirstTokenMs, 10),
				boolText(l.Stream), strconv.Itoa(l.Retries), l.ClientIP,
			})
			written++
		}
		if len(items) == 0 || written >= total {
			break
		}
	}
	cw.Flush()
}

func boolText(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

type overviewResponse struct {
	Range        map[string]any       `json:"range"`
	Current      store.Overview       `json:"current"`
	Previous     store.Overview       `json:"previous"`
	Series       []store.Point        `json:"series"`
	BucketSec    int64                `json:"bucket_sec"`
	ByModel      []store.BreakdownRow `json:"by_model"`
	ByUser       []store.BreakdownRow `json:"by_user"`
	ByProvider   []store.BreakdownRow `json:"by_provider"`
	RecentErrors []store.RequestLog   `json:"recent_errors"`
	Providers    []map[string]any     `json:"providers"`
	Models       []store.ModelAlias   `json:"models"`
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request, u *store.User) {
	rangeKey := r.URL.Query().Get("range")
	var dur time.Duration
	bucket := int64(3600)
	switch rangeKey {
	case "7d":
		dur = 7 * 24 * time.Hour
		bucket = 6 * 3600
	case "30d":
		dur = 30 * 24 * time.Hour
		bucket = 86400
	case "24h", "":
		dur = 24 * time.Hour
		rangeKey = "24h"
	default:
		jsonErr(w, http.StatusBadRequest, "range 仅支持 24h、7d、30d")
		return
	}
	now := time.Now()
	to := now.Unix()
	from := now.Add(-dur).Unix()
	prevFrom := from - int64(dur.Seconds())

	cur, err := s.Store.Overview(from, to)
	if err != nil {
		s.Logger.Error("stats overview query failed", "err", err)
		jsonErr(w, http.StatusInternalServerError, "统计查询失败")
		return
	}
	prev, _ := s.Store.Overview(prevFrom, from)
	series, _ := s.Store.Timeseries(from, to, bucket)
	byModel, _ := s.Store.BreakdownByModel(from, to, 8)
	byUser, _ := s.Store.BreakdownByUser(from, to, 8)
	byProvider, _ := s.Store.BreakdownByProvider(from, to, 8)
	errs, _ := s.Store.RecentErrors(8)
	providers, _ := s.Store.ListProviders()
	pout := make([]map[string]any, 0, len(providers))
	for _, p := range providers {
		pout = append(pout, map[string]any{
			"id": p.ID, "name": p.Name, "status": p.Status, "health": p.Health,
			"health_msg": p.HealthMsg, "checked_at": p.CheckedAt, "base_url": p.BaseURL,
		})
	}
	models, _ := s.Store.ListAliases()
	if errs == nil {
		errs = []store.RequestLog{}
	}
	jsonOK(w, overviewResponse{
		Range: map[string]any{
			"key": rangeKey, "from": from, "to": to,
		},
		Current:      cur,
		Previous:     prev,
		Series:       series,
		BucketSec:    bucket,
		ByModel:      byModel,
		ByUser:       byUser,
		ByProvider:   byProvider,
		RecentErrors: errs,
		Providers:    pout,
		Models:       models,
	})
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request, u *store.User) {
	page, size := pageParams(r)
	items, total, err := s.Store.ListAudit(size, (page-1)*size)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "查询审计日志失败")
		return
	}
	jsonOK(w, map[string]any{"items": items, "total": total, "page": page, "page_size": size})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request, u *store.User) {
	all, err := s.Store.AllSettings()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "读取设置失败")
		return
	}
	retention := 90
	if v := all["log_retention_days"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retention = n
		}
	}
	name := all["site_name"]
	if name == "" {
		name = s.Cfg.SiteName
	}
	jsonOK(w, map[string]any{
		"site_name":          name,
		"log_retention_days": retention,
		"trust_proxy":        s.Cfg.TrustProxy,
		"version":            s.Cfg.Version,
		"data_dir":           s.Cfg.DataDir,
	})
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request, u *store.User) {
	var in struct {
		SiteName         string `json:"site_name"`
		LogRetentionDays int    `json:"log_retention_days"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	in.SiteName = strings.TrimSpace(in.SiteName)
	if in.SiteName == "" || len(in.SiteName) > 60 {
		jsonErr(w, http.StatusBadRequest, "站点名称需为 1–60 个字符")
		return
	}
	if in.LogRetentionDays < 1 || in.LogRetentionDays > 3650 {
		jsonErr(w, http.StatusBadRequest, "日志保留天数需在 1–3650 之间")
		return
	}
	if err := s.Store.SetSetting("site_name", in.SiteName); err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存失败")
		return
	}
	if err := s.Store.SetSetting("log_retention_days", strconv.Itoa(in.LogRetentionDays)); err != nil {
		jsonErr(w, http.StatusInternalServerError, "保存失败")
		return
	}
	s.audit(r, u, "update_settings", "settings", in.SiteName)
	jsonOK(w, map[string]any{"ok": true})
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request, u *store.User) {
	users, _ := s.Store.CountUsers()
	keys, _ := s.Store.ListKeys(nil, "")
	providers, _ := s.Store.ListProviders()
	models, _ := s.Store.ListAliases()
	logs, _ := s.Store.CountLogs()
	jsonOK(w, map[string]any{
		"version":    s.Cfg.Version,
		"uptime_sec": int64(time.Since(s.StartedAt).Seconds()),
		"started_at": s.StartedAt.Unix(),
		"data_dir":   s.Cfg.DataDir,
		"users":      users,
		"keys":       len(keys),
		"providers":  len(providers),
		"models":     len(models),
		"logs":       logs,
	})
}
