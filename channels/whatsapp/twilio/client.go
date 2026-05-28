package twilio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	twiliosdk "github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
	eywa "github.com/wmulabs/eywa"
)

const (
	defaultPollInterval    = 2 * time.Second
	defaultDeliveryTimeout = 15 * time.Second

	maxMediaDownloadBytes = 20 * 1024 * 1024 // 20 MiB — WhatsApp max media size
	maxAPIResponseBytes   = 1 * 1024 * 1024  // 1 MiB — API JSON responses
)

// terminalStatuses are Twilio message statuses that indicate no further
// transitions will occur. Anything outside this set means the message is
// still in flight.
var terminalStatuses = map[string]bool{
	"delivered":   true,
	"read":        true,
	"sent":        true, // sent to WhatsApp, but delivery receipt not confirmed
	"failed":      true,
	"undelivered": true,
}

var failedStatuses = map[string]bool{
	"failed":      true,
	"undelivered": true,
}

// TwilioClient: https://www.twilio.com/docs/whatsapp/api
type TwilioClient struct {
	client          *twiliosdk.RestClient
	httpClient      *http.Client // used by DownloadMedia; defaults to &http.Client{Timeout: 30s}
	accountSID      string
	authToken       string
	fromNumber      string
	pollInterval    time.Duration
	deliveryTimeout time.Duration
	templateSIDs    map[string]string // Maps template names to Twilio Content SIDs
}

// TwilioConfig holds configuration for TwilioClient.
//
// Example usage with template mapping:
//
//	client := twilio.NewTwilioClient(twilio.TwilioConfig{
//	    AccountSID: "AC1234567890abcdef1234567890abcdef",
//	    AuthToken:  "your-auth-token",
//	    FromNumber: "+1234567890",
//	    TemplateSIDs: map[string]string{
//	        "freight_route_ready_notification": "HXb1234567890abcdef1234567890abcd",
//	        "delivery_confirmation":             "HXc9876543210fedcba0987654321fedc",
//	        "order_status_update":               "HXa1b2c3d4e5f6789012345678901234ab",
//	    },
//	})
//
// This allows LLM agents to use readable template names like "freight_route_ready_notification"
// while the client automatically resolves them to Twilio's technical Content SIDs.
type TwilioConfig struct {
	AccountSID string
	AuthToken  string

	// FromNumber is the WhatsApp-enabled Twilio number (e.g. "+1234567890").
	// The "whatsapp:" prefix is added automatically if absent.
	FromNumber string

	// PollInterval controls how often SendMessage polls for delivery status.
	// Defaults to 2s if zero.
	PollInterval time.Duration

	// DeliveryTimeout is the maximum time SendMessage waits for a terminal
	// status before returning an infrastructure error. Defaults to 30s if zero.
	DeliveryTimeout time.Duration

	// TemplateSIDs maps user-friendly template names to Twilio Content SIDs.
	// This allows LLMs to use readable names while Twilio requires technical SIDs.
	//
	// Format: map[friendlyName]contentSID
	// Example: {"freight_route_ready_notification": "HXb1234567890abcdef1234567890abcd"}
	//
	// All templates used by agents MUST be configured in this map.
	// If a template name is not found, SendTemplateMessage will return
	// a BusinessError with the list of available templates.
	TemplateSIDs map[string]string
}

func NewTwilioClient(config TwilioConfig) *TwilioClient {
	if config.PollInterval == 0 {
		config.PollInterval = defaultPollInterval
	}
	if config.DeliveryTimeout == 0 {
		config.DeliveryTimeout = defaultDeliveryTimeout
	}

	templateSIDs := config.TemplateSIDs
	if templateSIDs == nil {
		templateSIDs = make(map[string]string)
	}

	return &TwilioClient{
		client: twiliosdk.NewRestClientWithParams(twiliosdk.ClientParams{
			Username: config.AccountSID,
			Password: config.AuthToken,
		}),
		accountSID:      config.AccountSID,
		authToken:       config.AuthToken,
		fromNumber:      formatWhatsAppNumber(config.FromNumber),
		pollInterval:    config.PollInterval,
		deliveryTimeout: config.DeliveryTimeout,
		templateSIDs:    templateSIDs,
	}
}

// SendMessage blocks until delivered/read/sent or timeout. Business errors for invalid
// numbers or delivery failures; infrastructure errors for Twilio issues or timeout.
func (c *TwilioClient) SendMessage(ctx context.Context, phone, message string, metadata map[string]any) error {
	params := c.buildMessageParams(phone, message, metadata)

	resp, err := c.client.Api.CreateMessage(params)
	if err != nil {
		return c.classifyTwilioError(err)
	}

	if resp.Sid == nil {
		return eywa.NewInfrastructureError("twilio returned no message SID", nil)
	}

	return c.waitForTerminalStatus(ctx, *resp.Sid)
}

