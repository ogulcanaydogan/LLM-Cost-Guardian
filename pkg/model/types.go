package model

import "time"

const (
	TenantStatusActive   = "active"
	TenantStatusDisabled = "disabled"

	APIKeyStatusActive  = "active"
	APIKeyStatusRevoked = "revoked"
)

// UsageRecord represents a single LLM API call with cost data.
type UsageRecord struct {
	ID           string    `json:"id" db:"id"`
	TenantID     string    `json:"tenant_id,omitempty" db:"tenant_id"`
	Tenant       string    `json:"tenant,omitempty"`
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
	TenantID          string       `json:"tenant_id,omitempty" db:"tenant_id"`
	Tenant            string       `json:"tenant,omitempty"`
	Name              string       `json:"name" db:"name"`
	Project           string       `json:"project,omitempty" db:"project"`
	LimitUSD          float64      `json:"limit_usd" db:"limit_usd"`
	Period            BudgetPeriod `json:"period" db:"period"`
	CurrentSpend      float64      `json:"current_spend" db:"current_spend"`
	AlertThresholdPct float64      `json:"alert_threshold_pct" db:"alert_threshold_pct"`
	CreatedAt         time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at" db:"updated_at"`
}

// ReportFilter controls what usage records are included in reports.
type ReportFilter struct {
	Tenant    string    `json:"tenant,omitempty"`
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
	ByTenant          map[string]float64 `json:"by_tenant,omitempty"`
	ByProvider        map[string]float64 `json:"by_provider,omitempty"`
	ByModel           map[string]float64 `json:"by_model,omitempty"`
	ByProject         map[string]float64 `json:"by_project,omitempty"`
}

// Tenant identifies a logical customer boundary inside a single deployment.
type Tenant struct {
	ID        string    `json:"id" db:"id"`
	Slug      string    `json:"slug" db:"slug"`
	Name      string    `json:"name" db:"name"`
	Status    string    `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// APIKey stores a hashed access key bound to a tenant.
type APIKey struct {
	ID         string     `json:"id" db:"id"`
	TenantID   string     `json:"tenant_id" db:"tenant_id"`
	Tenant     string     `json:"tenant,omitempty"`
	Name       string     `json:"name" db:"name"`
	KeyPrefix  string     `json:"key_prefix" db:"key_prefix"`
	KeyHash    string     `json:"-" db:"key_hash"`
	Status     string     `json:"status" db:"status"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

// UsageRollup stores an aggregated usage bucket for analytics.
type UsageRollup struct {
	Tenant      string    `json:"tenant"`
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	Project     string    `json:"project"`
	Granularity string    `json:"granularity"`
	BucketStart time.Time `json:"bucket_start"`
	RequestCount int64    `json:"request_count"`
	InputTokens  int64    `json:"input_tokens"`
	OutputTokens int64    `json:"output_tokens"`
	CostUSD      float64  `json:"cost_usd"`
}

// UsageAnomaly describes an abnormal cost spike.
type UsageAnomaly struct {
	Tenant          string    `json:"tenant"`
	Provider        string    `json:"provider,omitempty"`
	Model           string    `json:"model,omitempty"`
	Project         string    `json:"project,omitempty"`
	Granularity     string    `json:"granularity"`
	BucketStart     time.Time `json:"bucket_start"`
	ObservedCostUSD float64   `json:"observed_cost_usd"`
	BaselineCostUSD float64   `json:"baseline_cost_usd"`
	ZScore          float64   `json:"z_score"`
	Severity        string    `json:"severity"`
	Message         string    `json:"message"`
}

// SpendForecast represents a cost forecast for a tenant or project.
type SpendForecast struct {
	Tenant             string  `json:"tenant"`
	Project            string  `json:"project,omitempty"`
	HorizonDays        int     `json:"horizon_days"`
	ForecastCostUSD    float64 `json:"forecast_cost_usd"`
	AverageDailyCostUSD float64 `json:"average_daily_cost_usd"`
	TrendDailyDeltaUSD float64 `json:"trend_daily_delta_usd"`
	Confidence         string  `json:"confidence"`
}

// ModelRecommendation suggests a lower-cost alternative for a workload.
type ModelRecommendation struct {
	Tenant              string  `json:"tenant"`
	Project             string  `json:"project,omitempty"`
	CurrentProvider     string  `json:"current_provider"`
	CurrentModel        string  `json:"current_model"`
	SuggestedProvider   string  `json:"suggested_provider"`
	SuggestedModel      string  `json:"suggested_model"`
	EstimatedSavingsUSD float64 `json:"estimated_savings_usd"`
	EstimatedSavingsPct float64 `json:"estimated_savings_pct"`
	Reason              string  `json:"reason"`
}

// PromptOptimization describes a prompt-efficiency recommendation derived from metadata.
type PromptOptimization struct {
	Tenant          string  `json:"tenant"`
	Project         string  `json:"project,omitempty"`
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	Severity        string  `json:"severity"`
	Suggestion      string  `json:"suggestion"`
	Evidence        string  `json:"evidence"`
	EstimatedImpact string  `json:"estimated_impact"`
	AverageRatio    float64 `json:"average_input_output_ratio,omitempty"`
}

// UsageMetadata stores derived prompt and response efficiency signals without raw prompt content.
type UsageMetadata struct {
	PromptChars            int     `json:"prompt_chars,omitempty"`
	PromptTokensEstimate   int64   `json:"prompt_tokens_estimate,omitempty"`
	SystemPromptChars      int     `json:"system_prompt_chars,omitempty"`
	MessageCount           int     `json:"message_count,omitempty"`
	RepeatedLineRatio      float64 `json:"repeated_line_ratio,omitempty"`
	LargeStaticContext     bool    `json:"large_static_context,omitempty"`
	CachedContextCandidate bool    `json:"cached_context_candidate,omitempty"`
	InputOutputRatio       float64 `json:"input_output_ratio,omitempty"`
	Streaming              bool    `json:"streaming,omitempty"`
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
