package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	eywa "github.com/wmulabs/eywa"
)

// oracle returns an AnthropicOracle with an empty API key (no real calls).
func oracle() *AnthropicOracle {
	return NewOracle("")
}

// --- NewOracle / NewOracleWithConfig / createAnthropicClient ---

func TestNewOracle_SetsDefaults(t *testing.T) {
	o := NewOracle("test-key")
	if o.apiKey != "test-key" {
		t.Errorf("expected apiKey=test-key, got %s", o.apiKey)
	}
	if o.config.MaxRetries != defaultMaxRetries {
		t.Errorf("expected MaxRetries=%d, got %d", defaultMaxRetries, o.config.MaxRetries)
	}
	if o.config.Timeout != defaultTimeout {
		t.Errorf("expected Timeout=%d, got %d", defaultTimeout, o.config.Timeout)
	}
}

func TestNewOracleWithConfig_CustomBaseURL(t *testing.T) {
	o := NewOracleWithConfig(Config{APIKey: "k", BaseURL: "https://proxy.example.com"})
	if o.config.BaseURL != "https://proxy.example.com" {
		t.Errorf("expected custom BaseURL, got %s", o.config.BaseURL)
	}
}

func TestNewOracleWithConfig_ZeroMaxRetries(t *testing.T) {
	// MaxRetries=0 skips the retries option — should not panic.
	o := NewOracleWithConfig(Config{APIKey: "k", MaxRetries: 0})
	if o == nil {
		t.Fatal("expected non-nil oracle")
	}
}

// --- GetName ---

func TestGetName(t *testing.T) {
	if oracle().GetName() != ProviderName {
		t.Errorf("expected %s, got %s", ProviderName, oracle().GetName())
	}
}

// --- GetAvailableModels ---

func TestGetAvailableModels_NonEmpty(t *testing.T) {
	models := oracle().GetAvailableModels()
	if len(models) == 0 {
		t.Error("expected non-empty model list")
	}
	for _, m := range models {
		if m == "" {
			t.Error("found empty model name")
		}
	}
}

// --- IsAvailable ---

func TestIsAvailable_WithKey_True(t *testing.T) {
	if !NewOracle("key").IsAvailable() {
		t.Error("expected IsAvailable=true with api key")
	}
}

func TestIsAvailable_NoKey_False(t *testing.T) {
	if NewOracle("").IsAvailable() {
		t.Error("expected IsAvailable=false with empty api key")
	}
}

// --- GetConfig ---

func TestGetConfig_ContainsExpectedKeys(t *testing.T) {
	o := NewOracleWithConfig(Config{APIKey: "k", BaseURL: "https://x.com", MaxRetries: 2, Timeout: 30})
	cfg := o.GetConfig()
	if cfg["provider"] != ProviderName {
		t.Errorf("expected provider=%s", ProviderName)
	}
	if cfg["has_api_key"] != true {
		t.Error("expected has_api_key=true")
	}
	if cfg["base_url"] != "https://x.com" {
		t.Errorf("expected base_url=https://x.com, got %v", cfg["base_url"])
	}
	if cfg["max_retries"] != 2 {
		t.Errorf("expected max_retries=2, got %v", cfg["max_retries"])
	}
}

// --- SupportsImages ---

func TestSupportsImages_Claude3_True(t *testing.T) {
	if !oracle().SupportsImages("claude-3-5-sonnet-20241022") {
		t.Error("expected true for claude-3")
	}
}

func TestSupportsImages_ClaudeOpus4_True(t *testing.T) {
	if !oracle().SupportsImages("claude-opus-4-5") {
		t.Error("expected true for claude-opus-4")
	}
}

func TestSupportsImages_ClaudeSonnet4_True(t *testing.T) {
	if !oracle().SupportsImages("claude-sonnet-4-6") {
		t.Error("expected true for claude-sonnet-4")
	}
}

func TestSupportsImages_ClaudeHaiku4_True(t *testing.T) {
	if !oracle().SupportsImages("claude-haiku-4-5") {
		t.Error("expected true for claude-haiku-4")
	}
}

func TestSupportsImages_Unknown_False(t *testing.T) {
	if oracle().SupportsImages("gpt-4o") {
		t.Error("expected false for unknown model")
	}
}

