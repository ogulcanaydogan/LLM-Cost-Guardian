package alerts

import "context"

// AlertLevel indicates the severity of a budget alert.
type AlertLevel string

const (
	AlertWarning  AlertLevel = "warning"  // Approaching budget threshold
	AlertCritical AlertLevel = "critical" // At or near budget limit
	AlertExceeded AlertLevel = "exceeded" // Budget limit exceeded
)

// Alert represents a budget threshold notification.
type Alert struct {
	Level        AlertLevel `json:"level"`
	BudgetName   string     `json:"budget_name"`
	LimitUSD     float64    `json:"limit_usd"`
	CurrentSpend float64    `json:"current_spend"`
	ThresholdPct float64    `json:"threshold_pct"`
	Period       string     `json:"period"`
	Message      string     `json:"message"`
}

// Notifier sends alerts to external systems.
type Notifier interface {
	// Name returns the notifier identifier.
	Name() string

	// Send delivers an alert. Implementations must be safe for concurrent use.
	Send(ctx context.Context, alert Alert) error
}
