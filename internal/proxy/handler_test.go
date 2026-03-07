package proxy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/proxy"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/providers"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/storage"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type proxyTestEnv struct {
	handler  *proxy.Handler
	upstream *httptest.Server
	store    storage.Storage
	calls    *atomic.Int32
}

func setupProxyTest(t *testing.T, upstreamHandler http.HandlerFunc, maxBodySize int64, denyOnExceed bool) *proxyTestEnv {
	t.Helper()

	calls := &atomic.Int32{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		upstreamHandler(w, r)
	}))
	t.Cleanup(upstream.Close)

	registry := providers.NewRegistry()
	require.NoError(t, registry.Register(providers.NewOpenAI(&providers.ProviderConfig{
		Provider: "openai",
		Models:   []providers.ModelPricing{{Model: "gpt-4o", InputPerMillion: 2.50, OutputPerMillion: 10.00}},
	})))
	require.NoError(t, registry.Register(providers.NewAnthropic(&providers.ProviderConfig{
		Provider: "anthropic",
		Models:   []providers.ModelPricing{{Model: "claude-3.5-sonnet", InputPerMillion: 3.00, OutputPerMillion: 15.00}},
	})))

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	budgetMgr := tracker.NewBudgetManager(store, nil, logger)
	usageTracker := tracker.NewUsageTracker(registry, store, budgetMgr, logger)

	handler := proxy.NewHandler(usageTracker, "default", maxBodySize, true, denyOnExceed, logger)
	return &proxyTestEnv{
		handler:  handler,
		upstream: upstream,
		store:    store,
		calls:    calls,
	}
}

func openAIResponseHandler(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{
		"id":    "chatcmpl-test",
		"model": "gpt-4o",
		"usage": map[string]any{
			"prompt_tokens":     24,
			"completion_tokens": 8,
			"total_tokens":      32,
		},
		"choices": []map[string]any{
			{"message": map[string]string{"role": "assistant", "content": "Hello!"}},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(err)
	}
}

func TestProxyHandler_MissingTarget(t *testing.T) {
	env := setupProxyTest(t, openAIResponseHandler, 1024, false)

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Zero(t, env.calls.Load())
}

func TestProxyHandler_FullRoundTrip(t *testing.T) {
	env := setupProxyTest(t, openAIResponseHandler, 1024, false)

	body := map[string]any{
		"model": "gpt-4o",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
	req.Header.Set("X-LCG-Target", env.upstream.URL+"/v1/chat/completions")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-LLM-Cost"))
	assert.Equal(t, "24", w.Header().Get("X-LLM-Input-Tokens"))
	assert.Equal(t, "8", w.Header().Get("X-LLM-Output-Tokens"))
	assert.Equal(t, "openai", w.Header().Get("X-LLM-Provider"))
	assert.Equal(t, "gpt-4o", w.Header().Get("X-LLM-Model"))
	assert.Equal(t, int32(1), env.calls.Load())

	records, err := env.store.QueryUsage(context.Background(), model.ReportFilter{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "default", records[0].Project)
}

func TestProxyHandler_ProviderOverride(t *testing.T) {
	env := setupProxyTest(t, openAIResponseHandler, 1024, false)

	body := map[string]any{
		"model": "gpt-4o",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("X-LCG-Target", env.upstream.URL+"/v1/messages")
	req.Header.Set("X-LCG-Provider", "openai")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "openai", w.Header().Get("X-LLM-Provider"))
	assert.Equal(t, "24", w.Header().Get("X-LLM-Input-Tokens"))
	assert.Equal(t, "8", w.Header().Get("X-LLM-Output-Tokens"))
}

func TestProxyHandler_RejectsOversizedBody(t *testing.T) {
	env := setupProxyTest(t, openAIResponseHandler, 16, false)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bytes.Repeat([]byte("a"), 32)))
	req.Header.Set("X-LCG-Target", env.upstream.URL+"/v1/chat/completions")

	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Zero(t, env.calls.Load())
}

func TestProxyHandler_ProjectScopedBudgetEnforcement(t *testing.T) {
	env := setupProxyTest(t, openAIResponseHandler, 1024, true)
	ctx := context.Background()

	require.NoError(t, env.store.SetBudget(ctx, &model.Budget{
		Name:     "global",
		LimitUSD: 100.00,
		Period:   model.PeriodMonthly,
	}))
	require.NoError(t, env.store.SetBudget(ctx, &model.Budget{
		Name:     "proj-a",
		Project:  "proj-a",
		LimitUSD: 100.00,
		Period:   model.PeriodMonthly,
	}))
	require.NoError(t, env.store.SetBudget(ctx, &model.Budget{
		Name:     "proj-b",
		Project:  "proj-b",
		LimitUSD: 50.00,
		Period:   model.PeriodMonthly,
	}))
	require.NoError(t, env.store.UpdateBudgetSpend(ctx, "proj-b", 75.00))

	body := map[string]any{
		"model": "gpt-4o",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	allowedReq := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
	allowedReq.Header.Set("X-LCG-Target", env.upstream.URL+"/v1/chat/completions")
	allowedReq.Header.Set("X-LCG-Project", "proj-a")

	allowedResp := httptest.NewRecorder()
	env.handler.ServeHTTP(allowedResp, allowedReq)
	assert.Equal(t, http.StatusOK, allowedResp.Code)

	blockedReq := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
	blockedReq.Header.Set("X-LCG-Target", env.upstream.URL+"/v1/chat/completions")
	blockedReq.Header.Set("X-LCG-Project", "proj-b")

	blockedResp := httptest.NewRecorder()
	env.handler.ServeHTTP(blockedResp, blockedReq)
	assert.Equal(t, http.StatusPaymentRequired, blockedResp.Code)
}

func TestProxyHandler_InvalidTargetURL(t *testing.T) {
	env := setupProxyTest(t, openAIResponseHandler, 1024, false)

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-LCG-Target", "://invalid-url")

	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Zero(t, env.calls.Load())
}