func TestSupportsImages_ShortModel_False(t *testing.T) {
	if oracle().SupportsImages("cl") {
		t.Error("expected false for too-short model name")
	}
}

// --- SupportsAudio ---

func TestSupportsAudio_AlwaysFalse(t *testing.T) {
	if oracle().SupportsAudio("any-model") {
		t.Error("expected SupportsAudio=false always")
	}
}

// --- SupportsDocuments ---

func TestSupportsDocuments_DelegatesToImages(t *testing.T) {
	o := oracle()
	if o.SupportsDocuments("claude-3-5-sonnet-20241022") != o.SupportsImages("claude-3-5-sonnet-20241022") {
		t.Error("expected SupportsDocuments to match SupportsImages")
	}
}

// --- normalizeStopReason ---

func TestNormalizeStopReason_AllCases(t *testing.T) {
	o := oracle()
	cases := []struct{ in, want string }{
		{"end_turn", eywa.StopReasonComplete},
		{"max_tokens", eywa.StopReasonLength},
		{"tool_use", eywa.StopReasonToolCalls},
		{"stop_sequence", eywa.StopReasonComplete},
		{"other_reason", "other_reason"},
	}
	for _, c := range cases {
		got := o.normalizeStopReason(c.in)
		if got != c.want {
			t.Errorf("normalizeStopReason(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- extractTokenUsage ---

func TestExtractTokenUsage(t *testing.T) {
	usage := anthropic.Usage{InputTokens: 100, OutputTokens: 50}
	got := oracle().extractTokenUsage(usage)
	if got.PromptTokens != 100 {
		t.Errorf("expected PromptTokens=100, got %d", got.PromptTokens)
	}
	if got.CompletionTokens != 50 {
		t.Errorf("expected CompletionTokens=50, got %d", got.CompletionTokens)
	}
	if got.TotalTokens != 150 {
		t.Errorf("expected TotalTokens=150, got %d", got.TotalTokens)
	}
}

// --- addSystemPrompt ---

func TestAddSystemPrompt_Empty_NoChange(t *testing.T) {
	params := anthropic.MessageNewParams{}
	oracle().addSystemPrompt(&params, "")
	if len(params.System) != 0 {
		t.Error("expected System to remain empty")
	}
}

func TestAddSystemPrompt_NonEmpty_SetsSystem(t *testing.T) {
	params := anthropic.MessageNewParams{}
	oracle().addSystemPrompt(&params, "you are helpful")
	if len(params.System) != 1 || params.System[0].Text != "you are helpful" {
		t.Errorf("unexpected System: %+v", params.System)
	}
}

// --- addSamplingParameters ---

func TestAddSamplingParameters_AllFields(t *testing.T) {
	params := anthropic.MessageNewParams{}
	req := &eywa.OracleRequest{TopP: 0.9, TopK: 40, StopSequences: []string{"STOP"}}
	oracle().addSamplingParameters(&params, req)
	if params.TopP.Value != 0.9 {
		t.Errorf("expected TopP=0.9, got %v", params.TopP)
	}
	if params.TopK.Value != 40 {
		t.Errorf("expected TopK=40, got %v", params.TopK)
	}
	if len(params.StopSequences) != 1 || params.StopSequences[0] != "STOP" {
		t.Errorf("unexpected StopSequences: %v", params.StopSequences)
	}
}

func TestAddSamplingParameters_ZeroValues_NoChange(t *testing.T) {
	params := anthropic.MessageNewParams{}
	oracle().addSamplingParameters(&params, &eywa.OracleRequest{})
	if params.TopP.Value != 0 || params.TopK.Value != 0 || len(params.StopSequences) != 0 {
		t.Error("expected no change for zero-value request")
	}
}

// --- addTools ---

func TestAddTools_UseToolsFalse_NoTools(t *testing.T) {
	params := anthropic.MessageNewParams{}
	oracle().addTools(&params, &eywa.OracleRequest{UseTools: false, Tools: []eywa.OracleTool{{Name: "fn"}}})
	if len(params.Tools) != 0 {
		t.Error("expected no tools when UseTools=false")
	}
}

func TestAddTools_EmptyTools_NoChange(t *testing.T) {
	params := anthropic.MessageNewParams{}
	oracle().addTools(&params, &eywa.OracleRequest{UseTools: true, Tools: nil})
	if len(params.Tools) != 0 {
		t.Error("expected no tools for empty tool list")
	}
}

func TestAddTools_WithTools_SetsTools(t *testing.T) {
	params := anthropic.MessageNewParams{}
	req := &eywa.OracleRequest{
		UseTools: true,
		Tools:    []eywa.OracleTool{{Name: "search", Description: "Search"}},
	}
	oracle().addTools(&params, req)
	if len(params.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(params.Tools))
	}
}

// --- convertRole ---

func TestConvertRole_Tool_ReturnsUser(t *testing.T) {
	got := oracle().convertRole(eywa.RoleTool)
	if got != anthropic.MessageParamRoleUser {
		t.Errorf("expected user role for tool, got %s", got)
	}
}

func TestConvertRole_User_ReturnsUser(t *testing.T) {
	got := oracle().convertRole(eywa.RoleUser)
	if string(got) != eywa.RoleUser {
		t.Errorf("expected user, got %s", got)
	}
}

func TestConvertRole_Assistant_ReturnsAssistant(t *testing.T) {
	got := oracle().convertRole(eywa.RoleAssistant)
	if string(got) != eywa.RoleAssistant {
		t.Errorf("expected assistant, got %s", got)
	}
}

// --- extractContentAndToolCalls ---

func TestExtractContentAndToolCalls_TextOnly(t *testing.T) {
	blocks := []anthropic.ContentBlockUnion{
		{Type: "text", Text: "hello world"},
	}
	content, calls, err := oracle().extractContentAndToolCalls(blocks)
	if err != nil || content != "hello world" || len(calls) != 0 {
		t.Errorf("unexpected: content=%q calls=%v err=%v", content, calls, err)
	}
}

func TestExtractContentAndToolCalls_ToolUse_ValidInput(t *testing.T) {
	args, _ := json.Marshal(map[string]any{"q": "golang"})
	blocks := []anthropic.ContentBlockUnion{
		{Type: "tool_use", ID: "tc1", Name: "search", Input: args},
	}
	_, calls, err := oracle().extractContentAndToolCalls(blocks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 1 || calls[0].Name != "search" || calls[0].ID != "tc1" {
		t.Errorf("unexpected tool calls: %v", calls)
	}
}

func TestExtractContentAndToolCalls_ToolUse_InvalidInput_ReturnsError(t *testing.T) {
	blocks := []anthropic.ContentBlockUnion{
		{Type: "tool_use", ID: "tc1", Name: "fn", Input: json.RawMessage("not-json")},
	}
	_, _, err := oracle().extractContentAndToolCalls(blocks)
	if err == nil {
		t.Error("expected error for invalid tool input JSON")
	}
}

func TestExtractContentAndToolCalls_MultipleBlocks(t *testing.T) {
	args1, _ := json.Marshal(map[string]any{"a": 1})
	args2, _ := json.Marshal(map[string]any{"b": 2})
	blocks := []anthropic.ContentBlockUnion{
		{Type: "text", Text: "thinking..."},
		{Type: "tool_use", ID: "t1", Name: "fn1", Input: args1},
		{Type: "tool_use", ID: "t2", Name: "fn2", Input: args2},
	}
	content, calls, err := oracle().extractContentAndToolCalls(blocks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "thinking..." || len(calls) != 2 {
		t.Errorf("unexpected: content=%q calls=%v", content, calls)
	}
}

// --- convertToolCallsToBlocks ---

func TestConvertToolCallsToBlocks(t *testing.T) {
	calls := []eywa.OracleToolCall{
		{ID: "tc1", Name: "fn1", Arguments: map[string]any{"x": 1}},
		{ID: "tc2", Name: "fn2", Arguments: map[string]any{"y": 2}},
	}
	blocks := oracle().convertToolCallsToBlocks(calls)
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks))
	}
}

// --- createToolResultBlock ---

func TestCreateToolResultBlock_WithToolCallID(t *testing.T) {
	msg := eywa.OracleMessage{Role: eywa.RoleTool, ToolCallID: "tc1", Content: "result", IsError: false}
	block := oracle().createToolResultBlock(msg)
	// OfToolResult should be set
	if block.OfToolResult == nil {
		t.Error("expected OfToolResult to be set")
	}
	if block.OfToolResult.ToolUseID != "tc1" {
		t.Errorf("expected ToolUseID=tc1, got %s", block.OfToolResult.ToolUseID)
	}
}

func TestCreateToolResultBlock_FallsBackToToolName(t *testing.T) {
	msg := eywa.OracleMessage{Role: eywa.RoleTool, ToolCallID: "", ToolName: "search", Content: "ok"}
	block := oracle().createToolResultBlock(msg)
	if block.OfToolResult == nil || block.OfToolResult.ToolUseID != "search" {
		t.Errorf("expected ToolUseID=search, got %v", block.OfToolResult)
	}
}

// --- buildContentBlocks ---

func TestBuildContentBlocks_TextContent(t *testing.T) {
	msg := eywa.OracleMessage{Role: eywa.RoleUser, Content: "hello"}
	blocks := oracle().buildContentBlocks(msg, &eywa.OracleRequest{})
	if len(blocks) != 1 {
		t.Errorf("expected 1 block for text content, got %d", len(blocks))
	}
}

func TestBuildContentBlocks_EmptyContent_NoBlocks(t *testing.T) {
	msg := eywa.OracleMessage{Role: eywa.RoleUser, Content: ""}
	blocks := oracle().buildContentBlocks(msg, &eywa.OracleRequest{})
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks for empty content, got %d", len(blocks))
	}
}

func TestBuildContentBlocks_WithToolCalls(t *testing.T) {
	msg := eywa.OracleMessage{
		Role:      eywa.RoleAssistant,
		ToolCalls: []eywa.OracleToolCall{{ID: "t1", Name: "fn", Arguments: map[string]any{}}},
	}
	blocks := oracle().buildContentBlocks(msg, &eywa.OracleRequest{})
	if len(blocks) != 1 {
		t.Errorf("expected 1 block for tool call, got %d", len(blocks))
	}
}

func TestBuildContentBlocks_ToolRole_WithContent_AddsResultBlock(t *testing.T) {
	msg := eywa.OracleMessage{Role: eywa.RoleTool, Content: "result", ToolCallID: "tc1"}
	blocks := oracle().buildContentBlocks(msg, &eywa.OracleRequest{})
	// Content is non-empty → text block added; RoleTool + Content != "" → tool result block added.
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks (text + tool result), got %d", len(blocks))
	}
}

