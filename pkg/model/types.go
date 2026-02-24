package model

import "time"

// UsageRecord represents a single LLM API call with cost data.
type UsageRecord struct {
	ID           string    `json:"id" db:"id"`
	Provider     string    `json:"provider" db:"provider"`
	Model        string    `json:"model" db:"model"`
	InputTokens  int64     `json:"input_tokens" db:"input_tokens"`
	OutputTokens int64     `json:"output_tokens" db:"output_tokens"`
	CostUSD      float64   `json:"cost_usd" db:"cost_usd"`
	Project      string    `json:"project" db:"project"`
	Metadata     string    `json:"metadata,omitempty" db:"metadata"`
	Timestamp    time.Time `json:"timestamp" db:"timestamp"`
}

// BudgetPeriod defines the time window for a budget.
type BudgetPeriod string

const (
	PeriodDaily   BudgetPeriod = "daily"
	PeriodWeekly  BudgetPeriod = "weekly"
	PeriodMonthly BudgetPeriod = "monthly"
)

// Budget defines a spending limit for a time period.
type Budget struct {
	ID                string       `json:"id" db:"id"`
	Name              string       `json:"name" db:"name"`
	LimitUSD          float64      `json:"limit_usd" db:"limit_usd"`
	Period            BudgetPeriod `json:"period" db:"period"`
	CurrentSpend      float64      `json:"current_spend" db:"current_spend"`
	AlertThresholdPct float64      `json:"alert_threshold_pct" db:"alert_threshold_pct"`
	CreatedAt         time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at" db:"updated_at"`
}

// ReportFilter controls what usage records are included in reports.
type ReportFilter struct {
	Provider  string    `json:"provider,omitempty"`
	Model     string    `json:"model,omitempty"`
	Project   string    `json:"project,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
}

// UsageSummary holds aggregated usage statistics.
type UsageSummary struct {
	TotalCostUSD      float64            `json:"total_cost_usd"`
	TotalInputTokens  int64              `json:"total_input_tokens"`
	TotalOutputTokens int64              `json:"total_output_tokens"`
	RecordCount       int64              `json:"record_count"`
	ByProvider        map[string]float64 `json:"by_provider,omitempty"`
	ByModel           map[string]float64 `json:"by_model,omitempty"`
}

// PeriodBounds returns the start and end time for the current period.
func PeriodBounds(period BudgetPeriod) (start, end time.Time) {
	now := time.Now().UTC()
	switch period {
	case PeriodDaily:
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 0, 1)
	case PeriodWeekly:
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 0, 7)
	case PeriodMonthly:
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, 0)
	default:
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 0, 1)
	}
	return start, end
}
