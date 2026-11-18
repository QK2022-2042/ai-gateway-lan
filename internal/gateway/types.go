package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"ai-gateway/internal/store"
)

// Identity is the authenticated caller of an inference endpoint.
type Identity struct {
	Key       *store.APIKey
	User      *store.User
	IP        string
	UserAgent string
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

type openAIErrorBody struct {
	Error openAIError `json:"error"`
}

func writeOpenAIError(w http.ResponseWriter, status int, message, typ, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(openAIErrorBody{Error: openAIError{
		Message: message, Type: typ, Code: code,
	}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type usage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

func (u *usage) normalize() {
	if u.TotalTokens == 0 {
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
	}
	if u.TotalTokens > 0 && u.PromptTokens == 0 && u.CompletionTokens == 0 {
		u.CompletionTokens = u.TotalTokens
	}
}

// estimateTokens is a dependency-free approximation used only when an
// upstream does not return usage. Latin text averages ~4 chars/token, CJK and
// other non-ASCII scripts average closer to 1.5 chars/token.
func estimateTokens(s string) int64 {
	if s == "" {
		return 0
	}
	var ascii, nonASCII int64
	for _, r := range s {
		if r < utf8.RuneSelf {
			ascii++
		} else {
			nonASCII++
		}
	}
	n := ascii/4 + nonASCII*2/3
	if n < 1 {
		n = 1
	}
	return n
}

func estimatePromptTokens(payload map[string]any) int64 {
	// Messages are the dominant prompt component. Marshaling is cheap and
	// gives a stable approximation across chat/completion/embedding shapes.
	b, err := json.Marshal(payload)
	if err != nil {
		return 0
	}
	return estimateTokens(string(b))
}

func joinUpstream(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/v1") {
		return base + strings.TrimPrefix(path, "/v1")
	}
	if base == "" {
		return path
	}
	return base + path
}

func readLimited(r io.Reader, max int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, max))
}

func parseExtraHeaders(raw string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func errorMessageFromBody(body []byte, fallback string) string {
	var b struct {
		Error struct {
			Message string `json:"message"`
			Detail  string `json:"detail"`
		} `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal(body, &b) == nil {
		switch {
		case b.Error.Message != "":
			return b.Error.Message
		case b.Error.Detail != "":
			return b.Error.Detail
		case b.Message != "":
			return b.Message
		case b.Detail != "":
			return b.Detail
		}
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 500 {
		text = text[:500] + "…"
	}
	if text == "" {
		text = fallback
	}
	return text
}
