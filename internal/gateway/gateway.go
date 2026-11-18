package gateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ai-gateway/internal/limiter"
	"ai-gateway/internal/secure"
	"ai-gateway/internal/store"
)

const (
	pathChat        = "/v1/chat/completions"
	pathCompletions = "/v1/completions"
	pathEmbeddings  = "/v1/embeddings"
	pathModels      = "/v1/models"

	maxRequestBytes  = 32 << 20
	maxResponseBytes = 64 << 20
	maxErrorBytes    = 64 << 10
)

type Gateway struct {
	Store   *store.Store
	Vault   *secure.Vault
	Limiter *limiter.Limiter
	Client  *http.Client
	Logger  *slog.Logger

	keyIdx sync.Map // providerID -> *atomic.Uint64
}

func New(st *store.Store, vault *secure.Vault, lim *limiter.Limiter, logger *slog.Logger) *Gateway {
	if logger == nil {
		logger = slog.Default()
	}
	transport := &http.Transport{
		Proxy:                 nil, // air-gapped: never route through an ambient proxy
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     false,
	}
	return &Gateway{
		Store:   st,
		Vault:   vault,
		Limiter: lim,
		Client:  &http.Client{Transport: transport},
		Logger:  logger,
	}
}

type proxyMode struct {
	path string
	kind string // chat | completions | embeddings
}

func (g *Gateway) HandleChat(w http.ResponseWriter, r *http.Request, id *Identity) {
	g.handleProxy(w, r, id, proxyMode{path: pathChat, kind: "chat"})
}

func (g *Gateway) HandleCompletions(w http.ResponseWriter, r *http.Request, id *Identity) {
	g.handleProxy(w, r, id, proxyMode{path: pathCompletions, kind: "completions"})
}

func (g *Gateway) HandleEmbeddings(w http.ResponseWriter, r *http.Request, id *Identity) {
	g.handleProxy(w, r, id, proxyMode{path: pathEmbeddings, kind: "embeddings"})
}

func (g *Gateway) handleProxy(w http.ResponseWriter, r *http.Request, id *Identity, mode proxyMode) {
	start := time.Now()
	requestID := newRequestID()
	w.Header().Set("X-Request-Id", requestID)

	body, err := readLimited(http.MaxBytesReader(w, r.Body, maxRequestBytes), maxRequestBytes)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "读取请求体失败: "+err.Error(), "invalid_request_error", "invalid_body")
		return
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "请求体不是合法 JSON: "+err.Error(), "invalid_request_error", "invalid_json")
		return
	}
	modelName, _ := payload["model"].(string)
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		writeOpenAIError(w, http.StatusBadRequest, "缺少 model 参数", "invalid_request_error", "missing_model")
		return
	}
	stream, _ := payload["stream"].(bool)
	if !mode.streamAllowed() {
		stream = false
	}

	alias, err := g.Store.GetAlias(modelName)
	if err != nil || !alias.Enabled {
		g.logRejected(id, mode, modelName, http.StatusNotFound, "模型不存在或未启用", requestID, start)
		writeOpenAIError(w, http.StatusNotFound, "模型 "+modelName+" 不存在或未启用", "invalid_request_error", "model_not_found")
		return
	}
	if !g.modelAllowed(id, modelName) {
		g.logRejected(id, mode, modelName, http.StatusForbidden, "当前 API Key 无权访问该模型", requestID, start)
		writeOpenAIError(w, http.StatusForbidden, "当前 API Key 无权访问模型 "+modelName, "permission_error", "model_not_allowed")
		return
	}

	release, rej := g.checkQuota(id)
	if rej != nil {
		g.logRejected(id, mode, modelName, rej.status, rej.message, requestID, start)
		if rej.retryAfter > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", rej.retryAfter))
		}
		writeOpenAIError(w, rej.status, rej.message, rej.typ, rej.code)
		return
	}
	defer release()

	targets, err := g.selectTargets(modelName)
	if err != nil {
		g.logRejected(id, mode, modelName, http.StatusServiceUnavailable, "查询供应商失败: "+err.Error(), requestID, start)
		writeOpenAIError(w, http.StatusServiceUnavailable, "查询供应商失败", "server_error", "provider_lookup_failed")
		return
	}
	if len(targets) == 0 {
		g.logRejected(id, mode, modelName, http.StatusServiceUnavailable, "该模型没有可用的供应商路由", requestID, start)
		writeOpenAIError(w, http.StatusServiceUnavailable, "模型 "+modelName+" 暂无可用供应商，请联系管理员", "server_error", "no_available_provider")
		return
	}

	_, hadStreamOptions := payload["stream_options"]
	injectUsage := stream && !hadStreamOptions

	var (
		lastStatus int
		lastBody   []byte
		lastErr    error
		lastTarget store.RouteTarget
		retries    int
		attempted  int
	)

