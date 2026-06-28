package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL   = "https://slack.com/api"
	maxResponseBytes = 1 << 20
	maxMediaBytes    = 20 << 20
)

// Client calls the Slack Web API. baseURL targets the official host for chat.postMessage; file downloads
// use the per-file url_private from the (signature-verified) event payload, restricted to slack.com.
type Client struct {
	botToken      string
	baseURL       string
	http          *http.Client
	skipHostCheck bool // test-only: allow non-slack.com download hosts (httptest)
}

func NewClient(botToken string) *Client {
	return &Client{
		botToken: botToken,
		baseURL:  defaultBaseURL,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// PostMessage sends a plain-text message to a Slack channel via chat.postMessage. Slack returns HTTP 200
// even for logical failures, so the JSON "ok" field is the real status.
func (c *Client) PostMessage(ctx context.Context, channel, text string) error {
	payload, err := json.Marshal(map[string]any{"channel": channel, "text": text})
	if err != nil {
		return fmt.Errorf("slack: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat.postMessage", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("slack: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+c.botToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("slack: postMessage: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack: postMessage status %d: %s", resp.StatusCode, string(body))
	}
	var res struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return fmt.Errorf("slack: parse postMessage response: %w", err)
	}
	if !res.OK {
		return fmt.Errorf("slack: postMessage failed: %s", res.Error)
	}
	return nil
}

// DownloadFile fetches a file's bytes from its url_private using the bot token. The URL comes from a
// signature-verified event payload; it is additionally restricted to slack.com hosts as defense in depth.
func (c *Client) DownloadFile(ctx context.Context, fileURL string) ([]byte, error) {
	if !c.skipHostCheck {
		u, err := url.Parse(fileURL)
		if err != nil {
			return nil, fmt.Errorf("slack: parse file url: %w", err)
		}
		if u.Scheme != "https" || !isSlackHost(u.Hostname()) {
			return nil, fmt.Errorf("slack: refusing to download from non-slack host %q", u.Hostname())
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("slack: build download request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.botToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slack: download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("slack: download status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMediaBytes))
	if err != nil {
		return nil, fmt.Errorf("slack: read file: %w", err)
	}
	return data, nil
}

func isSlackHost(host string) bool {
	return host == "slack.com" || strings.HasSuffix(host, ".slack.com")
}
