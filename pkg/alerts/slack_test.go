package alerts_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yapay-ai/llm-cost-guardian/pkg/alerts"
)

func TestSlackNotifier_Name(t *testing.T) {
	n := alerts.NewSlackNotifier("https://hooks.slack.com/test", "#test")
	assert.Equal(t, "slack", n.Name())
}

func TestSlackNotifier_Send(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, http.MethodPost, r.Method)

		err := json.NewDecoder(r.Body).Decode(&received)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := alerts.NewSlackNotifier(server.URL, "#llm-costs")

	alert := alerts.Alert{
		Level:        alerts.AlertWarning,
		BudgetName:   "test-budget",
		LimitUSD:     100.00,
		CurrentSpend: 85.00,
		ThresholdPct: 80.0,
		Period:       "monthly",
		Message:      "Budget at 85%",
	}

	err := n.Send(context.Background(), alert)
	require.NoError(t, err)
	assert.Equal(t, "#llm-costs", received["channel"])
	assert.NotNil(t, received["attachments"])
}

func TestSlackNotifier_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	n := alerts.NewSlackNotifier(server.URL, "#test")
	err := n.Send(context.Background(), alerts.Alert{Level: alerts.AlertWarning})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestSlackNotifier_AlertLevelColors(t *testing.T) {
	tests := []struct {
		level alerts.AlertLevel
	}{
		{alerts.AlertWarning},
		{alerts.AlertCritical},
		{alerts.AlertExceeded},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			n := alerts.NewSlackNotifier(server.URL, "#test")
			err := n.Send(context.Background(), alerts.Alert{
				Level:    tt.level,
				LimitUSD: 100,
			})
			require.NoError(t, err)
		})
	}
}
