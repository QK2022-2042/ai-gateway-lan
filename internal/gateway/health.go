package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ai-gateway/internal/store"
)

type ProviderCheck struct {
	OK        bool     `json:"ok"`
	Status    int      `json:"status"`
	LatencyMs int64    `json:"latency_ms"`
	Message   string   `json:"message"`
	ModelIDs  []string `json:"model_ids,omitempty"`
}

// CheckProvider probes an upstream's /v1/models endpoint.
func (g *Gateway) CheckProvider(ctx context.Context, p *store.Provider) ProviderCheck {
	keys, _ := g.Vault.DecryptStrings(p.KeysEnc)
	key := ""
	if len(keys) > 0 {
		key = keys[0]
	}
	timeout := 15 * time.Second
	if p.TimeoutSec > 0 && time.Duration(p.TimeoutSec)*time.Second < timeout {
		timeout = time.Duration(p.TimeoutSec) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, joinUpstream(p.BaseURL, pathModels), nil)
	if err != nil {
		return ProviderCheck{Message: err.Error()}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ai-gateway/1.0")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for k, v := range parseExtraHeaders(p.ExtraHeaders) {
		req.Header.Set(k, v)
	}
	resp, err := g.Client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ProviderCheck{LatencyMs: latency, Message: "连接超时"}
		}
		return ProviderCheck{LatencyMs: latency, Message: err.Error()}
	}
	defer resp.Body.Close()
	body, _ := readLimited(resp.Body, 2<<20)
	check := ProviderCheck{Status: resp.StatusCode, LatencyMs: latency}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		check.Message = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, errorMessageFromBody(body, "上游返回错误"))
		return check
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		check.Message = "响应不是合法的模型列表"
		return check
	}
	check.OK = true
	check.Message = fmt.Sprintf("连接正常，发现 %d 个模型", len(parsed.Data))
	for _, d := range parsed.Data {
		if d.ID != "" {
			check.ModelIDs = append(check.ModelIDs, d.ID)
		}
	}
	return check
}
