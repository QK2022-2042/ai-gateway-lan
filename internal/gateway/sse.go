package gateway

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-gateway/internal/store"
)

type streamDelta struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
	Reasoning        string `json:"reasoning"`
	ToolCalls        []struct {
		Function struct {
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}

type streamChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta streamDelta `json:"delta"`
		Text  string      `json:"text"`
	} `json:"choices"`
	Usage *usage `json:"usage"`
}

// serveStream relays an upstream SSE stream while extracting usage and
// rewriting the model id back to the public alias.
func (g *Gateway) serveStream(w http.ResponseWriter, r *http.Request, resp *http.Response, id *Identity,
	alias *store.ModelAlias, target store.RouteTarget, requestID string, start time.Time, retries int,
	payload map[string]any) {
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "text/event-stream; charset=utf-8"
	}
	if !strings.Contains(strings.ToLower(ct), "text/event-stream") {
		// Upstream ignored stream=true and returned a normal JSON body.
		body, err := readLimited(resp.Body, maxResponseBytes)
		if err != nil {
			g.recordFailure(id, proxyMode{kind: "chat"}, alias.Alias, target, requestID,
				http.StatusBadGateway, "读取上游响应失败: "+err.Error(), start, retries)
			writeOpenAIError(w, http.StatusBadGateway, "读取上游响应失败", "upstream_error", "upstream_read_error")
			return
		}
		var parsed struct {
			Usage *usage `json:"usage"`
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &parsed)
		u := usage{}
		if parsed.Usage != nil {
			u = *parsed.Usage
		}
		u.normalize()
		if u.TotalTokens == 0 {
			prompt := estimatePromptTokens(payload)
			total := estimateTokens(string(body))
			if total < prompt {
				total = prompt
			}
			u = usage{PromptTokens: prompt, CompletionTokens: total - prompt, TotalTokens: total}
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
		latency := time.Since(start).Milliseconds()
		g.completeLog(id, alias, target, requestID, resp.StatusCode, "", u, true, retries,
			start, latency, latency, payload)
		return
	}

	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}

	var (
		u            usage
		firstTokenMs int64
		gotFirst     bool
		acc          strings.Builder
		errMsg       string
		clientGone   bool
	)

	reader := bufio.NewReaderSize(resp.Body, 64*1024)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := strings.TrimRight(string(line), "\r\n")
			if strings.HasPrefix(trimmed, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				if data != "" && data != "[DONE]" {
					out, chunkUsage, delta := processChunk(data, alias.Alias)
					if !gotFirst {
						gotFirst = true
						firstTokenMs = time.Since(start).Milliseconds()
					}
					if chunkUsage != nil && (chunkUsage.TotalTokens > 0 || chunkUsage.PromptTokens > 0 || chunkUsage.CompletionTokens > 0) {
						u = *chunkUsage
					}
					if acc.Len() < 2<<20 {
						acc.WriteString(delta)
					}
					if out != nil {
						line = append(out, '\n')
					}
				}
			}
			if _, werr := w.Write(line); werr != nil {
				clientGone = true
				break
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				if r.Context().Err() != nil {
					clientGone = true
				} else {
					errMsg = readErr.Error()
				}
			}
			break
		}
	}

	status := http.StatusOK
	if clientGone {
		status = 499
		errMsg = "客户端提前断开连接"
	} else if errMsg != "" {
		status = http.StatusBadGateway
		errMsg = "上游流式响应中断: " + errMsg
	}

	if u.TotalTokens == 0 {
		prompt := estimatePromptTokens(payload)
		completion := estimateTokens(acc.String())
		u = usage{PromptTokens: prompt, CompletionTokens: completion, TotalTokens: prompt + completion}
	}
	u.normalize()

	latency := time.Since(start).Milliseconds()
	g.completeLog(id, alias, target, requestID, status, errMsg, u, true, retries,
		start, latency, firstTokenMs, payload)
}

// processChunk parses one SSE data payload, optionally rewriting its model id,
// and returns the line to forward plus any usage/content it carried.
func processChunk(data, alias string) ([]byte, *usage, string) {
	var chunk streamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		// Unknown payload shape: forward untouched.
		return []byte("data: " + data), nil, ""
	}
	if chunk.Model != "" && chunk.Model != alias {
		var m map[string]any
		if json.Unmarshal([]byte(data), &m) == nil {
			m["model"] = alias
			if b, err := json.Marshal(m); err == nil {
				data = string(b)
			}
		}
	}
	var delta strings.Builder
	for _, c := range chunk.Choices {
		delta.WriteString(c.Text)
		delta.WriteString(c.Delta.Content)
		delta.WriteString(c.Delta.ReasoningContent)
		delta.WriteString(c.Delta.Reasoning)
		for _, tc := range c.Delta.ToolCalls {
			delta.WriteString(tc.Function.Arguments)
		}
	}
	return []byte("data: " + data), chunk.Usage, delta.String()
}
