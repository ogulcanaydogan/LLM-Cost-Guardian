package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackNotifier sends alerts to a Slack webhook.
type SlackNotifier struct {
	webhookURL string
	channel    string
	client     *http.Client
}

// NewSlackNotifier creates a Slack webhook notifier.
func NewSlackNotifier(webhookURL, channel string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		channel:    channel,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *SlackNotifier) Name() string { return "slack" }

func (s *SlackNotifier) Send(ctx context.Context, alert Alert) error {
	color := "#36a64f" // green
	switch alert.Level {
	case AlertWarning:
		color = "#ff9900" // orange
	case AlertCritical:
		color = "#ff0000" // red
	case AlertExceeded:
		color = "#cc0000" // dark red
	}

	payload := slackPayload{
		Channel: s.channel,
		Attachments: []slackAttachment{
			{
				Color: color,
				Title: fmt.Sprintf("LLM Cost Guardian: Budget %s", string(alert.Level)),
				Fields: []slackField{
					{Title: "Budget", Value: alert.BudgetName, Short: true},
					{Title: "Period", Value: alert.Period, Short: true},
					{Title: "Current Spend", Value: fmt.Sprintf("$%.2f", alert.CurrentSpend), Short: true},
					{Title: "Limit", Value: fmt.Sprintf("$%.2f", alert.LimitUSD), Short: true},
					{Title: "Threshold", Value: fmt.Sprintf("%.0f%%", alert.ThresholdPct), Short: true},
					{Title: "Usage", Value: fmt.Sprintf("%.1f%%", (alert.CurrentSpend/alert.LimitUSD)*100), Short: true},
				},
				Footer: "LLM Cost Guardian",
				Ts:     time.Now().Unix(),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send slack alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}
	return nil
}

type slackPayload struct {
	Channel     string            `json:"channel,omitempty"`
	Attachments []slackAttachment `json:"attachments"`
}

type slackAttachment struct {
	Color  string       `json:"color"`
	Title  string       `json:"title"`
	Fields []slackField `json:"fields"`
	Footer string       `json:"footer"`
	Ts     int64        `json:"ts"`
}

type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}
