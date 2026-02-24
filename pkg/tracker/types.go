package tracker

import "github.com/yapay-ai/llm-cost-guardian/pkg/model"

// Re-export types from model package for convenience.
type (
	UsageRecord  = model.UsageRecord
	Budget       = model.Budget
	BudgetPeriod = model.BudgetPeriod
	ReportFilter = model.ReportFilter
	UsageSummary = model.UsageSummary
)

// Re-export constants.
const (
	PeriodDaily   = model.PeriodDaily
	PeriodWeekly  = model.PeriodWeekly
	PeriodMonthly = model.PeriodMonthly
)

// PeriodBounds wraps model.PeriodBounds.
var PeriodBounds = model.PeriodBounds
