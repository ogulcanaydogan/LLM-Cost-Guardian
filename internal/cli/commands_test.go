package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/storage"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCLIConfig(t *testing.T) (string, string) {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "guardian.db")
	cfgPath := filepath.Join(dir, "config.yaml")
	pricingDir := filepath.Clean(filepath.Join(wd, "..", "..", "pricing"))

	configData := strings.Join([]string{
		"storage:",
		"  path: " + dbPath,
		"proxy:",
		`  listen: "127.0.0.1:0"`,
		"  read_timeout: 30s",
		"  write_timeout: 60s",
		"  max_body_size: 1024",
		"  add_cost_headers: true",
		"pricing:",
		"  dir: " + pricingDir,
		"logging:",
		"  level: error",
		"  format: text",
		"defaults:",
		"  project: default-project",
	}, "\n")

	require.NoError(t, os.WriteFile(cfgPath, []byte(configData), 0o644))
	return cfgPath, dbPath
}

func resetCommandState() {
	cfgFile = ""
	resetFlags(trackCmd)
	resetFlags(reportCmd)
	resetFlags(budgetSetCmd)
	resetFlags(budgetStatusCmd)
	resetFlags(tenantsCreateCmd)
	resetFlags(tenantsListCmd)
	resetFlags(tenantsDisableCmd)
	resetFlags(apiKeysCreateCmd)
	resetFlags(apiKeysListCmd)
	resetFlags(apiKeysRevokeCmd)
	resetFlags(anomaliesCmd)
	resetFlags(forecastCmd)
	resetFlags(recommendCmd)
	resetFlags(promptsOptimizeCmd)
	resetFlags(providersListCmd)
	resetFlags(proxyStartCmd)
	resetFlags(versionCmd)
}

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	})
	cmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	})
}

func captureOutput(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	stderrR, stderrW, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = stdoutW
	os.Stderr = stderrW

	runErr := fn()

	require.NoError(t, stdoutW.Close())
	require.NoError(t, stderrW.Close())
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	stdoutBytes, err := io.ReadAll(stdoutR)
	require.NoError(t, err)
	stderrBytes, err := io.ReadAll(stderrR)
	require.NoError(t, err)

	return string(stdoutBytes), string(stderrBytes), runErr
}

