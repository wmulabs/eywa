// Package telegram is an inbound/outbound channel adapter for the Telegram Bot API. It implements
// eywa.Receptor (webhook Update -> Pulse), eywa.Voice (response -> chat), and a RequestVerifier for the
// Bot API secret token. Webhook-based; long polling is out of scope.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL   = "https://api.telegram.org"
	maxResponseBytes = 1 << 20 // 1 MiB cap on the Bot API response body
)

// Client calls the Telegram Bot API. The base URL is fixed to the official host (no user-controlled
// URLs, so there is no SSRF surface); it is overridable within the package for tests.
type Client struct {
	botToken string
	baseURL  string
	http     *http.Client
}

func NewClient(botToken string) *Client {
	return &Client{
		botToken: botToken,
		baseURL:  defaultBaseURL,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// SendMessage posts a plain-text message to a Telegram chat.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	payload, err := json.Marshal(map[string]any{"chat_id": chatID, "text": text})
	if err != nil {
		return fmt.Errorf("telegram: marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", c.baseURL, c.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: api status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
