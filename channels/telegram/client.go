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
	"net/url"
	"time"
)

const (
	defaultBaseURL   = "https://api.telegram.org"
	maxResponseBytes = 1 << 20  // 1 MiB cap on the Bot API JSON response body
	maxMediaBytes    = 20 << 20 // 20 MiB cap on a downloaded media file
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

// DownloadFile resolves a Telegram file_id to its bytes: getFile to obtain the file path, then a GET
// against the file endpoint. The downloaded body is bounded by maxMediaBytes.
func (c *Client) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	getURL := fmt.Sprintf("%s/bot%s/getFile?file_id=%s", c.baseURL, c.botToken, url.QueryEscape(fileID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		return nil, fmt.Errorf("telegram: build getFile request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram: getFile: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram: getFile status %d: %s", resp.StatusCode, string(raw))
	}
	var fr struct {
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &fr); err != nil {
		return nil, fmt.Errorf("telegram: parse getFile: %w", err)
	}
	if fr.Result.FilePath == "" {
		return nil, fmt.Errorf("telegram: getFile returned no file_path")
	}

	dlURL := fmt.Sprintf("%s/file/bot%s/%s", c.baseURL, c.botToken, fr.Result.FilePath)
	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		return nil, fmt.Errorf("telegram: build download request: %w", err)
	}
	dlResp, err := c.http.Do(dlReq)
	if err != nil {
		return nil, fmt.Errorf("telegram: download: %w", err)
	}
	defer func() { _ = dlResp.Body.Close() }()

	if dlResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram: download status %d", dlResp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(dlResp.Body, maxMediaBytes))
	if err != nil {
		return nil, fmt.Errorf("telegram: read file: %w", err)
	}
	return data, nil
}