// SendTemplateMessage: Twilio expects ContentSid and ContentVariables;
// components are converted to flat variable numbering (1, 2, 3...).
func (c *TwilioClient) SendTemplateMessage(ctx context.Context, phone string, template *eywa.TemplateMessage) error {
	contentSid, err := c.resolveTemplateSID(template.Name)
	if err != nil {
		return err
	}

	params := &twilioApi.CreateMessageParams{}
	params.SetTo(formatWhatsAppNumber(phone))
	params.SetFrom(c.fromNumber)
	params.SetContentSid(contentSid)

	variables := c.buildTwilioVariables(template)

	if len(variables) > 0 {
		varsJSON, err := json.Marshal(variables)
		if err != nil {
			return eywa.NewInfrastructureError("failed to marshal template variables", err)
		}
		params.SetContentVariables(string(varsJSON))
	}

	resp, err := c.client.Api.CreateMessage(params)
	if err != nil {
		return c.classifyTwilioError(err)
	}

	if resp.Sid == nil {
		return eywa.NewInfrastructureError("twilio returned no message SID", nil)
	}

	return c.waitForTerminalStatus(ctx, *resp.Sid)
}

// resolveTemplateSID: template name must exist in the templateSIDs map.
func (c *TwilioClient) resolveTemplateSID(templateName string) (string, error) {
	if sid, exists := c.templateSIDs[templateName]; exists {
		return sid, nil
	}

	return "", eywa.NewBusinessError(
		fmt.Sprintf("template '%s' not found in Twilio template mappings", templateName),
		fmt.Errorf("available templates: %v", eywa.StringMapKeys(c.templateSIDs)),
	)
}

// buildTwilioVariables converts our template components to Twilio's numbered format.
// Twilio uses a flat numbering across all components: {"1": "first", "2": "second", ...}
//
// Empty parameters are replaced with "N/D" (No Disponible) to prevent Twilio error 21656.
func (c *TwilioClient) buildTwilioVariables(template *eywa.TemplateMessage) map[string]string {
	variables := make(map[string]string)
	position := 1

	for _, component := range template.Components {
		for _, param := range component.Parameters {
			key := fmt.Sprintf("%d", position)
			var value string

			switch param.Type {
			case "text":
				value = safeParam(param.Text)
			case "image":
				value = safeParam(param.Image)
			case "video":
				value = safeParam(param.Video)
			default:
				value = safeParam(param.Text)
			}

			variables[key] = value
			position++
		}
	}

	return variables
}

// safeParam ensures parameter values are never empty (Twilio requirement).
// Returns "N/D" (No Disponible) for empty/whitespace-only strings to prevent error 21656.
func safeParam(s string) string {
	if strings.TrimSpace(s) == "" {
		return "N/D"
	}
	return s
}

func (c *TwilioClient) waitForTerminalStatus(ctx context.Context, sid string) error {
	ctx, cancel := context.WithTimeout(ctx, c.deliveryTimeout)
	defer cancel()

	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return eywa.NewInfrastructureError(
				fmt.Sprintf("delivery confirmation timed out after %s (sid: %s)", c.deliveryTimeout, sid),
				ctx.Err(),
			)

		case <-ticker.C:
			status, errorMessage, errorCode, err := c.fetchMessageStatus(sid)
			if err != nil {
				// Treat a fetch error as transient — keep polling until timeout.
				continue
			}

			if !terminalStatuses[status] {
				// Still in flight (queued, sending, etc.) — keep polling.
				continue
			}

			if failedStatuses[status] {
				return c.classifyDeliveryFailure(status, errorMessage, errorCode)
			}

			// delivered, read, or sent — success.
			return nil
		}
	}
}

func (c *TwilioClient) fetchMessageStatus(sid string) (status, errorMessage string, errorCode int, err error) {
	msg, err := c.client.Api.FetchMessage(sid, &twilioApi.FetchMessageParams{})
	if err != nil {
		return "", "", 0, err
	}

	if msg.Status != nil {
		status = *msg.Status
	}
	if msg.ErrorMessage != nil {
		errorMessage = *msg.ErrorMessage
	}
	if msg.ErrorCode != nil {
		errorCode = *msg.ErrorCode
	}

	return status, errorMessage, errorCode, nil
}

