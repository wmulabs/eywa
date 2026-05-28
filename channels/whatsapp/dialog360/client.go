package dialog360

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	eywa "github.com/wmulabs/eywa"
)

const (
	defaultBaseURL = "https://waba-v2.360dialog.io"
	defaultTimeout = 30 * time.Second

	maxMediaDownloadBytes = 20 * 1024 * 1024 // 20 MiB — WhatsApp max media size
	maxAPIResponseBytes   = 1 * 1024 * 1024  // 1 MiB — API JSON responses
)

// Dialog360Client: https://docs.360dialog.com/docs/waba-messaging/messaging
type Dialog360Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

type Dialog360Config struct {
	APIKey     string
	BaseURL    string        // Optional: defaults to https://waba-v2.360dialog.io
	Timeout    time.Duration // Optional: defaults to 30s
	HTTPClient *http.Client  // Optional: custom HTTP client
}

func NewDialog360Client(config Dialog360Config) *Dialog360Client {
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}

	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: config.Timeout,
		}
	}

	return &Dialog360Client{
		apiKey:     config.APIKey,
		baseURL:    config.BaseURL,
		httpClient: httpClient,
	}
}

func (c *Dialog360Client) SendMessage(ctx context.Context, phone, message string, metadata map[string]any) error {
	payload := c.buildPayload(phone, message, metadata)

	body, err := json.Marshal(payload)
	if err != nil {
		return eywa.NewInfrastructureError("failed to marshal request", err)
	}

	url := fmt.Sprintf("%s/v1/messages", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return eywa.NewInfrastructureError("failed to create request", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("D360-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return eywa.NewInfrastructureError("failed to send request", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	return c.errorFrom(resp)
}

// SendTemplateMessage: https://docs.360dialog.com/docs/waba-messaging/templates
func (c *Dialog360Client) SendTemplateMessage(ctx context.Context, phone string, template *eywa.TemplateMessage) error {
	payload := c.buildTemplatePayload(phone, template)

	body, err := json.Marshal(payload)
	if err != nil {
		return eywa.NewInfrastructureError("failed to marshal template request", err)
	}

	url := fmt.Sprintf("%s/v1/messages", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return eywa.NewInfrastructureError("failed to create template request", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("D360-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return eywa.NewInfrastructureError("failed to send template request", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	return c.errorFrom(resp)
}

// buildTemplatePayload converts TemplateMessage to Meta WhatsApp API format.
func (c *Dialog360Client) buildTemplatePayload(phone string, template *eywa.TemplateMessage) map[string]any {
	payload := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                phone,
		"type":              "template",
		"template": map[string]any{
			"name": template.Name,
			"language": map[string]any{
				"code": template.Language,
			},
		},
	}

	if len(template.Components) > 0 {
		components := make([]map[string]any, 0, len(template.Components))

		for _, comp := range template.Components {
			component := map[string]any{
				"type": comp.Type,
			}

			if comp.SubType != "" {
				component["sub_type"] = comp.SubType
			}

			if comp.Type == "button" {
				component["index"] = comp.Index
			}

			if len(comp.Parameters) > 0 {
				parameters := make([]map[string]any, 0, len(comp.Parameters))

				for _, param := range comp.Parameters {
					parameter := map[string]any{
						"type": param.Type,
					}

					switch param.Type {
					case "text":
						parameter["text"] = param.Text
					case "image":
						parameter["image"] = map[string]any{
							"link": param.Image,
						}
					case "video":
						parameter["video"] = map[string]any{
							"link": param.Video,
						}
					}

					parameters = append(parameters, parameter)
				}

				component["parameters"] = parameters
			}

			components = append(components, component)
		}

		payload["template"].(map[string]any)["components"] = components
	}

	return payload
}

func (c *Dialog360Client) buildPayload(phone, message string, metadata map[string]any) map[string]any {
	payload := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                phone,
	}

	if buttons, ok := metadata["buttons"].([]any); ok && len(buttons) > 0 {
		payload["type"] = "interactive"
		payload["interactive"] = c.buildInteractivePayload(message, buttons)
		return payload
	}

	if imageURL, ok := metadata["image_url"].(string); ok && imageURL != "" {
		payload["type"] = "image"
		payload["image"] = map[string]any{
			"link": imageURL,
		}
		if message != "" {
			payload["image"].(map[string]any)["caption"] = message
		}
		return payload
	}

	payload["type"] = "text"
	payload["text"] = map[string]any{
		"body": message,
	}

	return payload
}

func (c *Dialog360Client) buildInteractivePayload(message string, buttons []any) map[string]any {
	interactive := map[string]any{
		"type": "button",
		"body": map[string]any{
			"text": message,
		},
	}

	actionButtons := make([]map[string]any, 0, len(buttons))
	for i, btn := range buttons {
		if btnMap, ok := btn.(map[string]any); ok {
			title, _ := btnMap["title"].(string)
			id, _ := btnMap["id"].(string)

			if id == "" {
				id = fmt.Sprintf("btn_%d", i+1)
			}

			actionButtons = append(actionButtons, map[string]any{
				"type": "reply",
				"reply": map[string]any{
					"id":    id,
					"title": title,
				},
			})
		}
	}

	interactive["action"] = map[string]any{
		"buttons": actionButtons,
	}

	return interactive
}

// DownloadMedia: https://docs.360dialog.com/docs/waba-messaging/media
func (c *Dialog360Client) DownloadMedia(ctx context.Context, mediaID string) (data []byte, mimeType string, err error) {
	url := fmt.Sprintf("%s/v1/media/%s", c.baseURL, mediaID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", eywa.NewInfrastructureError("failed to create media info request", err)
	}

	req.Header.Set("D360-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", eywa.NewInfrastructureError("failed to get media info", err)
	}
	defer resp.Body.Close()

	if toolErr := eywa.FromHTTPResponse(resp); toolErr != nil {
		return nil, "", toolErr
	}

	var mediaInfo struct {
		URL      string `json:"url"`
		MimeType string `json:"mime_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mediaInfo); err != nil {
		return nil, "", eywa.NewInfrastructureError("failed to decode media info", err)
	}

	// 360Dialog requires replacing the Facebook CDN host with its own host for downloads.
	// Original: https://lookaside.fbsbx.com/..., Download: https://waba-v2.360dialog.io/...
	downloadURL := strings.Replace(mediaInfo.URL, "https://lookaside.fbsbx.com", c.baseURL, 1)

	downloadReq, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, "", eywa.NewInfrastructureError("failed to create download request", err)
	}

	downloadReq.Header.Set("D360-API-KEY", c.apiKey)

	downloadResp, err := c.httpClient.Do(downloadReq)
	if err != nil {
		return nil, "", eywa.NewInfrastructureError("failed to download media", err)
	}
	defer downloadResp.Body.Close() //nolint:errcheck

	if toolErr := eywa.FromHTTPResponse(downloadResp); toolErr != nil {
		return nil, "", toolErr
	}

	data, err = io.ReadAll(io.LimitReader(downloadResp.Body, maxMediaDownloadBytes))
	if err != nil {
		return nil, "", eywa.NewInfrastructureError("failed to read media data", err)
	}

	return data, mediaInfo.MimeType, nil
}

// errorFrom: parses Dialog360's {"error":{"message":"..."}} response format.
func (c *Dialog360Client) errorFrom(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseBytes))

	var apiErr struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
		return eywa.FromHTTPStatus(resp.StatusCode, apiErr.Error.Message)
	}

	return eywa.FromHTTPStatus(resp.StatusCode, string(body))
}
