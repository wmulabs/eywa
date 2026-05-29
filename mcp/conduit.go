// Package mcp provides an MCP (Model Context Protocol) Conduit for Eywa.
// It connects to an external MCP server over HTTP (StreamableHTTP transport),
// discovers its tools, and adapts them as Eywa Actions.
//
// Usage:
//
//	conduit := mcp.NewConduit(mcp.ConduitConfig{
//	    Name:      "my-mcp-server",
//	    Transport: "http",
//	    URL:       "https://mcp.example.com",
//	})
//	weave, _ := eywa.NewWeaveBuilder(ctx).
//	    WithConduit(conduit).
//	    Build()
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	eywa "github.com/wmulabs/eywa"
	"github.com/wmulabs/eywa/internal/helpers/netutil"
)

const maxMCPResponseBytes = 4 * 1024 * 1024 // 4 MiB

// ConduitConfig holds the configuration for connecting to an MCP server.
type ConduitConfig struct {
	// Name is the unique identifier for this Conduit in the Eywa registry.
	// Tool names are prefixed: "<Name>__<tool_name>".
	Name string

	// Transport: "http" (StreamableHTTP). Stdio support planned.
	Transport string

	// URL is the MCP server endpoint (HTTP transport).
	URL string

	// Headers are added to every HTTP request (e.g. Authorization).
	Headers map[string]string

	// Timeout for individual MCP calls. Default: 30s.
	Timeout time.Duration
}

type MCPConduit struct {
	cfg          ConduitConfig
	client       *http.Client
	tools        []eywa.ActionDefinition
	reqID        atomic.Int64
	urlValidator func(string) error // defaults to netutil.ValidateURL; overridable in tests
}

func (c *MCPConduit) nextID() int {
	return int(c.reqID.Add(1))
}

func NewConduit(cfg ConduitConfig) eywa.Conduit {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &MCPConduit{
		cfg:          cfg,
		client:       &http.Client{Timeout: cfg.Timeout},
		urlValidator: netutil.ValidateURL,
	}
}

func (c *MCPConduit) GetName() string { return c.cfg.Name }

func (c *MCPConduit) Connect(ctx context.Context) error {
	if c.cfg.Transport != "http" && c.cfg.Transport != "" {
		return fmt.Errorf("mcp conduit: unsupported transport %q (only http supported)", c.cfg.Transport)
	}

	if err := c.urlValidator(c.cfg.URL); err != nil {
		return fmt.Errorf("mcp conduit %q: %w", c.cfg.Name, err)
	}

	if err := c.initialize(ctx); err != nil {
		return fmt.Errorf("mcp conduit %q: initialize failed: %w", c.cfg.Name, err)
	}

	tools, err := c.listTools(ctx)
	if err != nil {
		return fmt.Errorf("mcp conduit %q: list tools failed: %w", c.cfg.Name, err)
	}
	c.tools = tools
	return nil
}

func (c *MCPConduit) ListTools(_ context.Context) ([]eywa.ActionDefinition, error) {
	return c.tools, nil
}

func (c *MCPConduit) Call(ctx context.Context, toolName string, args map[string]any) (string, error) {
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      c.nextID(),
		Method:  "tools/call",
		Params: map[string]any{
			"name":      toolName,
			"arguments": args,
		},
	}

	var resp mcpResponse
	if err := c.post(ctx, req, &resp); err != nil {
		return "", err
	}

	if resp.Error != nil {
		return "", fmt.Errorf("mcp tool %q: %s", toolName, resp.Error.Message)
	}

	return extractTextContent(resp.Result), nil
}

func (c *MCPConduit) Close() error { return nil }

// initialize sends the MCP initialize handshake.
func (c *MCPConduit) initialize(ctx context.Context) error {
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      c.nextID(),
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "eywa",
				"version": "1.0",
			},
		},
	}

	var resp mcpResponse
	return c.post(ctx, req, &resp)
}

// listTools sends tools/list and converts to ActionDefinitions.
func (c *MCPConduit) listTools(ctx context.Context) ([]eywa.ActionDefinition, error) {
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      c.nextID(),
		Method:  "tools/list",
	}

	var resp mcpResponse
	if err := c.post(ctx, req, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list: %s", resp.Error.Message)
	}

	return parseToolList(resp.Result), nil
}

func (c *MCPConduit) post(ctx context.Context, payload mcpRequest, out *mcpResponse) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal mcp request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create mcp request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range c.cfg.Headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("mcp http call: %w", err)
	}
	defer httpResp.Body.Close() //nolint:errcheck

	if httpResp.StatusCode >= 400 {
		return fmt.Errorf("mcp server returned %d", httpResp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(httpResp.Body, maxMCPResponseBytes))
	if err != nil {
		return fmt.Errorf("read mcp response: %w", err)
	}

	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}

// mcpRequest is the JSON-RPC 2.0 request envelope.
type mcpRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// mcpResponse is the JSON-RPC 2.0 response envelope.
type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func parseToolList(raw json.RawMessage) []eywa.ActionDefinition {
	var result struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}

	defs := make([]eywa.ActionDefinition, 0, len(result.Tools))
	for _, t := range result.Tools {
		defs = append(defs, eywa.ActionDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		})
	}
	return defs
}

func extractTextContent(raw json.RawMessage) string {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return string(raw)
	}

	var out string
	for _, c := range result.Content {
		if c.Type == "text" {
			out += c.Text
		}
	}
	return out
}
