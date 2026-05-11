package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kubeboiii/vellum/internal/model"
)

type SlackAlerter struct {
	WebhookURL string
	HTTPClient *http.Client
}

func NewSlackAlerter(url string, timeout time.Duration) *SlackAlerter {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &SlackAlerter{
		WebhookURL: url,
		HTTPClient: &http.Client{Timeout: timeout},
	}
}

func (*SlackAlerter) Name() string { return "slack_webhook" }

type slackPayload struct {
	Text string `json:"text"`
}

func (s *SlackAlerter) Dispatch(ctx context.Context, wi model.WorkItem) error {
	if s.WebhookURL == "" {

		return nil
	}

	body := slackPayload{
		Text: fmt.Sprintf("🚨 *%s* incident on `%s` (signals: %d)",
			wi.Severity, wi.ComponentID, wi.SignalCount),
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("slack: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.WebhookURL, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("slack: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack: post: %w", err)
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack: unexpected status %d", resp.StatusCode)
	}
	return nil
}