func TestBuildContentBlocks_UserWithAttachments(t *testing.T) {
	msg := eywa.OracleMessage{Role: eywa.RoleUser, Content: "look at this"}
	req := &eywa.OracleRequest{
		Attachments: []eywa.LLMAttachment{
			{Type: eywa.ArtifactTypeImage, Data: []byte{0xFF, 0xD8}, MimeType: "image/jpeg"},
		},
	}
	blocks := oracle().buildContentBlocks(msg, req)
	if len(blocks) != 2 { // text + image
		t.Errorf("expected 2 blocks (text+image), got %d", len(blocks))
	}
}

// --- convertMessages ---

func TestConvertMessages_SkipsEmptyBlocks(t *testing.T) {
	req := &eywa.OracleRequest{
		Messages: []eywa.OracleMessage{
			{Role: eywa.RoleUser, Content: ""},   // empty → no blocks → skipped
			{Role: eywa.RoleUser, Content: "hi"}, // non-empty → kept
		},
	}
	msgs := oracle().convertMessages(req)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message (empty skipped), got %d", len(msgs))
	}
}

// --- convertTools / createToolParam / buildInputSchema ---

func TestConvertTools_ConvertsAll(t *testing.T) {
	tools := []eywa.OracleTool{
		{Name: "search", Description: "Search web", Parameters: map[string]any{"type": "object"}},
		{Name: "calc"},
	}
	result := oracle().convertTools(tools)
	if len(result) != 2 {
		t.Errorf("expected 2 tools, got %d", len(result))
	}
}

