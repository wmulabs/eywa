package orchestrator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers"
	"github.com/wmulabs/eywa/internal/helpers/netutil"
)

var _ ports.Action = (*HTTPToolExecutor)(nil)

type HTTPToolExecutor struct {
	definition entities.HTTPTool
	httpClient *http.Client
}

func NewHTTPToolExecutor(tool entities.HTTPTool) *HTTPToolExecutor {
	timeout := time.Duration(tool.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPToolExecutor{
		definition: tool,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (e *HTTPToolExecutor) GetName() string                   { return e.definition.Name }
func (e *HTTPToolExecutor) GetDescription() string            { return e.definition.Description }
func (e *HTTPToolExecutor) IsCritical() bool                  { return e.definition.IsCritical }
func (e *HTTPToolExecutor) GetCategory() ports.ActionCategory { return ports.ActionGeneral }

func (e *HTTPToolExecutor) GetParameters() map[string]any {
	properties := make(map[string]any)
	required := []string{}
	for _, p := range e.definition.Parameters {
		properties[p.Name] = map[string]any{
			"type":        p.Type,
			"description": p.Description,
		}
		if p.Required {
			required = append(required, p.Name)
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func (e *HTTPToolExecutor) Validate(args map[string]any) error {
	for _, p := range e.definition.Parameters {
		if p.Required {
			if _, ok := args[p.Name]; !ok {
				return fmt.Errorf("required parameter missing: %s", p.Name)
			}
		}
	}
	return nil
}

func (e *HTTPToolExecutor) Execute(ctx context.Context, args map[string]any) (string, error) {
	resolvedURL := resolveTemplate(e.definition.URL, args)
	if err := validateURL(resolvedURL); err != nil {
		return "", err
	}
	return e.execute(ctx, resolvedURL, args)
}

func (e *HTTPToolExecutor) execute(ctx context.Context, resolvedURL string, args map[string]any) (string, error) {
	resolvedBody := resolveTemplate(e.definition.BodyTemplate, args)

	var bodyReader io.Reader
	if resolvedBody != "" {
		bodyReader = strings.NewReader(resolvedBody)
	}

	req, err := http.NewRequestWithContext(ctx, e.definition.Method, resolvedURL, bodyReader)
	if err != nil {
		return "", domainerrors.NewInfrastructureError(err.Error())
	}
	for k, v := range e.definition.Headers {
		req.Header.Set(k, resolveTemplate(v, args))
	}
	if resolvedBody != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", domainerrors.NewInfrastructureError(err.Error())
	}
	defer resp.Body.Close()

	limit := int64(e.definition.MaxResponseBytes)
	if limit <= 0 {
		limit = 1 << 20 // 1 MiB default
	}
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, limit))

	if actionErr := helpers.FromHTTPStatus(resp.StatusCode, string(responseBody)); actionErr != nil {
		return "", actionErr
	}
	return string(responseBody), nil
}

// HTTPToolTestResult holds diagnostic data returned by the management live-test endpoint.
type HTTPToolTestResult struct {
	StatusCode  int
	Response    string
	ResolvedURL string
	LatencyMS   int64
}

// Test executes the HTTP call and returns richer diagnostic info than Execute.
// Used exclusively by the management API test endpoint — not part of ports.Action.
func (e *HTTPToolExecutor) Test(ctx context.Context, args map[string]any) (*HTTPToolTestResult, error) {
	resolvedURL := resolveTemplate(e.definition.URL, args)
	if err := validateURL(resolvedURL); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}
	return e.test(ctx, resolvedURL, args)
}

func (e *HTTPToolExecutor) test(ctx context.Context, resolvedURL string, args map[string]any) (*HTTPToolTestResult, error) {
	resolvedBody := resolveTemplate(e.definition.BodyTemplate, args)

	var bodyReader io.Reader
	if resolvedBody != "" {
		bodyReader = strings.NewReader(resolvedBody)
	}

	req, err := http.NewRequestWithContext(ctx, e.definition.Method, resolvedURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	for k, v := range e.definition.Headers {
		req.Header.Set(k, resolveTemplate(v, args))
	}
	if resolvedBody != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := e.httpClient.Do(req)
	latencyMS := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	limit := int64(e.definition.MaxResponseBytes)
	if limit <= 0 {
		limit = 1 << 20 // 1 MiB default
	}
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, limit))

	return &HTTPToolTestResult{
		StatusCode:  resp.StatusCode,
		Response:    string(responseBody),
		ResolvedURL: resolvedURL,
		LatencyMS:   latencyMS,
	}, nil
}

// resolveTemplate replaces {{key}} placeholders with values from args.
func resolveTemplate(template string, args map[string]any) string {
	result := template
	for k, v := range args {
		result = strings.ReplaceAll(result, "{{"+k+"}}", fmt.Sprintf("%v", v))
	}
	return result
}

// validateURL checks that the resolved URL is safe to request via netutil.ValidateURL,
// wrapping any error as an infrastructure error.
func validateURL(rawURL string) error {
	if err := netutil.ValidateURL(rawURL); err != nil {
		return domainerrors.NewInfrastructureError(err.Error())
	}
	return nil
}
