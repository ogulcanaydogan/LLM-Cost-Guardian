package alerts

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookNotifier sends alerts to a generic HTTP webhook.
type WebhookNotifier struct {
	url    string
	secret string
	client *http.Client
}

// NewWebhookNotifier creates a generic webhook notifier.
// If secret is non-empty, requests are signed with HMAC-SHA256.
func NewWebhookNotifier(url, secret string) *WebhookNotifier {
	return &WebhookNotifier{
		url:    url,
		secret: secret,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (w *WebhookNotifier) Name() string { return "webhook" }

func (w *WebhookNotifier) Send(ctx context.Context, alert Alert) error {
	payload := webhookPayload{
		Event:     "budget_alert",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Alert:     alert,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "LLM-Cost-Guardian/1.0")

	if w.secret != "" {
		sig := computeHMAC(body, []byte(w.secret))
		req.Header.Set("X-Signature-256", "sha256="+sig)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

type webhookPayload struct {
	Event     string `json:"event"`
	Timestamp string `json:"timestamp"`
	Alert     Alert  `json:"alert"`
}

func computeHMAC(message, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return hex.EncodeToString(mac.Sum(nil))
}
