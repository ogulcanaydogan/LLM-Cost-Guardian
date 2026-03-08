package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger_LevelsAndFormats(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		format string
	}{
		{name: "debug-text", level: "debug", format: "text"},
		{name: "warn-json", level: "warn", format: "json"},
		{name: "error-text", level: "error", format: "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(&config.Config{
				Logging: config.LoggingConfig{
					Level:  tt.level,
					Format: tt.format,
				},
			})
			assert.NotNil(t, logger)
		})
	}
}

func TestNewNotifiers(t *testing.T) {
	cfg := &config.Config{
		Alerts: config.AlertsConfig{
			Slack: config.SlackConfig{
				Enabled:    true,
				WebhookURL: "https://hooks.slack.test/services/abc",
				Channel:    "#llm-costs",
			},
			Webhook: config.WebhookConfig{
				Enabled: true,
				URL:     "https://example.test/hooks",
				Secret:  "topsecret",
			},
		},
	}

	notifiers := NewNotifiers(cfg)
	require.Len(t, notifiers, 2)
}

func TestResolvePricingDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "openai.yaml"), []byte("provider: openai\nmodels:\n  - model: gpt-4o\n    input_per_million: 1\n    output_per_million: 2\n"), 0o644))
	assert.Equal(t, dir, resolvePricingDir(dir))

	missing := filepath.Join(t.TempDir(), "missing")
	assert.Equal(t, missing, resolvePricingDir(missing))
}

func TestServiceClose_NilStore(t *testing.T) {
	service := &Service{}
	assert.NoError(t, service.Close())
}