func (c *TwilioClient) classifyDeliveryFailure(status, errorMessage string, errorCode int) error {
	msg := errorMessage
	if msg == "" {
		msg = fmt.Sprintf("message %s (code %d)", status, errorCode)
	}

	// Twilio error codes that indicate a permanent, user-side rejection.
	// https://www.twilio.com/docs/api/errors
	permanentCodes := map[int]bool{
		21211: true, // invalid To number
		21408: true, // permission denied
		21610: true, // unsubscribed recipient
		21614: true, // not a valid mobile number
		63003: true, // channel unavailable for recipient
		63016: true, // message blocked
	}

	if permanentCodes[errorCode] {
		return eywa.NewBusinessError(msg, fmt.Errorf("twilio error code %d: %s", errorCode, status))
	}

	return eywa.NewInfrastructureError(msg, fmt.Errorf("twilio error code %d: %s", errorCode, status))
}

func (c *TwilioClient) buildMessageParams(phone, message string, metadata map[string]any) *twilioApi.CreateMessageParams {
	params := &twilioApi.CreateMessageParams{}
	params.SetTo(formatWhatsAppNumber(phone))
	params.SetFrom(c.fromNumber)

	if imageURL, ok := metadata["image_url"].(string); ok && imageURL != "" {
		params.SetMediaUrl([]string{imageURL})
		if message != "" {
			params.SetBody(message)
		}
		return params
	}

	if buttons, ok := metadata["buttons"].([]any); ok && len(buttons) > 0 {
		params.SetBody(formatMessageWithButtons(message, buttons))
		return params
	}

	params.SetBody(message)
	return params
}

func (c *TwilioClient) classifyTwilioError(err error) error {
	type twilioErrorBody struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	var body twilioErrorBody
	if jsonErr := json.Unmarshal([]byte(err.Error()), &body); jsonErr != nil {
		return eywa.NewInfrastructureError("twilio API error", err)
	}

	switch body.Code {
	case 21211:
		return eywa.NewBusinessError("invalid phone number", err)
	case 21408:
		return eywa.NewBusinessError("permission denied to send to this number", err)
	case 21610:
		return eywa.NewBusinessError("recipient has unsubscribed", err)
	case 21614:
		return eywa.NewBusinessError("not a valid mobile number", err)
	case 21910:
		return eywa.NewInfrastructureError("invalid From/To pair — both must use whatsapp: prefix", err)
	case 20003:
		return eywa.NewInfrastructureError("authentication failed", err)
	case 20429:
		return eywa.NewInfrastructureError("rate limit exceeded", err)
	case 20500, 20503:
		return eywa.NewInfrastructureError("twilio service unavailable", err)
	default:
		return eywa.NewInfrastructureError(body.Message, err)
	}
}

// DownloadMedia: mediaID must be "MessageSid/MediaSid" format or a full HTTPS URL.
func (c *TwilioClient) DownloadMedia(ctx context.Context, mediaID string) (data []byte, mimeType string, err error) {
	mediaURL, err := c.resolveMediaURL(mediaID)
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, "", eywa.NewInfrastructureError("failed to create media request", err)
	}
	req.SetBasicAuth(c.accountSID, c.authToken)

	hc := c.httpClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, "", eywa.NewInfrastructureError("failed to download media", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseBytes))
		return nil, "", eywa.NewInfrastructureError(
			fmt.Sprintf("media download failed (HTTP %d): %s", resp.StatusCode, body),
			fmt.Errorf("status code: %d", resp.StatusCode),
		)
	}

	data, err = io.ReadAll(io.LimitReader(resp.Body, maxMediaDownloadBytes))
	if err != nil {
		return nil, "", eywa.NewInfrastructureError("failed to read media body", err)
	}

	mimeType = resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	return data, mimeType, nil
}

func (c *TwilioClient) resolveMediaURL(mediaID string) (string, error) {
	if strings.HasPrefix(mediaID, "https://") {
		return mediaID, nil
	}

	parts := strings.SplitN(mediaID, "/", 2)
	if len(parts) != 2 {
		return "", eywa.NewBusinessError(
			"invalid media ID format — expected \"MessageSid/MediaSid\" or a full URL",
			fmt.Errorf("got: %q", mediaID),
		)
	}

	return fmt.Sprintf(
		"https://api.twilio.com/2010-04-01/Accounts/%s/Messages/%s/Media/%s",
		c.accountSID, parts[0], parts[1],
	), nil
}

func formatWhatsAppNumber(phone string) string {
	if strings.HasPrefix(phone, "whatsapp:") {
		return phone
	}
	return "whatsapp:" + phone
}

// formatMessageWithButtons: fallback because Twilio WhatsApp has limited interactive button support.
func formatMessageWithButtons(message string, buttons []any) string {
	var sb strings.Builder
	sb.WriteString(message)
	sb.WriteString("\n")

	for i, btn := range buttons {
		if m, ok := btn.(map[string]any); ok {
			title, _ := m["title"].(string)
			sb.WriteString(fmt.Sprintf("\n%d. %s", i+1, title))
		}
	}

	return sb.String()
}