func TestRunTrack_UsesDefaultProject(t *testing.T) {
	resetCommandState()
	cfgPath, dbPath := testCLIConfig(t)
	cfgFile = cfgPath

	require.NoError(t, trackCmd.Flags().Set("provider", "openai"))
	require.NoError(t, trackCmd.Flags().Set("model", "gpt-4o"))
	require.NoError(t, trackCmd.Flags().Set("input-tokens", "1000"))
	require.NoError(t, trackCmd.Flags().Set("output-tokens", "500"))

	stdout, _, err := captureOutput(t, func() error {
		return runTrack(trackCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "Project:       default-project")

	db, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	records, err := db.QueryUsage(context.Background(), model.ReportFilter{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "default-project", records[0].Project)
}

func TestRunReport_Detailed(t *testing.T) {
	resetCommandState()
	cfgPath, dbPath := testCLIConfig(t)
	cfgFile = cfgPath

	db, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	require.NoError(t, db.RecordUsage(context.Background(), &model.UsageRecord{
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      1.25,
		Project:      "proj-a",
	}))
	require.NoError(t, db.RecordUsage(context.Background(), &model.UsageRecord{
		Provider:     "anthropic",
		Model:        "claude-3.5-sonnet",
		InputTokens:  75,
		OutputTokens: 25,
		CostUSD:      2.50,
		Project:      "proj-b",
	}))

	require.NoError(t, reportCmd.Flags().Set("period", "daily"))
	require.NoError(t, reportCmd.Flags().Set("provider", "openai"))
	require.NoError(t, reportCmd.Flags().Set("detailed", "true"))

	stdout, _, err := captureOutput(t, func() error {
		return runReport(reportCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "Total Cost:          $1.2500")
	assert.Contains(t, stdout, "gpt-4o")
	assert.NotContains(t, stdout, "claude-3.5-sonnet")
}

func TestRunReport_CSVExport(t *testing.T) {
	resetCommandState()
	cfgPath, dbPath := testCLIConfig(t)
	cfgFile = cfgPath

	db, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	require.NoError(t, db.RecordUsage(context.Background(), &model.UsageRecord{
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      1.25,
		Project:      "proj-a",
		Timestamp:    time.Now().UTC(),
	}))

	output := filepath.Join(t.TempDir(), "chargeback.csv")
	require.NoError(t, reportCmd.Flags().Set("period", "daily"))
	require.NoError(t, reportCmd.Flags().Set("format", "csv"))
	require.NoError(t, reportCmd.Flags().Set("output", output))

	stdout, _, err := captureOutput(t, func() error {
		return runReport(reportCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "Exported csv report")

	data, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.Contains(t, string(data), "proj-a,1,100,50,1.250000")
}

func TestRunReport_PDFExport(t *testing.T) {
	resetCommandState()
	cfgPath, dbPath := testCLIConfig(t)
	cfgFile = cfgPath

	db, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	require.NoError(t, db.RecordUsage(context.Background(), &model.UsageRecord{
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      1.25,
		Project:      "proj-a",
		Timestamp:    time.Now().UTC(),
	}))

	output := filepath.Join(t.TempDir(), "chargeback.pdf")
	require.NoError(t, reportCmd.Flags().Set("period", "daily"))
	require.NoError(t, reportCmd.Flags().Set("format", "pdf"))
	require.NoError(t, reportCmd.Flags().Set("output", output))

	_, _, err = captureOutput(t, func() error {
		return runReport(reportCmd, nil)
	})
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.Contains(t, string(data[:8]), "%PDF-1.4")
}

func TestRunBudgetSetAndStatus_ProjectScoped(t *testing.T) {
	resetCommandState()
	cfgPath, _ := testCLIConfig(t)
	cfgFile = cfgPath

	require.NoError(t, budgetSetCmd.Flags().Set("name", "project-budget"))
	require.NoError(t, budgetSetCmd.Flags().Set("project", "proj-a"))
	require.NoError(t, budgetSetCmd.Flags().Set("limit", "100"))
	require.NoError(t, budgetSetCmd.Flags().Set("period", "monthly"))

	stdout, _, err := captureOutput(t, func() error {
		return runBudgetSet(budgetSetCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "Scope:     project:proj-a")

	resetFlags(budgetSetCmd)
	require.NoError(t, budgetSetCmd.Flags().Set("name", "global-budget"))
	require.NoError(t, budgetSetCmd.Flags().Set("limit", "250"))
	require.NoError(t, budgetSetCmd.Flags().Set("period", "monthly"))
	_, _, err = captureOutput(t, func() error {
		return runBudgetSet(budgetSetCmd, nil)
	})
	require.NoError(t, err)

	resetFlags(budgetSetCmd)
	require.NoError(t, budgetSetCmd.Flags().Set("name", "other-project"))
	require.NoError(t, budgetSetCmd.Flags().Set("project", "proj-b"))
	require.NoError(t, budgetSetCmd.Flags().Set("limit", "75"))
	require.NoError(t, budgetSetCmd.Flags().Set("period", "monthly"))
	_, _, err = captureOutput(t, func() error {
		return runBudgetSet(budgetSetCmd, nil)
	})
	require.NoError(t, err)

	require.NoError(t, budgetStatusCmd.Flags().Set("project", "proj-a"))
	stdout, _, err = captureOutput(t, func() error {
		return runBudgetStatus(budgetStatusCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "project-budget")
	assert.Contains(t, stdout, "project:proj-a")
	assert.Contains(t, stdout, "global-budget")
	assert.NotContains(t, stdout, "other-project")
}

func TestRunProvidersList(t *testing.T) {
	resetCommandState()
	cfgPath, _ := testCLIConfig(t)
	cfgFile = cfgPath

	stdout, _, err := captureOutput(t, func() error {
		return runProvidersList(providersListCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "openai")
	assert.Contains(t, stdout, "anthropic")
	assert.Contains(t, stdout, "azure-openai")
	assert.Contains(t, stdout, "bedrock")
	assert.Contains(t, stdout, "vertex-ai")
}

func TestRunProxyStart_UsesListenFlag(t *testing.T) {
	resetCommandState()
	cfgPath, _ := testCLIConfig(t)
	cfgFile = cfgPath

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	proxyStartCmd.SetContext(ctx)
	require.NoError(t, proxyStartCmd.Flags().Set("listen", "127.0.0.1:0"))

	_, stderr, err := captureOutput(t, func() error {
		return runProxyStart(proxyStartCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stderr, "LLM Cost Guardian proxy listening on 127.0.0.1:")
}

func TestVersionCommand(t *testing.T) {
	resetCommandState()
	oldVersion := Version
	Version = "test-version"
	t.Cleanup(func() { Version = oldVersion })

	stdout, _, err := captureOutput(t, func() error {
		versionCmd.Run(versionCmd, nil)
		return nil
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "lcg version test-version")
}

func TestRunTenantAndAPIKeyCommands(t *testing.T) {
	resetCommandState()
	cfgPath, dbPath := testCLIConfig(t)
	cfgFile = cfgPath

	require.NoError(t, tenantsCreateCmd.Flags().Set("slug", "acme"))
	require.NoError(t, tenantsCreateCmd.Flags().Set("name", "Acme Corp"))
	stdout, _, err := captureOutput(t, func() error {
		return runTenantCreate(tenantsCreateCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "Tenant created")
	assert.Contains(t, stdout, "acme")

	stdout, _, err = captureOutput(t, func() error {
		return runTenantList(tenantsListCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "acme")

	require.NoError(t, apiKeysCreateCmd.Flags().Set("tenant", "acme"))
	require.NoError(t, apiKeysCreateCmd.Flags().Set("name", "primary"))
	stdout, _, err = captureOutput(t, func() error {
		return runAPIKeyCreate(apiKeysCreateCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "API key created")
	assert.Contains(t, stdout, "Raw Key:   lcg_")

	require.NoError(t, apiKeysListCmd.Flags().Set("tenant", "acme"))
	stdout, _, err = captureOutput(t, func() error {
		return runAPIKeyList(apiKeysListCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "primary")
	assert.Contains(t, stdout, "acme")

	db, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	keys, err := db.ListAPIKeys(context.Background(), "acme")
	require.NoError(t, err)
	require.Len(t, keys, 1)

	require.NoError(t, apiKeysRevokeCmd.Flags().Set("id", keys[0].ID))
	stdout, _, err = captureOutput(t, func() error {
		return runAPIKeyRevoke(apiKeysRevokeCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "API key revoked")

	require.NoError(t, tenantsDisableCmd.Flags().Set("slug", "acme"))
	stdout, _, err = captureOutput(t, func() error {
		return runTenantDisable(tenantsDisableCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "Tenant disabled: acme")
}

func TestRunAnomaliesCommand(t *testing.T) {
	resetCommandState()
	cfgPath, dbPath := testCLIConfig(t)
	cfgFile = cfgPath

	db, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	base := time.Now().UTC().AddDate(0, 0, -10)
	for day := 0; day < 7; day++ {
		require.NoError(t, db.RecordUsage(context.Background(), &model.UsageRecord{
			Tenant:       "default",
			Provider:     "openai",
			Model:        "gpt-4o",
			InputTokens:  500,
			OutputTokens: 200,
			CostUSD:      0.80,
			Project:      "payments",
			Timestamp:    base.AddDate(0, 0, day),
		}))
	}
	require.NoError(t, db.RecordUsage(context.Background(), &model.UsageRecord{
		Tenant:       "default",
		Provider:     "openai",
		Model:        "gpt-4o",
		InputTokens:  500,
		OutputTokens: 7000,
		CostUSD:      7.50,
		Project:      "payments",
		Timestamp:    base.AddDate(0, 0, 7),
	}))

	require.NoError(t, anomaliesCmd.Flags().Set("tenant", "default"))
	require.NoError(t, anomaliesCmd.Flags().Set("project", "payments"))
	stdout, _, err := captureOutput(t, func() error {
		return runAnomalies(anomaliesCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "payments")
	assert.Contains(t, stdout, "gpt-4o")
}

func TestRunForecastRecommendAndPromptCommands(t *testing.T) {
	resetCommandState()
	cfgPath, dbPath := testCLIConfig(t)
	cfgFile = cfgPath

	db, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	base := time.Now().UTC().AddDate(0, 0, -12)
	for day := 0; day < 10; day++ {
		require.NoError(t, db.RecordUsage(context.Background(), &model.UsageRecord{
			Tenant:       "default",
			Provider:     "openai",
			Model:        "gpt-4o",
			InputTokens:  1500 + int64(day*30),
			OutputTokens: 450 + int64(day*10),
			CostUSD:      2.25 + float64(day)*0.1,
			Project:      "assistant",
			Timestamp:    base.AddDate(0, 0, day),
			Metadata:     `{"prompt_chars":9000,"prompt_tokens_estimate":1600,"input_output_ratio":9,"large_static_context":true,"cached_context_candidate":true}`,
		}))
	}

	require.NoError(t, forecastCmd.Flags().Set("tenant", "default"))
	require.NoError(t, forecastCmd.Flags().Set("project", "assistant"))
	stdout, _, err := captureOutput(t, func() error {
		return runForecast(forecastCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "assistant")
	assert.Contains(t, stdout, "7d")

	require.NoError(t, recommendCmd.Flags().Set("tenant", "default"))
	require.NoError(t, recommendCmd.Flags().Set("project", "assistant"))
	stdout, _, err = captureOutput(t, func() error {
		return runRecommend(recommendCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "gpt-4o")

	require.NoError(t, promptsOptimizeCmd.Flags().Set("tenant", "default"))
	require.NoError(t, promptsOptimizeCmd.Flags().Set("project", "assistant"))
	stdout, _, err = captureOutput(t, func() error {
		return runPromptOptimize(promptsOptimizeCmd, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "assistant")
	assert.Contains(t, stdout, "Reduce")
}
