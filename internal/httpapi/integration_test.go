package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-gateway/internal/gateway"
	"ai-gateway/internal/httpapi"
	"ai-gateway/internal/limiter"
	"ai-gateway/internal/secure"
	"ai-gateway/internal/store"
)

type testEnv struct {
	t        *testing.T
	store    *store.Store
	vault    *secure.Vault
	server   *httpapi.Server
	handler  http.Handler
	user     *store.User
	key      *store.APIKey
	keyPlain string
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	vault, err := secure.NewVault(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	lim := limiter.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := gateway.New(st, vault, lim, logger)
	srv := httpapi.New(st, vault, gw, lim, httpapi.Config{
		SiteName: "测试站", Version: "test",
	}, logger)
	env := &testEnv{t: t, store: st, vault: vault, server: srv, handler: srv.Handler()}

	adminHash, _ := secure.HashPassword("admin-pass-123")
	admin := &store.User{Username: "admin", PasswordHash: adminHash, Role: "admin",
		Status: "active", AllowedModels: []string{"*"}}
	if err := st.CreateUser(admin); err != nil {
		t.Fatal(err)
	}
	env.user = env.createUser("alice", []string{"*"})
	env.key, env.keyPlain = env.createKey(env.user, "default", func(k *store.APIKey) {})
	return env
}

func (e *testEnv) createUser(name string, models []string) *store.User {
	e.t.Helper()
	hash, _ := secure.HashPassword("password-123")
	u := &store.User{Username: name, PasswordHash: hash, Role: "user",
		Status: "active", AllowedModels: models}
	if err := e.store.CreateUser(u); err != nil {
		e.t.Fatal(err)
	}
	return u
}

func (e *testEnv) createKey(u *store.User, name string, mutate func(*store.APIKey)) (*store.APIKey, string) {
	e.t.Helper()
	plain, hash, prefix := secure.NewAPIKey()
	k := &store.APIKey{UserID: u.ID, Name: name, KeyHash: hash, KeyPrefix: prefix,
		Status: "active", AllowedModels: []string{}}
	if mutate != nil {
		mutate(k)
	}
	if err := e.store.CreateKey(k); err != nil {
		e.t.Fatal(err)
	}
	return k, plain
}

func (e *testEnv) addProvider(baseURL string, priority int) *store.Provider {
	e.t.Helper()
	enc, err := e.vault.EncryptStrings([]string{"upstream-secret"})
	if err != nil {
		e.t.Fatal(err)
	}
	p := &store.Provider{Name: fmt.Sprintf("provider-%d", priority), Type: "openai",
		BaseURL: baseURL, KeysEnc: enc, ExtraHeaders: "{}", TimeoutSec: 10,
		Priority: priority, Weight: 1, Status: "enabled"}
	if err := e.store.CreateProvider(p); err != nil {
		e.t.Fatal(err)
	}
	return p
}

func (e *testEnv) addModel(alias string, prices ...float64) {
	e.t.Helper()
	in, out := 0.0, 0.0
	if len(prices) > 0 {
		in = prices[0]
	}
	if len(prices) > 1 {
		out = prices[1]
	}
	if err := e.store.UpsertAlias(&store.ModelAlias{Alias: alias, InputPrice: in,
		OutputPrice: out, Enabled: true}); err != nil {
		e.t.Fatal(err)
	}
}

func (e *testEnv) addRoute(alias string, p *store.Provider, upstream string, priority int) {
	e.t.Helper()
	if err := e.store.CreateRoute(&store.ModelRoute{Alias: alias, ProviderID: p.ID,
		UpstreamModel: upstream, Priority: priority, Weight: 1, Enabled: true}); err != nil {
		e.t.Fatal(err)
	}
}

func (e *testEnv) do(method, path, body, key string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func (e *testEnv) chat(model string) *httptest.ResponseRecorder {
	return e.do(http.MethodPost, "/v1/chat/completions",
		fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hello"}]}`, model), e.keyPlain)
}

type mockUpstream struct {
	mu        sync.Mutex
	calls     int
	lastModel string
	stream    bool
	fail      bool
}

func (m *mockUpstream) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "up-model"}}})
		case "/v1/chat/completions":
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			m.mu.Lock()
			m.calls++
			if s, ok := in["model"].(string); ok {
				m.lastModel = s
			}
			fail := m.fail
			stream := m.stream
			m.mu.Unlock()
			if fail {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`{"error":{"message":"mock upstream down"}}`))
				return
			}
			if stream {
				w.Header().Set("Content-Type", "text/event-stream")
				fl := w.(http.Flusher)
				fmt.Fprint(w, "data: {\"id\":\"x\",\"model\":\"up-model\",\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
				fl.Flush()
				fmt.Fprint(w, "data: {\"id\":\"x\",\"model\":\"up-model\",\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}\n\n")
				fl.Flush()
				fmt.Fprint(w, "data: {\"id\":\"x\",\"model\":\"up-model\",\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3,\"total_tokens\":10}}\n\n")
				fl.Flush()
				fmt.Fprint(w, "data: [DONE]\n\n")
				fl.Flush()
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "chatcmpl-1", "object": "chat.completion", "model": "up-model",
				"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "hi"}, "finish_reason": "stop"}},
				"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestChatProxyRoutesAndLogs(t *testing.T) {
	env := newEnv(t)
	up := &mockUpstream{}
	ts := up.server()
	defer ts.Close()
	p := env.addProvider(ts.URL, 10)
	env.addModel("test-model", 1, 2)
	env.addRoute("test-model", p, "up-model", 10)

	rec := env.chat("test-model")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"model":"test-model"`) {
		t.Errorf("response model not rewritten: %s", rec.Body.String())
	}
	up.mu.Lock()
	if up.lastModel != "up-model" {
		t.Errorf("upstream got model %q", up.lastModel)
	}
	up.mu.Unlock()

	logs, total, err := env.store.QueryLogs(&store.LogFilter{Limit: 10})
	if err != nil || total != 1 {
		t.Fatalf("logs total=%d err=%v", total, err)
	}
	l := logs[0]
	if l.Status != 200 || l.TotalTokens != 15 || l.ProviderID == nil || *l.ProviderID != p.ID {
		t.Errorf("unexpected log: %+v", l)
	}
	wantCost := 10*1.0/1e6 + 5*2.0/1e6
	if math.Abs(l.Cost-wantCost) > 1e-9 {
		t.Errorf("cost=%v want %v", l.Cost, wantCost)
	}
	usage, _ := env.store.KeyUsageOnDay(env.key.ID, time.Now().Format("2006-01-02"))
	if usage.Requests != 1 || usage.Tokens != 15 {
		t.Errorf("usage=%+v", usage)
	}
}

func TestChatProxyFailover(t *testing.T) {
	env := newEnv(t)
	bad := &mockUpstream{fail: true}
	badSrv := bad.server()
	defer badSrv.Close()
	good := &mockUpstream{}
	goodSrv := good.server()
	defer goodSrv.Close()

	pBad := env.addProvider(badSrv.URL, 1)
	pGood := env.addProvider(goodSrv.URL, 2)
	env.addModel("failover-model")
	env.addRoute("failover-model", pBad, "up-model", 1)
	env.addRoute("failover-model", pGood, "up-model", 2)

	rec := env.chat("failover-model")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	good.mu.Lock()
	if good.calls != 1 {
		t.Errorf("good provider calls=%d", good.calls)
	}
	good.mu.Unlock()
	logs, _, _ := env.store.QueryLogs(&store.LogFilter{Limit: 5})
	if len(logs) != 1 || logs[0].Retries < 1 {
		t.Errorf("expected retry recorded: %+v", logs)
	}
}

func TestStreamingUsage(t *testing.T) {
	env := newEnv(t)
	up := &mockUpstream{stream: true}
	ts := up.server()
	defer ts.Close()
	p := env.addProvider(ts.URL, 10)
	env.addModel("stream-model")
	env.addRoute("stream-model", p, "up-model", 10)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"stream-model","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer "+env.keyPlain)
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "你好") || !strings.Contains(body, "[DONE]") {
		t.Errorf("stream body missing content: %s", body)
	}
	if !strings.Contains(body, `"model":"stream-model"`) {
		t.Errorf("stream model not rewritten: %s", body)
	}
	logs, _, _ := env.store.QueryLogs(&store.LogFilter{Limit: 5})
	if len(logs) != 1 {
		t.Fatalf("logs=%d", len(logs))
	}
	if !logs[0].Stream || logs[0].TotalTokens != 10 {
		t.Errorf("unexpected stream log: %+v", logs[0])
	}
}

func TestModelPermissionAndQuota(t *testing.T) {
	env := newEnv(t)
	up := &mockUpstream{}
	ts := up.server()
	defer ts.Close()
	p := env.addProvider(ts.URL, 10)
	env.addModel("allowed")
	env.addModel("denied")
	env.addRoute("allowed", p, "up-model", 10)
	env.addRoute("denied", p, "up-model", 10)

	// User-level model restriction.
	env.user.AllowedModels = []string{"allowed"}
	if err := env.store.UpdateUser(env.user); err != nil {
		t.Fatal(err)
	}
	if rec := env.chat("denied"); rec.Code != http.StatusForbidden {
		t.Errorf("denied model status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := env.chat("allowed"); rec.Code != http.StatusOK {
		t.Errorf("allowed model status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Daily request quota (one request was already consumed above).
	k := env.key
	k.DailyRequestLimit = 2
	if err := env.store.UpdateKey(k); err != nil {
		t.Fatal(err)
	}
	first := env.chat("allowed")
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d", first.Code)
	}
	second := env.chat("allowed")
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d body=%s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "daily_request_limit") {
		t.Errorf("unexpected quota body: %s", second.Body.String())
	}
}

func TestUserQuotaAcrossKeys(t *testing.T) {
	env := newEnv(t)
	up := &mockUpstream{}
	ts := up.server()
	defer ts.Close()
	p := env.addProvider(ts.URL, 10)
	env.addModel("quota-model")
	env.addRoute("quota-model", p, "up-model", 10)

	env.user.DailyRequestLimit = 2
	if err := env.store.UpdateUser(env.user); err != nil {
		t.Fatal(err)
	}
	_, secondPlain := env.createKey(env.user, "second", nil)

	body := `{"model":"quota-model","messages":[{"role":"user","content":"hi"}]}`
	if rec := env.do(http.MethodPost, "/v1/chat/completions", body, env.keyPlain); rec.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := env.do(http.MethodPost, "/v1/chat/completions", body, secondPlain); rec.Code != http.StatusOK {
		t.Fatalf("second status=%d body=%s", rec.Code, rec.Body.String())
	}
	third := env.do(http.MethodPost, "/v1/chat/completions", body, secondPlain)
	if third.Code != http.StatusTooManyRequests || !strings.Contains(third.Body.String(), "user_daily_request_limit") {
		t.Fatalf("third status=%d body=%s", third.Code, third.Body.String())
	}

	// Deleting the key that produced the usage must not reset the account's
	// daily total; otherwise users could rotate around their quota.
	if err := env.store.DeleteKey(env.key.ID); err != nil {
		t.Fatal(err)
	}
	_, thirdKeyPlain := env.createKey(env.user, "third", nil)
	fourth := env.do(http.MethodPost, "/v1/chat/completions", body, thirdKeyPlain)
	if fourth.Code != http.StatusTooManyRequests || !strings.Contains(fourth.Body.String(), "user_daily_request_limit") {
		t.Fatalf("after key delete status=%d body=%s", fourth.Code, fourth.Body.String())
	}
}

func TestUserRPMAcrossKeys(t *testing.T) {
	env := newEnv(t)
	up := &mockUpstream{}
	ts := up.server()
	defer ts.Close()
	p := env.addProvider(ts.URL, 10)
	env.addModel("rpm-model")
	env.addRoute("rpm-model", p, "up-model", 10)

	env.user.RPMLimit = 1
	if err := env.store.UpdateUser(env.user); err != nil {
		t.Fatal(err)
	}
	_, secondPlain := env.createKey(env.user, "second", nil)

	body := `{"model":"rpm-model","messages":[{"role":"user","content":"hi"}]}`
	if rec := env.do(http.MethodPost, "/v1/chat/completions", body, env.keyPlain); rec.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", rec.Code, rec.Body.String())
	}
	second := env.do(http.MethodPost, "/v1/chat/completions", body, secondPlain)
	if second.Code != http.StatusTooManyRequests || !strings.Contains(second.Body.String(), "user_rpm_limit") {
		t.Fatalf("second status=%d body=%s", second.Code, second.Body.String())
	}
}

func TestInvalidKeyAndModels(t *testing.T) {
	env := newEnv(t)
	up := &mockUpstream{}
	ts := up.server()
	defer ts.Close()
	p := env.addProvider(ts.URL, 10)
	env.addModel("m1")
	env.addRoute("m1", p, "up-model", 10)

	rec := env.do(http.MethodPost, "/v1/chat/completions", `{"model":"m1"}`, "gw-invalid")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("invalid key status=%d", rec.Code)
	}
	rec = env.do(http.MethodGet, "/v1/models", "", env.keyPlain)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "m1") {
		t.Errorf("models status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminSessionAndCSRF(t *testing.T) {
	env := newEnv(t)
	login := env.do(http.MethodPost, "/api/auth/login", `{"username":"admin","password":"admin-pass-123"}`, "")
	if login.Code != 200 {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range login.Result().Cookies() {
		if c.Name == "gw_session" {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("admin users status=%d body=%s", rec.Code, rec.Body.String())
	}
	// Missing CSRF header must be rejected for state changes.
	req = httptest.NewRequest(http.MethodPost, "/api/keys", strings.NewReader(`{"name":"x"}`))
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("csrf status=%d body=%s", rec.Code, rec.Body.String())
	}
}
