package proxy_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yapay-ai/llm-cost-guardian/internal/proxy"
	"github.com/yapay-ai/llm-cost-guardian/pkg/providers"
	"github.com/yapay-ai/llm-cost-guardian/pkg/storage"
	"github.com/yapay-ai/llm-cost-guardian/pkg/tracker"
)

func setupProxyTest(t *testing.T) (*proxy.Handler, *httptest.Server) {
	t.Helper()

	// Create mock upstream LLM API
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(upstream.Close)

	// Setup tracker
	registry := providers.NewRegistry()
	openai := providers.NewOpenAI(&providers.ProviderConfig{
		Provider: "openai",
		Models:   []providers.ModelPricing{{Model: "gpt-4o", InputPerMillion: 2.50, OutputPerMillion: 10.00}},
	})
	_ = registry.Register(openai)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ut := tracker.NewUsageTracker(registry, store, nil, logger)

	handler := proxy.NewHandler(ut, "default", true, false, logger)
	return handler, upstream
}

func TestProxyHandler_MissingTarget(t *testing.T) {
	handler, _ := setupProxyTest(t)

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProxyHandler_FullRoundTrip(t *testing.T) {
	handler, upstream := setupProxyTest(t)

	body := map[string]any{
		"model": "gpt-4o",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
	req.Header.Set("X-LCG-Target", upstream.URL+"/v1/chat/completions")
	req.Header.Set("X-LCG-Provider", "openai")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check cost headers
	assert.NotEmpty(t, w.Header().Get("X-LLM-Cost"))
	assert.Equal(t, "24", w.Header().Get("X-LLM-Input-Tokens"))
	assert.Equal(t, "8", w.Header().Get("X-LLM-Output-Tokens"))
	assert.Equal(t, "openai", w.Header().Get("X-LLM-Provider"))
	assert.Equal(t, "gpt-4o", w.Header().Get("X-LLM-Model"))
}

func TestProxyHandler_WithProject(t *testing.T) {
	handler, upstream := setupProxyTest(t)

	body := map[string]any{
		"model":    "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": "Hi"}},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
	req.Header.Set("X-LCG-Target", upstream.URL+"/v1/chat/completions")
	req.Header.Set("X-LCG-Provider", "openai")
	req.Header.Set("X-LCG-Project", "my-project")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProxyHandler_InvalidTargetURL(t *testing.T) {
	handler, _ := setupProxyTest(t)

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-LCG-Target", "://invalid-url")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