targetLoop:
	for _, target := range targets {
		keys := g.providerKeyOrder(target.Provider)
	keyLoop:
		for _, apiKey := range keys {
			inject := injectUsage
			for try := 0; try < 2; try++ {
				attempted++
				reqBody, err := buildPayload(payload, target.Route.UpstreamModel, inject)
				if err != nil {
					lastErr = err
					continue targetLoop
				}
				resp, err := g.sendUpstream(r.Context(), &target.Provider, apiKey, mode.path, reqBody, stream)
				if err != nil {
					lastErr = err
					lastStatus = http.StatusBadGateway
					lastTarget = target
					retries++
					if r.Context().Err() != nil {
						g.recordFailure(id, mode, modelName, target, requestID, http.StatusBadGateway, "客户端已断开", start, retries)
						return
					}
					continue keyLoop
				}

				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					lastTarget = target
					if stream {
						g.serveStream(w, r, resp, id, alias, target, requestID, start, retries, payload)
					} else {
						g.serveBuffered(w, resp, id, alias, target, requestID, start, retries, payload, mode)
					}
					return
				}

				errBody, _ := readLimited(resp.Body, maxErrorBytes)
				resp.Body.Close()
				lastStatus = resp.StatusCode
				lastBody = errBody
				lastTarget = target
				lastErr = fmt.Errorf("upstream returned %d: %s", resp.StatusCode, errorMessageFromBody(errBody, ""))

				// Some OpenAI-compatible servers reject stream_options. Retry
				// the same target without injecting it before failing over.
				if resp.StatusCode == http.StatusBadRequest && inject {
					inject = false
					retries++
					continue
				}

				switch {
				case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
					resp.StatusCode == http.StatusTooManyRequests:
					retries++
					continue keyLoop // try another upstream key
				case resp.StatusCode >= 500 || resp.StatusCode == http.StatusNotFound:
					retries++
					continue targetLoop // fail over to the next provider
				default:
					// Client-side error (400/413/422...). Return upstream's
					// message; retrying elsewhere will not help.
					g.recordFailure(id, mode, modelName, target, requestID, resp.StatusCode,
						errorMessageFromBody(errBody, "上游拒绝了请求"), start, retries)
					writeUpstreamError(w, resp.StatusCode, errBody)
					return
				}
			}
		}
	}

	status := lastStatus
	if status == 0 {
		status = http.StatusBadGateway
	}
	msg := "所有供应商均不可用"
	if lastBody != nil {
		msg = errorMessageFromBody(lastBody, msg)
	} else if lastErr != nil {
		msg = lastErr.Error()
	}
	g.Logger.Error("all upstream attempts failed", "model", modelName, "attempts", attempted, "err", msg)
	g.recordFailure(id, mode, modelName, lastTarget, requestID, status,
		fmt.Sprintf("上游调用失败（已尝试 %d 次）：%s", attempted, msg), start, retries)
	if status < 400 {
		status = http.StatusBadGateway
	}
	writeOpenAIError(w, status, "上游调用失败："+msg, "upstream_error", "upstream_error")
}

func (mode proxyMode) streamAllowed() bool { return mode.kind != "embeddings" }

func buildPayload(raw map[string]any, upstreamModel string, injectUsage bool) ([]byte, error) {
	m := make(map[string]any, len(raw)+1)
	for k, v := range raw {
		m[k] = v
	}
	m["model"] = upstreamModel
	if injectUsage {
		m["stream_options"] = map[string]any{"include_usage": true}
	}
	return json.Marshal(m)
}