func TestCreateToolParam_WithDescription(t *testing.T) {
	tool := eywa.OracleTool{Name: "fn", Description: "Does stuff", Parameters: map[string]any{}}
	param := oracle().createToolParam(tool)
	if param.Name != "fn" {
		t.Errorf("expected Name=fn, got %s", param.Name)
	}
	if param.Description.Value != "Does stuff" {
		t.Errorf("expected description set, got %v", param.Description)
	}
}

func TestCreateToolParam_NoDescription(t *testing.T) {
	tool := eywa.OracleTool{Name: "fn", Parameters: map[string]any{}}
	param := oracle().createToolParam(tool)
	if param.Description.Value != "" {
		t.Errorf("expected empty description, got %v", param.Description)
	}
}

func TestBuildInputSchema_WithProperties(t *testing.T) {
	params := map[string]any{
		"type":       "object",
		"properties": map[string]any{"q": map[string]any{"type": "string"}},
		"required":   []string{"q"},
	}
	schema := oracle().buildInputSchema(params)
	if schema.Type != "object" {
		t.Errorf("expected type=object, got %s", schema.Type)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "q" {
		t.Errorf("expected required=[q], got %v", schema.Required)
	}
}

func TestBuildInputSchema_NoProperties(t *testing.T) {
	schema := oracle().buildInputSchema(map[string]any{})
	if schema.Type != "object" {
		t.Errorf("expected type=object")
	}
}

// --- extractRequiredFields ---

func TestExtractRequiredFields_StringSlice(t *testing.T) {
	params := map[string]any{"required": []string{"a", "b"}}
	got := oracle().extractRequiredFields(params)
	if len(got) != 2 || got[0] != "a" {
		t.Errorf("unexpected: %v", got)
	}
}

func TestExtractRequiredFields_InterfaceSlice(t *testing.T) {
	params := map[string]any{"required": []any{"x", "y"}}
	got := oracle().extractRequiredFields(params)
	if len(got) != 2 || got[0] != "x" {
		t.Errorf("unexpected: %v", got)
	}
}

func TestExtractRequiredFields_Missing_ReturnsNil(t *testing.T) {
	if oracle().extractRequiredFields(map[string]any{}) != nil {
		t.Error("expected nil for missing required")
	}
}

func TestExtractRequiredFields_UnknownType_ReturnsNil(t *testing.T) {
	params := map[string]any{"required": 42}
	if oracle().extractRequiredFields(params) != nil {
		t.Error("expected nil for unknown type")
	}
}

// --- convertInterfaceSliceToStringSlice ---

func TestConvertInterfaceSliceToStringSlice_FiltersNonStrings(t *testing.T) {
	got := oracle().convertInterfaceSliceToStringSlice([]any{"a", 1, "b", nil})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("expected [a b], got %v", got)
	}
}

