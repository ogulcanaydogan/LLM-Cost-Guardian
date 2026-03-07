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