func (g *Gateway) serveBuffered(w http.ResponseWriter, resp *http.Response, id *Identity, alias *store.ModelAlias,
	target store.RouteTarget, requestID string, start time.Time, retries int, payload map[string]any, mode proxyMode) {
	respBody, err := readLimited(resp.Body, maxResponseBytes)
	resp.Body.Close()
	if err != nil {
		g.recordFailure(id, mode, alias.Alias, target, requestID, http.StatusBadGateway,
			"读取上游响应失败: "+err.Error(), start, retries)
		writeOpenAIError(w, http.StatusBadGateway, "读取上游响应失败", "upstream_error", "upstream_read_error")
		return
	}

	var parsed struct {
		Usage *usage `json:"usage"`
		Model string `json:"model"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	u := usage{}
	if parsed.Usage != nil {
		u = *parsed.Usage
	}
	if u.TotalTokens == 0 {
		prompt := estimatePromptTokens(payload)
		total := estimateTokens(string(respBody))
		if total < prompt {
			total = prompt
		}
		completion := total - prompt
		if mode.kind == "embeddings" {
			completion = 0
		}
		u = usage{PromptTokens: prompt, CompletionTokens: completion, TotalTokens: prompt + completion}
	}
	u.normalize()

	// Rewrite the upstream model id back to the client-facing alias.
	if parsed.Model != "" && parsed.Model != alias.Alias {
		var m map[string]any
		if json.Unmarshal(respBody, &m) == nil {
			m["model"] = alias.Alias
			if b, err := json.Marshal(m); err == nil {
				respBody = b
			}
		}
	}

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)

	latency := time.Since(start).Milliseconds()
	g.completeLog(id, alias, target, requestID, resp.StatusCode, "", u, false, retries,
		start, latency, latency, payload)
}

func (g *Gateway) recordFailure(id *Identity, mode proxyMode, modelName string, target store.RouteTarget,
	requestID string, status int, msg string, start time.Time, retries int) {
	latency := time.Since(start).Milliseconds()
	l := &store.RequestLog{
		Ts:            start.Unix(),
		RequestID:     requestID,
		UserID:        &id.User.ID,
		KeyID:         &id.Key.ID,
		Model:         modelName,
		Status:        status,
		Error:         msg,
		LatencyMs:     latency,
		Retries:       retries,
		ClientIP:      id.IP,
		UserAgent:     id.UserAgent,
		UpstreamModel: target.Route.UpstreamModel,
	}
	if target.Provider.ID != 0 {
		l.ProviderID = &target.Provider.ID
	}
	if err := g.Store.InsertLogAndUsage(l, time.Now().Format("2006-01-02")); err != nil {
		g.Logger.Error("write request log", "err", err)
	}
}

func (g *Gateway) logRejected(id *Identity, mode proxyMode, modelName string, status int, msg, requestID string, start time.Time) {
	latency := time.Since(start).Milliseconds()
	l := &store.RequestLog{
		Ts:        start.Unix(),
		RequestID: requestID,
		UserID:    &id.User.ID,
		KeyID:     &id.Key.ID,
		Model:     modelName,
		Status:    status,
		Error:     msg,
		LatencyMs: latency,
		ClientIP:  id.IP,
		UserAgent: id.UserAgent,
	}
	if err := g.Store.InsertLog(l); err != nil {
		g.Logger.Error("write rejected log", "err", err)
	}
}

func (g *Gateway) completeLog(id *Identity, alias *store.ModelAlias, target store.RouteTarget,
	requestID string, status int, errMsg string, u usage, stream bool, retries int,
	start time.Time, latencyMs, firstTokenMs int64, payload map[string]any) {
	u.normalize()
	cost := float64(u.PromptTokens)*alias.InputPrice/1e6 + float64(u.CompletionTokens)*alias.OutputPrice/1e6
	l := &store.RequestLog{
		Ts:               start.Unix(),
		RequestID:        requestID,
		UserID:           &id.User.ID,
		KeyID:            &id.Key.ID,
		Model:            alias.Alias,
		UpstreamModel:    target.Route.UpstreamModel,
		Status:           status,
		Error:            errMsg,
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
		Cost:             cost,
		LatencyMs:        latencyMs,
		FirstTokenMs:     firstTokenMs,
		Stream:           stream,
		Retries:          retries,
		ClientIP:         id.IP,
		UserAgent:        id.UserAgent,
	}
	if target.Provider.ID != 0 {
		l.ProviderID = &target.Provider.ID
	}
	if err := g.Store.InsertLogAndUsage(l, time.Now().Format("2006-01-02")); err != nil {
		g.Logger.Error("write request log", "err", err)
	}
	g.Limiter.RecordTokens(id.Key.ID, u.TotalTokens)
	if id.User.TPMLimit > 0 {
		g.Limiter.RecordTokens(userLimiterID(id.User.ID), u.TotalTokens)
	}
	_ = g.Store.TouchKeyUse(id.Key.ID)
}

func (g *Gateway) HandleModels(w http.ResponseWriter, r *http.Request, id *Identity) {
	aliases, err := g.Store.ListAliases()
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "查询模型失败", "server_error", "internal_error")
		return
	}
	data := make([]map[string]any, 0, len(aliases))
	now := time.Now().Unix()
	for _, a := range aliases {
		if !a.Enabled || !g.modelAllowed(id, a.Alias) {
			continue
		}
		data = append(data, map[string]any{
			"id":       a.Alias,
			"object":   "model",
			"created":  now,
			"owned_by": "ai-gateway",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (g *Gateway) modelAllowed(id *Identity, model string) bool {
	if !matchModelList(id.User.AllowedModels, model) {
		return false
	}
	if len(id.Key.AllowedModels) == 0 {
		return true
	}
	return matchModelList(id.Key.AllowedModels, model)
}

func matchModelList(list []string, model string) bool {
	if len(list) == 0 {
		return true
	}
	for _, v := range list {
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

type quotaRejection struct {
	status     int
	typ        string
	code       string
	message    string
	retryAfter int
}

func (g *Gateway) checkQuota(id *Identity) (func(), *quotaRejection) {
	now := time.Now()
	if id.Key.Status != "active" {
		return func() {}, &quotaRejection{http.StatusForbidden, "permission_error", "key_disabled", "该 API Key 已被禁用", 0}
	}
	if id.Key.ExpiresAt != nil && *id.Key.ExpiresAt > 0 && now.Unix() > *id.Key.ExpiresAt {
		return func() {}, &quotaRejection{http.StatusForbidden, "permission_error", "key_expired", "该 API Key 已过期", 0}
	}
	day := now.Format("2006-01-02")
	month := now.Format("2006-01")
	daily, err := g.Store.KeyUsageOnDay(id.Key.ID, day)
	if err != nil {
		g.Logger.Error("quota lookup", "err", err)
	}
	if id.Key.DailyRequestLimit > 0 && daily.Requests >= int64(id.Key.DailyRequestLimit) {
		return func() {}, &quotaRejection{http.StatusTooManyRequests, "rate_limit_error", "daily_request_limit",
			fmt.Sprintf("今日请求数已达上限（%d 次）", id.Key.DailyRequestLimit), secondsUntilTomorrow(now)}
	}
	if id.Key.DailyTokenLimit > 0 && daily.Tokens >= int64(id.Key.DailyTokenLimit) {
		return func() {}, &quotaRejection{http.StatusTooManyRequests, "rate_limit_error", "daily_token_limit",
			fmt.Sprintf("今日 Token 配额已用尽（%d）", id.Key.DailyTokenLimit), secondsUntilTomorrow(now)}
	}
	if id.Key.MonthlyTokenLimit > 0 {
		monthly, err := g.Store.KeyUsageOnMonth(id.Key.ID, month)
		if err != nil {
			g.Logger.Error("quota lookup", "err", err)
		}
		if monthly.Tokens >= int64(id.Key.MonthlyTokenLimit) {
			return func() {}, &quotaRejection{http.StatusTooManyRequests, "rate_limit_error", "monthly_token_limit",
				fmt.Sprintf("本月 Token 配额已用尽（%d）", id.Key.MonthlyTokenLimit), secondsUntilNextMonth(now)}
		}
	}

	// User-level limits are the outer cap across every key the user owns, so
	// self-provisioned keys cannot escape the administrator's quota.
	if id.User.DailyRequestLimit > 0 || id.User.DailyTokenLimit > 0 {
		userDaily, err := g.Store.UserUsageOnDay(id.User.ID, day)
		if err != nil {
			g.Logger.Error("user quota lookup", "err", err)
		}
		if id.User.DailyRequestLimit > 0 && userDaily.Requests >= int64(id.User.DailyRequestLimit) {
			return func() {}, &quotaRejection{http.StatusTooManyRequests, "rate_limit_error", "user_daily_request_limit",
				fmt.Sprintf("账号今日请求总数已达上限（%d 次）", id.User.DailyRequestLimit), secondsUntilTomorrow(now)}
		}
		if id.User.DailyTokenLimit > 0 && userDaily.Tokens >= int64(id.User.DailyTokenLimit) {
			return func() {}, &quotaRejection{http.StatusTooManyRequests, "rate_limit_error", "user_daily_token_limit",
				fmt.Sprintf("账号今日 Token 总配额已用尽（%d）", id.User.DailyTokenLimit), secondsUntilTomorrow(now)}
		}
	}
	if id.User.MonthlyTokenLimit > 0 {
		userMonthly, err := g.Store.UserUsageOnMonth(id.User.ID, month)
		if err != nil {
			g.Logger.Error("user quota lookup", "err", err)
		}
		if userMonthly.Tokens >= int64(id.User.MonthlyTokenLimit) {
			return func() {}, &quotaRejection{http.StatusTooManyRequests, "rate_limit_error", "user_monthly_token_limit",
				fmt.Sprintf("账号本月 Token 总配额已用尽（%d）", id.User.MonthlyTokenLimit), secondsUntilNextMonth(now)}
		}
	}

	if id.Key.TPMLimit > 0 && g.Limiter.TokensInWindow(id.Key.ID) >= int64(id.Key.TPMLimit) {
		return func() {}, &quotaRejection{http.StatusTooManyRequests, "rate_limit_error", "tpm_limit",
			fmt.Sprintf("每分钟 Token 限额已触发（%d）", id.Key.TPMLimit), 60}
	}
	if id.User.TPMLimit > 0 && g.Limiter.TokensInWindow(userLimiterID(id.User.ID)) >= int64(id.User.TPMLimit) {
		return func() {}, &quotaRejection{http.StatusTooManyRequests, "rate_limit_error", "user_tpm_limit",
			fmt.Sprintf("账号每分钟 Token 总限额已触发（%d）", id.User.TPMLimit), 60}
	}

	releaseKey, reason, retry := g.Limiter.Admit(id.Key.ID, id.Key.RPMLimit, id.Key.ConcurrencyLimit)
	if reason != "" {
		msg := "请求频率超限"
		code := "rpm_limit"
		if reason == "concurrency" {
			msg = "并发请求数超限"
			code = "concurrency_limit"
		}
		return func() {}, &quotaRejection{http.StatusTooManyRequests, "rate_limit_error", code, msg, retry}
	}

	// Only reserve a user-level window when the user actually has a limit;
	// otherwise we would track every request pointlessly.
	releaseUser := func() {}
	if id.User.RPMLimit > 0 || id.User.ConcurrencyLimit > 0 {
		var userReason string
		releaseUser, userReason, retry = g.Limiter.Admit(userLimiterID(id.User.ID),
			id.User.RPMLimit, id.User.ConcurrencyLimit)
		if userReason != "" {
			releaseKey()
			msg := "账号请求频率超限"
			code := "user_rpm_limit"
			if userReason == "concurrency" {
				msg = "账号并发请求数超限"
				code = "user_concurrency_limit"
			}
			return func() {}, &quotaRejection{http.StatusTooManyRequests, "rate_limit_error", code, msg, retry}
		}
	}
	return func() {
		releaseKey()
		releaseUser()
	}, nil
}

// User rate-limit windows share the limiter with key windows. Key IDs are
// positive, so negative user IDs namespace the two safely.
func userLimiterID(userID int64) int64 { return -userID }

func secondsUntilTomorrow(t time.Time) int {
	next := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, 1)
	return int(time.Until(next).Seconds()) + 1
}

func secondsUntilNextMonth(t time.Time) int {
	next := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).AddDate(0, 1, 0)
	return int(time.Until(next).Seconds()) + 1
}

// selectTargets returns routes in priority order, shuffling weighted choices
// within each priority band.
func (g *Gateway) selectTargets(alias string) ([]store.RouteTarget, error) {
	targets, err := g.Store.TargetsForAlias(alias)
	if err != nil {
		return nil, err
	}
	if len(targets) <= 1 {
		return targets, nil
	}
	bands := map[int][]store.RouteTarget{}
	var priorities []int
	for _, t := range targets {
		p := t.Route.Priority
		if _, ok := bands[p]; !ok {
			priorities = append(priorities, p)
		}
		bands[p] = append(bands[p], t)
	}
	for i := 0; i < len(priorities); i++ {
		for j := i + 1; j < len(priorities); j++ {
			if priorities[j] < priorities[i] {
				priorities[i], priorities[j] = priorities[j], priorities[i]
			}
		}
	}
	out := make([]store.RouteTarget, 0, len(targets))
	for _, p := range priorities {
		out = append(out, weightedShuffle(bands[p])...)
	}
	return out, nil
}

func weightedShuffle(in []store.RouteTarget) []store.RouteTarget {
	rest := append([]store.RouteTarget(nil), in...)
	out := make([]store.RouteTarget, 0, len(rest))
	for len(rest) > 0 {
		total := 0
		for _, t := range rest {
			w := t.Route.Weight
			if w < 1 {
				w = 1
			}
			total += w
		}
		pick := int(randUint32()) % total
		idx := 0
		for i, t := range rest {
			w := t.Route.Weight
			if w < 1 {
				w = 1
			}
			if pick < w {
				idx = i
				break
			}
			pick -= w
		}
		out = append(out, rest[idx])
		rest = append(rest[:idx], rest[idx+1:]...)
	}
	return out
}

func (g *Gateway) providerKeyOrder(p store.Provider) []string {
	keys, err := g.Vault.DecryptStrings(p.KeysEnc)
	if err != nil {
		g.Logger.Error("decrypt provider keys", "provider", p.ID, "err", err)
	}
	if len(keys) == 0 {
		return []string{""}
	}
	start := g.nextKeyIndex(p.ID, len(keys))
	out := make([]string, 0, len(keys))
	for i := 0; i < len(keys); i++ {
		out = append(out, keys[(start+i)%len(keys)])
	}
	return out
}

func (g *Gateway) nextKeyIndex(providerID int64, n int) int {
	if n <= 1 {
		return 0
	}
	v, _ := g.keyIdx.LoadOrStore(providerID, new(atomic.Uint64))
	return int(v.(*atomic.Uint64).Add(1)-1) % n
}

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelReadCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

func (g *Gateway) sendUpstream(ctx context.Context, p *store.Provider, apiKey, path string, body []byte, stream bool) (*http.Response, error) {
	timeout := time.Duration(p.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	if stream {
		timeout = time.Duration(p.TimeoutSec) * 10 * time.Second
		if timeout < 30*time.Minute {
			timeout = 30 * time.Minute
		}
		if timeout > time.Hour {
			timeout = time.Hour
		}
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, joinUpstream(p.BaseURL, path), bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("User-Agent", "ai-gateway/1.0")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	for k, v := range parseExtraHeaders(p.ExtraHeaders) {
		req.Header.Set(k, v)
	}
	resp, err := g.Client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelReadCloser{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

func writeUpstreamError(w http.ResponseWriter, status int, body []byte) {
	var probe map[string]any
	if json.Unmarshal(body, &probe) != nil {
		writeOpenAIError(w, status, errorMessageFromBody(body, "上游拒绝了请求"), "upstream_error", "upstream_rejected")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}

func randUint32() uint32 {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}
