package reporting_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/reporting"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testReportDocument() reporting.ReportDocument {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	records := []tracker.UsageRecord{
		{
			Provider:     "openai",
			Model:        "gpt-4o",
			Project:      "payments",
			InputTokens:  100,
			OutputTokens: 50,
			CostUSD:      1.25,
			Timestamp:    now,
		},
		{
			Provider:     "anthropic",
			Model:        "claude-3.5-sonnet",
			Project:      "risk",
			InputTokens:  75,
			OutputTokens: 20,
			CostUSD:      2.50,
			Timestamp:    now.Add(-time.Hour),
		},
	}

	return reporting.ReportDocument{
		Period:      "daily",
		Start:       now.Add(-24 * time.Hour),
		End:         now,
		Summary:     &tracker.UsageSummary{TotalCostUSD: 3.75, TotalInputTokens: 175, TotalOutputTokens: 70, RecordCount: 2},
		Records:     records,
		Chargebacks: reporting.BuildProjectChargebacks(records),
	}
}

func TestBuildProjectChargebacks(t *testing.T) {
	doc := testReportDocument()
	require.Len(t, doc.Chargebacks, 2)
	assert.Equal(t, "risk", doc.Chargebacks[0].Project)
	assert.Equal(t, "payments", doc.Chargebacks[1].Project)
}

func TestWriteCSV_Summary(t *testing.T) {
	doc := testReportDocument()
	path := filepath.Join(t.TempDir(), "report.csv")

	require.NoError(t, reporting.WriteCSV(path, doc, false))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "project,record_count,input_tokens,output_tokens,total_cost_usd")
	assert.Contains(t, string(data), "risk,1,75,20,2.500000")
}

func TestWriteCSV_Detailed(t *testing.T) {
	doc := testReportDocument()
	path := filepath.Join(t.TempDir(), "report-detailed.csv")

	require.NoError(t, reporting.WriteCSV(path, doc, true))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "timestamp,project,provider,model,input_tokens,output_tokens,cost_usd")
	assert.Contains(t, string(data), "payments,openai,gpt-4o,100,50,1.250000")
}

func TestWritePDF(t *testing.T) {
	doc := testReportDocument()
	path := filepath.Join(t.TempDir(), "report.pdf")

	require.NoError(t, reporting.WritePDF(path, doc, false))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data[:8]), "%PDF-1.4")
	assert.Greater(t, len(data), 500)
}

func TestWritePDF_Detailed(t *testing.T) {
	doc := testReportDocument()
	path := filepath.Join(t.TempDir(), "report-detailed.pdf")

	require.NoError(t, reporting.WritePDF(path, doc, true))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Detailed records")
}

func TestDefaultOutputPath(t *testing.T) {
	path := reporting.DefaultOutputPath("monthly", "pdf")
	assert.Contains(t, path, filepath.Join("output", "pdf", "chargeback-monthly-"))
	assert.Contains(t, path, ".pdf")
}
