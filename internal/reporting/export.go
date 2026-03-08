package reporting

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/tracker"
)

// ProjectChargeback holds aggregated spend for a single project.
type ProjectChargeback struct {
	Project           string
	TotalCostUSD      float64
	TotalInputTokens  int64
	TotalOutputTokens int64
	RecordCount       int64
}

// ReportDocument contains the rendered report payload.
type ReportDocument struct {
	Period      string
	Start       time.Time
	End         time.Time
	Filter      tracker.ReportFilter
	Summary     *tracker.UsageSummary
	Records     []tracker.UsageRecord
	Chargebacks []ProjectChargeback
}

// BuildProjectChargebacks aggregates records into project chargeback rows.
func BuildProjectChargebacks(records []tracker.UsageRecord) []ProjectChargeback {
	grouped := make(map[string]ProjectChargeback)
	for _, record := range records {
		project := record.Project
		if project == "" {
			project = "unassigned"
		}

		row := grouped[project]
		row.Project = project
		row.TotalCostUSD += record.CostUSD
		row.TotalInputTokens += record.InputTokens
		row.TotalOutputTokens += record.OutputTokens
		row.RecordCount++
		grouped[project] = row
	}

	rows := make([]ProjectChargeback, 0, len(grouped))
	for _, row := range grouped {
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalCostUSD != rows[j].TotalCostUSD {
			return rows[i].TotalCostUSD > rows[j].TotalCostUSD
		}
		return rows[i].Project < rows[j].Project
	})

	return rows
}

// DefaultOutputPath returns a stable export filename for the report.
func DefaultOutputPath(period, format string) string {
	ts := time.Now().UTC().Format("20060102-150405")
	ext := strings.ToLower(format)
	return filepath.Join("output", ext, fmt.Sprintf("chargeback-%s-%s.%s", period, ts, ext))
}

// WriteCSV exports a report to CSV.
func WriteCSV(path string, doc ReportDocument, detailed bool) error {
	if err := ensureOutputDir(path); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if detailed {
		if err := writer.Write([]string{
			"timestamp",
			"project",
			"provider",
			"model",
			"input_tokens",
			"output_tokens",
			"cost_usd",
		}); err != nil {
			return fmt.Errorf("write csv header: %w", err)
		}

		for _, record := range doc.Records {
			if err := writer.Write([]string{
				record.Timestamp.Format(time.RFC3339),
				record.Project,
				record.Provider,
				record.Model,
				fmt.Sprintf("%d", record.InputTokens),
				fmt.Sprintf("%d", record.OutputTokens),
				fmt.Sprintf("%.6f", record.CostUSD),
			}); err != nil {
				return fmt.Errorf("write csv record: %w", err)
			}
		}
	} else {
		if err := writer.Write([]string{
			"project",
			"record_count",
			"input_tokens",
			"output_tokens",
			"total_cost_usd",
		}); err != nil {
			return fmt.Errorf("write csv header: %w", err)
		}

		for _, chargeback := range doc.Chargebacks {
			if err := writer.Write([]string{
				chargeback.Project,
				fmt.Sprintf("%d", chargeback.RecordCount),
				fmt.Sprintf("%d", chargeback.TotalInputTokens),
				fmt.Sprintf("%d", chargeback.TotalOutputTokens),
				fmt.Sprintf("%.6f", chargeback.TotalCostUSD),
			}); err != nil {
				return fmt.Errorf("write csv record: %w", err)
			}
		}
	}

	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

func ensureOutputDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	return nil
}
