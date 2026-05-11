package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kubeboiii/ims/internal/model"
)

// SlackAlerter posts an incident summary to Slack via an Incoming
// Webhook. FR-6.2: P1 and P2 incidents route here.
//
// Behaviour:
//   - If `WebhookURL` is non-empty, POST the standard {"text": "..."}
//     payload Slack expects.
//   - If `WebhookURL` is empty (no SLACK_WEBHOOK_URL set), the
//     registry constructor should fall back to ConsoleAlerter
//     instead of instantiating us. We belt-and-brace by no-op'ing
//     dispatch when URL is empty — better than panicking.
type SlackAlerter struct {
	WebhookURL string
	HTTPClient *http.Client
}

// NewSlackAlerter builds an alerter with a sensible HTTP client.
// 5-second timeout matches the FR-6.4 budget — alerts can't pile up
// on a slow webhook.
func NewSlackAlerter(url string, timeout time.Duration) *SlackAlerter {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &SlackAlerter{
		WebhookURL: url,
		HTTPClient: &http.Client{Timeout: timeout},
	}
}

// Name identifies the alerter.
func (*SlackAlerter) Name() string { return "slack_webhook" }

// slackPayload is the minimal shape Slack's Incoming Webhook
// expects. The full schema supports blocks/attachments, but a plain
// text field is enough for the assignment.
type slackPayload struct {
	Text string `json:"text"`
}

// Dispatch posts to the webhook. Honours ctx's deadline.
//
// Errors are returned to the caller (the processor) but the caller
// runs this in `go alerter.Dispatch(...)` and just logs failures —
// FR-6.4 forbids blocking workflow on a flaky webhook.
func (s *SlackAlerter) Dispatch(ctx context.Context, wi model.WorkItem) error {
	if s.WebhookURL == "" {
		// Defensive: registry should never wire us up without a URL,
		// but if it did, no-op rather than panic.
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
	// Drain so the connection can be reused (cheap; payload is tiny).
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack: unexpected status %d", resp.StatusCode)
	}
	return nil
}
