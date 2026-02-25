package alerts_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/alerts"
)

func TestWebhookNotifier_Name(t *testing.T) {
	n := alerts.NewWebhookNotifier("https://example.com/webhook", "")
	assert.Equal(t, "webhook", n.Name())
}

func TestWebhookNotifier_Send(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "LLM-Cost-Guardian/1.0", r.Header.Get("User-Agent"))
		assert.Equal(t, http.MethodPost, r.Method)

		err := json.NewDecoder(r.Body).Decode(&received)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := alerts.NewWebhookNotifier(server.URL, "")
	alert := alerts.Alert{
		Level:        alerts.AlertCritical,
		BudgetName:   "prod",
		LimitUSD:     500.00,
		CurrentSpend: 490.00,
		ThresholdPct: 80.0,
		Period:       "monthly",
	}

	err := n.Send(context.Background(), alert)
	require.NoError(t, err)
	assert.Equal(t, "budget_alert", received["event"])
	assert.NotEmpty(t, received["timestamp"])
}

func TestWebhookNotifier_Send_WithHMAC(t *testing.T) {
	var signature string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signature = r.Header.Get("X-Signature-256")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := alerts.NewWebhookNotifier(server.URL, "test-secret")
	err := n.Send(context.Background(), alerts.Alert{Level: alerts.AlertWarning})
	require.NoError(t, err)
	assert.True(t, len(signature) > 0)
	assert.Contains(t, signature, "sha256=")
}

func TestWebhookNotifier_Send_NoHMAC(t *testing.T) {
	var hasSignature bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasSignature = r.Header.Get("X-Signature-256") != ""
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := alerts.NewWebhookNotifier(server.URL, "")
	err := n.Send(context.Background(), alerts.Alert{Level: alerts.AlertWarning})
	require.NoError(t, err)
	assert.False(t, hasSignature)
}

func TestWebhookNotifier_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	n := alerts.NewWebhookNotifier(server.URL, "")
	err := n.Send(context.Background(), alerts.Alert{Level: alerts.AlertWarning})
	assert.Error(t, err)
}
