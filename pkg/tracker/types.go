package tracker

import "github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"

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