// --- isImageAttachment ---

func TestIsImageAttachment_ImageType_True(t *testing.T) {
	if !oracle().isImageAttachment(eywa.LLMAttachment{Type: eywa.ArtifactTypeImage}) {
		t.Error("expected true for image type")
	}
}

func TestIsImageAttachment_NonImage_False(t *testing.T) {
	if oracle().isImageAttachment(eywa.LLMAttachment{Type: "document"}) {
		t.Error("expected false for non-image type")
	}
}

// --- createImageBlock ---

func TestCreateImageBlock_WithData_ReturnsBase64Block(t *testing.T) {
	att := eywa.LLMAttachment{Type: eywa.ArtifactTypeImage, Data: []byte{0xFF, 0xD8}, MimeType: "image/jpeg"}
	block := oracle().createImageBlock(att)
	if block == nil || block.OfImage == nil {
		t.Fatal("expected image block")
	}
	if block.OfImage.Source.OfBase64 == nil {
		t.Error("expected base64 source")
	}
}

func TestCreateImageBlock_WithURL_ReturnsURLBlock(t *testing.T) {
	att := eywa.LLMAttachment{Type: eywa.ArtifactTypeImage, URL: "https://example.com/img.jpg"}
	block := oracle().createImageBlock(att)
	if block == nil || block.OfImage == nil {
		t.Fatal("expected image block")
	}
	if block.OfImage.Source.OfURL == nil {
		t.Error("expected URL source")
	}
}

func TestCreateImageBlock_NoDataNoURL_ReturnsNil(t *testing.T) {
	att := eywa.LLMAttachment{Type: eywa.ArtifactTypeImage}
	if oracle().createImageBlock(att) != nil {
		t.Error("expected nil when no data and no URL")
	}
}

// --- determineImageMediaType ---

func TestDetermineImageMediaType_AllCases(t *testing.T) {
	o := oracle()
	cases := []struct {
		mime string
		want anthropic.Base64ImageSourceMediaType
	}{
		{"", anthropic.Base64ImageSourceMediaTypeImageJPEG},
		{"image/jpeg", anthropic.Base64ImageSourceMediaTypeImageJPEG},
		{"image/jpg", anthropic.Base64ImageSourceMediaTypeImageJPEG},
		{"image/png", anthropic.Base64ImageSourceMediaTypeImagePNG},
		{"image/gif", anthropic.Base64ImageSourceMediaTypeImageGIF},
		{"image/webp", anthropic.Base64ImageSourceMediaTypeImageWebP},
		{"image/bmp", anthropic.Base64ImageSourceMediaTypeImageJPEG}, // default
	}
	for _, c := range cases {
		got := o.determineImageMediaType(c.mime)
		if got != c.want {
			t.Errorf("determineImageMediaType(%q) = %v, want %v", c.mime, got, c.want)
		}
	}
}

// --- convertAttachmentsToImageBlocks ---

func TestConvertAttachmentsToImageBlocks_SkipsNonImage(t *testing.T) {
	atts := []eywa.LLMAttachment{
		{Type: "document", URL: "https://example.com/doc.pdf"},
		{Type: eywa.ArtifactTypeImage, URL: "https://example.com/img.jpg"},
	}
	blocks := oracle().convertAttachmentsToImageBlocks(atts)
	if len(blocks) != 1 {
		t.Errorf("expected 1 image block (document skipped), got %d", len(blocks))
	}
}

func TestConvertAttachmentsToImageBlocks_NilDataNilURL_Skipped(t *testing.T) {
	atts := []eywa.LLMAttachment{
		{Type: eywa.ArtifactTypeImage}, // no data, no URL → createImageBlock returns nil
	}
	blocks := oracle().convertAttachmentsToImageBlocks(atts)
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks for attachment with no data/URL, got %d", len(blocks))
	}
}

// --- buildRequestParams ---

func TestBuildRequestParams_SetsFields(t *testing.T) {
	req := &eywa.OracleRequest{
		Model:        "claude-3-5-sonnet-20241022",
		MaxTokens:    1024,
		Temperature:  0.7,
		SystemPrompt: "you are helpful",
		TopP:         0.9,
		Messages: []eywa.OracleMessage{
			{Role: eywa.RoleUser, Content: "hi"},
		},
	}
	params := oracle().buildRequestParams(req)
	if params.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("unexpected model: %s", params.Model)
	}
	if params.MaxTokens != 1024 {
		t.Errorf("unexpected MaxTokens: %d", params.MaxTokens)
	}
	if len(params.System) != 1 {
		t.Errorf("expected 1 system block, got %d", len(params.System))
	}
}

// --- parseResponse (via extractContentAndToolCalls + extractTokenUsage) ---

func TestParseResponse_TextResponse(t *testing.T) {
	resp := &anthropic.Message{
		Content:    []anthropic.ContentBlockUnion{{Type: "text", Text: "pong"}},
		StopReason: "end_turn",
		Usage:      anthropic.Usage{InputTokens: 10, OutputTokens: 5},
	}
	result, err := oracle().parseResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "pong" {
		t.Errorf("expected Content=pong, got %s", result.Content)
	}
	if result.StopReason != eywa.StopReasonComplete {
		t.Errorf("expected stop=stop, got %s", result.StopReason)
	}
	if result.TokensUsed.TotalTokens != 15 {
		t.Errorf("expected TotalTokens=15, got %d", result.TokensUsed.TotalTokens)
	}
}

func TestParseResponse_InvalidToolInput_ReturnsError(t *testing.T) {
	resp := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "tool_use", ID: "t1", Name: "fn", Input: json.RawMessage("not-json")},
		},
	}
	if _, err := oracle().parseResponse(resp); err == nil {
		t.Error("expected error for invalid tool input")
	}
}
