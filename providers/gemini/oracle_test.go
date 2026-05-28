package gemini

import (
	"testing"

	"google.golang.org/genai"

	eywa "github.com/wmulabs/eywa"
)

// newOracle creates an oracle with a fake key (no real API calls made).
func newOracle(t *testing.T) *GeminiOracle {
	t.Helper()
	p, err := NewOracle("test-key")
	if err != nil {
		t.Fatalf("NewOracle failed: %v", err)
	}
	return p
}

// --- constructors ---

func TestNewOracle_ReturnsNonNil(t *testing.T) {
	p, err := NewOracle("test-key")
	if err != nil || p == nil {
		t.Fatalf("expected non-nil oracle, err=%v", err)
	}
}

func TestNewOracle_DefaultMaxRetries(t *testing.T) {
	p := newOracle(t)
	if p.config.MaxRetries != defaultMaxRetries {
		t.Errorf("expected %d, got %d", defaultMaxRetries, p.config.MaxRetries)
	}
}

func TestNewOracle_DefaultTimeout(t *testing.T) {
	p := newOracle(t)
	if p.config.Timeout != defaultTimeout {
		t.Errorf("expected %d, got %d", defaultTimeout, p.config.Timeout)
	}
}

func TestNewOracleWithConfig_EmptyAPIKey_UsesGeminiBackend(t *testing.T) {
	p, err := NewOracleWithConfig(Config{APIKey: "k"})
	if err != nil || p == nil {
		t.Fatalf("unexpected: err=%v", err)
	}
}

func TestNewOracleWithConfig_WithProject_UsesVertexBackend(t *testing.T) {
	// Project set → Vertex AI backend (no ADC needed for client creation)
	p, err := NewOracleWithConfig(Config{Project: "my-project", Location: "us-central1"})
	if err != nil || p == nil {
		t.Fatalf("unexpected: err=%v", err)
	}
}

func TestNewVertexOracle_ReturnsNonNil(t *testing.T) {
	p, err := NewVertexOracle(t.Context(), "my-project", "us-central1")
	if err != nil || p == nil {
		t.Fatalf("unexpected: err=%v", err)
	}
	if p.GetName() != "vertexai" {
		t.Errorf("expected vertexai, got %s", p.GetName())
	}
}

// --- GetName ---

func TestGetName_DefaultProviderName(t *testing.T) {
	p := newOracle(t)
	if p.GetName() != ProviderName {
		t.Errorf("expected %q, got %q", ProviderName, p.GetName())
	}
}

func TestGetName_CustomName(t *testing.T) {
	p := &GeminiOracle{config: Config{Name: "vertexai"}}
	if p.GetName() != "vertexai" {
		t.Errorf("expected vertexai, got %s", p.GetName())
	}
}

func TestGetName_EmptyName_FallsBackToProviderName(t *testing.T) {
	p := &GeminiOracle{config: Config{}}
	if p.GetName() != ProviderName {
		t.Errorf("expected %q, got %q", ProviderName, p.GetName())
	}
}

// --- GetAvailableModels ---

func TestGetAvailableModels_NonEmpty(t *testing.T) {
	p := newOracle(t)
	if len(p.GetAvailableModels()) == 0 {
		t.Error("expected non-empty model list")
	}
}

// --- IsAvailable ---

func TestIsAvailable_WithClient_True(t *testing.T) {
	p := newOracle(t)
	if !p.IsAvailable() {
		t.Error("expected IsAvailable=true")
	}
}

func TestIsAvailable_NilClient_False(t *testing.T) {
	p := &GeminiOracle{client: nil}
	if p.IsAvailable() {
		t.Error("expected IsAvailable=false for nil client")
	}
}

// --- GetConfig ---

func TestGetConfig_ContainsExpectedKeys(t *testing.T) {
	p, _ := NewOracleWithConfig(Config{APIKey: "mykey", BaseURL: "http://x"})
	cfg := p.GetConfig()
	if cfg["provider"] != ProviderName {
		t.Errorf("unexpected provider: %v", cfg["provider"])
	}
	if cfg["has_api_key"] != true {
		t.Errorf("expected has_api_key=true, got %v", cfg["has_api_key"])
	}
}

func TestGetConfig_NoAPIKey(t *testing.T) {
	p := &GeminiOracle{config: Config{}}
	cfg := p.GetConfig()
	if cfg["has_api_key"] != false {
		t.Errorf("expected has_api_key=false, got %v", cfg["has_api_key"])
	}
}

// --- SupportsImages / SupportsAudio / SupportsDocuments ---

func TestSupportsImages_Gemini15_True(t *testing.T) {
	p := &GeminiOracle{}
	if !p.SupportsImages("gemini-1.5-pro") {
		t.Error("gemini-1.5 should support images")
	}
}

func TestSupportsImages_Gemini2_True(t *testing.T) {
	p := &GeminiOracle{}
	if !p.SupportsImages("gemini-2.5-flash") {
		t.Error("gemini-2 should support images")
	}
}

func TestSupportsImages_Gemini3_True(t *testing.T) {
	p := &GeminiOracle{}
	if !p.SupportsImages("gemini-3.1-pro") {
		t.Error("gemini-3 should support images")
	}
}

func TestSupportsImages_OtherModel_False(t *testing.T) {
	p := &GeminiOracle{}
	if p.SupportsImages("gemini-1.0-pro") {
		t.Error("gemini-1.0 should not support images")
	}
}

func TestSupportsImages_EmptyModel_False(t *testing.T) {
	p := &GeminiOracle{}
	if p.SupportsImages("") {
		t.Error("empty model should not support images")
	}
}

func TestSupportsAudio_DelegatesToImages(t *testing.T) {
	p := &GeminiOracle{}
	if p.SupportsAudio("gemini-1.5-pro") != p.SupportsImages("gemini-1.5-pro") {
		t.Error("SupportsAudio should match SupportsImages")
	}
}

func TestSupportsDocuments_DelegatesToImages(t *testing.T) {
	p := &GeminiOracle{}
	if p.SupportsDocuments("gemini-2.5-flash") != p.SupportsImages("gemini-2.5-flash") {
		t.Error("SupportsDocuments should match SupportsImages")
	}
}

// --- normalizeStopReason ---

func TestNormalizeStopReason_Stop(t *testing.T) {
	p := &GeminiOracle{}
	if got := p.normalizeStopReason(genai.FinishReasonStop); got != eywa.StopReasonComplete {
		t.Errorf("expected %q, got %q", eywa.StopReasonComplete, got)
	}
}

func TestNormalizeStopReason_MaxTokens(t *testing.T) {
	p := &GeminiOracle{}
	if got := p.normalizeStopReason(genai.FinishReasonMaxTokens); got != eywa.StopReasonLength {
		t.Errorf("expected %q, got %q", eywa.StopReasonLength, got)
	}
}

func TestNormalizeStopReason_Safety(t *testing.T) {
	p := &GeminiOracle{}
	if got := p.normalizeStopReason(genai.FinishReasonSafety); got != eywa.StopReasonContentFilter {
		t.Errorf("expected %q, got %q", eywa.StopReasonContentFilter, got)
	}
}

func TestNormalizeStopReason_Recitation(t *testing.T) {
	p := &GeminiOracle{}
	if got := p.normalizeStopReason(genai.FinishReasonRecitation); got != eywa.StopReasonContentFilter {
		t.Errorf("expected %q, got %q", eywa.StopReasonContentFilter, got)
	}
}

func TestNormalizeStopReason_Unknown_DefaultsToComplete(t *testing.T) {
	p := &GeminiOracle{}
	if got := p.normalizeStopReason(genai.FinishReasonUnspecified); got != eywa.StopReasonComplete {
		t.Errorf("expected complete default, got %q", got)
	}
}

// --- extractTokenUsage ---

func TestExtractTokenUsage_NilMetadata_ReturnsZero(t *testing.T) {
	p := &GeminiOracle{}
	got := p.extractTokenUsage(nil)
	if got.PromptTokens != 0 || got.CompletionTokens != 0 || got.TotalTokens != 0 {
		t.Errorf("expected zero usage, got %+v", got)
	}
}

func TestExtractTokenUsage_MapsFields(t *testing.T) {
	p := &GeminiOracle{}
	meta := &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:     100,
		CandidatesTokenCount: 50,
		TotalTokenCount:      150,
	}
	got := p.extractTokenUsage(meta)
	if got.PromptTokens != 100 || got.CompletionTokens != 50 || got.TotalTokens != 150 {
		t.Errorf("unexpected usage: %+v", got)
	}
}

// --- validateResponse ---

func TestValidateResponse_EmptyCandidates_ReturnsError(t *testing.T) {
	p := &GeminiOracle{}
	err := p.validateResponse(&genai.GenerateContentResponse{})
	if err == nil {
		t.Error("expected error for empty candidates")
	}
}

func TestValidateResponse_WithCandidates_NoError(t *testing.T) {
	p := &GeminiOracle{}
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{Content: &genai.Content{Parts: []*genai.Part{{Text: "hi"}}}},
		},
	}
	if err := p.validateResponse(resp); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- convertRole ---

func TestConvertRole_Assistant_ReturnsModel(t *testing.T) {
	p := &GeminiOracle{}
	if got := p.convertRole(eywa.RoleAssistant); got != roleModel {
		t.Errorf("expected %q, got %q", roleModel, got)
	}
}

func TestConvertRole_User_ReturnsUser(t *testing.T) {
	p := &GeminiOracle{}
	if got := p.convertRole(eywa.RoleUser); got != roleUser {
		t.Errorf("expected %q, got %q", roleUser, got)
	}
}

func TestConvertRole_Other_ReturnsUser(t *testing.T) {
	p := &GeminiOracle{}
	if got := p.convertRole("unknown"); got != roleUser {
		t.Errorf("expected roleUser fallback, got %q", got)
	}
}

// --- buildMessageParts ---

func TestBuildMessageParts_ToolRole_ReturnsFunctionResponse(t *testing.T) {
	p := &GeminiOracle{}
	msg := eywa.OracleMessage{
		Role:     eywa.RoleTool,
		Content:  "result",
		ToolName: "search",
	}
	parts := p.buildMessageParts(msg)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].FunctionResponse == nil {
		t.Error("expected FunctionResponse part")
	}
	if parts[0].FunctionResponse.Name != "search" {
		t.Errorf("unexpected name: %s", parts[0].FunctionResponse.Name)
	}
}

func TestBuildMessageParts_UserWithContent(t *testing.T) {
	p := &GeminiOracle{}
	msg := eywa.OracleMessage{Role: eywa.RoleUser, Content: "hello"}
	parts := p.buildMessageParts(msg)
	if len(parts) != 1 || parts[0].Text != "hello" {
		t.Errorf("unexpected parts: %+v", parts)
	}
}

func TestBuildMessageParts_EmptyContent_NoParts(t *testing.T) {
	p := &GeminiOracle{}
	msg := eywa.OracleMessage{Role: eywa.RoleUser, Content: ""}
	parts := p.buildMessageParts(msg)
	if len(parts) != 0 {
		t.Errorf("expected 0 parts, got %d", len(parts))
	}
}

func TestBuildMessageParts_WithToolCalls(t *testing.T) {
	p := &GeminiOracle{}
	msg := eywa.OracleMessage{
		Role:    eywa.RoleAssistant,
		Content: "calling",
		ToolCalls: []eywa.OracleToolCall{
			{ID: "tc-1", Name: "search", Arguments: map[string]any{"q": "test"}},
		},
	}
	parts := p.buildMessageParts(msg)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts (text+function_call), got %d", len(parts))
	}
	if parts[1].FunctionCall == nil {
		t.Error("expected FunctionCall part")
	}
}

// --- convertMessageToContent ---

func TestConvertMessageToContent_EmptyParts_ReturnsNil(t *testing.T) {
	p := &GeminiOracle{}
	msg := eywa.OracleMessage{Role: eywa.RoleUser, Content: ""}
	result := p.convertMessageToContent(msg)
	if result != nil {
		t.Error("expected nil for empty parts")
	}
}

func TestConvertMessageToContent_WithContent_ReturnsContent(t *testing.T) {
	p := &GeminiOracle{}
	msg := eywa.OracleMessage{Role: eywa.RoleUser, Content: "hi"}
	result := p.convertMessageToContent(msg)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.Role != roleUser {
		t.Errorf("unexpected role: %s", result.Role)
	}
}

// --- convertToHistory ---

func TestConvertToHistory_MergesConsecutiveToolMessages(t *testing.T) {
	p := &GeminiOracle{}
	msgs := []eywa.OracleMessage{
		{Role: eywa.RoleUser, Content: "ask"},
		{Role: eywa.RoleTool, Content: "result1", ToolName: "tool1"},
		{Role: eywa.RoleTool, Content: "result2", ToolName: "tool2"},
	}
	history := p.convertToHistory(msgs)
	// user + merged tool batch (as user)
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	// Merged tool batch has 2 parts
	if len(history[1].Parts) != 2 {
		t.Errorf("expected 2 parts in merged tool turn, got %d", len(history[1].Parts))
	}
}

func TestConvertToHistory_SkipsEmptyMessages(t *testing.T) {
	p := &GeminiOracle{}
	msgs := []eywa.OracleMessage{
		{Role: eywa.RoleUser, Content: ""},
		{Role: eywa.RoleUser, Content: "real"},
	}
	history := p.convertToHistory(msgs)
	if len(history) != 1 {
		t.Errorf("expected 1 entry (empty skipped), got %d", len(history))
	}
}

// --- hasConversationHistory ---

func TestHasConversationHistory_WithMessages_True(t *testing.T) {
	p := &GeminiOracle{}
	req := &eywa.OracleRequest{Messages: []eywa.OracleMessage{{Role: eywa.RoleUser, Content: "hi"}}}
	if !p.hasConversationHistory(req) {
		t.Error("expected true")
	}
}

func TestHasConversationHistory_NoMessages_False(t *testing.T) {
	p := &GeminiOracle{}
	req := &eywa.OracleRequest{}
	if p.hasConversationHistory(req) {
		t.Error("expected false")
	}
}

// --- addSamplingParameters ---

func TestAddSamplingParameters_AllFields(t *testing.T) {
	p := &GeminiOracle{}
	config := &genai.GenerateContentConfig{}
	req := &eywa.OracleRequest{
		Temperature:   0.7,
		MaxTokens:     200,
		TopP:          0.9,
		TopK:          40,
		StopSequences: []string{"END"},
	}
	p.addSamplingParameters(config, req)

	if config.Temperature == nil || *config.Temperature != 0.7 {
		t.Errorf("unexpected temperature: %v", config.Temperature)
	}
	if config.MaxOutputTokens != 200 {
		t.Errorf("unexpected max_tokens: %d", config.MaxOutputTokens)
	}
	if config.TopP == nil || *config.TopP != 0.9 {
		t.Errorf("unexpected top_p: %v", config.TopP)
	}
	if config.TopK == nil || *config.TopK != 40 {
		t.Errorf("unexpected top_k: %v", config.TopK)
	}
	if len(config.StopSequences) != 1 {
		t.Errorf("unexpected stop sequences: %v", config.StopSequences)
	}
}

func TestAddSamplingParameters_ZeroValues_NotSet(t *testing.T) {
	p := &GeminiOracle{}
	config := &genai.GenerateContentConfig{}
	req := &eywa.OracleRequest{}
	p.addSamplingParameters(config, req)

	if config.Temperature != nil {
		t.Error("temperature should not be set")
	}
	if config.MaxOutputTokens != 0 {
		t.Error("max_tokens should not be set")
	}
}

// --- addSystemInstruction ---

func TestAddSystemInstruction_NonEmpty_SetsInstruction(t *testing.T) {
	p := &GeminiOracle{}
	config := &genai.GenerateContentConfig{}
	p.addSystemInstruction(config, "be helpful")
	if config.SystemInstruction == nil {
		t.Fatal("expected SystemInstruction to be set")
	}
	if len(config.SystemInstruction.Parts) != 1 || config.SystemInstruction.Parts[0].Text != "be helpful" {
		t.Errorf("unexpected system instruction: %+v", config.SystemInstruction)
	}
}

func TestAddSystemInstruction_Empty_NoOp(t *testing.T) {
	p := &GeminiOracle{}
	config := &genai.GenerateContentConfig{}
	p.addSystemInstruction(config, "")
	if config.SystemInstruction != nil {
		t.Error("expected nil SystemInstruction for empty prompt")
	}
}

// --- addTools ---

func TestAddTools_UseToolsFalse_NoTools(t *testing.T) {
	p := &GeminiOracle{}
	config := &genai.GenerateContentConfig{}
	req := &eywa.OracleRequest{UseTools: false, Tools: []eywa.OracleTool{{Name: "x"}}}
	p.addTools(config, req)
	if len(config.Tools) != 0 {
		t.Error("expected no tools when UseTools=false")
	}
}

func TestAddTools_EmptyTools_NoTools(t *testing.T) {
	p := &GeminiOracle{}
	config := &genai.GenerateContentConfig{}
	req := &eywa.OracleRequest{UseTools: true, Tools: nil}
	p.addTools(config, req)
	if len(config.Tools) != 0 {
		t.Error("expected no tools for empty list")
	}
}

func TestAddTools_WithTools_SetsConfig(t *testing.T) {
	p := &GeminiOracle{}
	config := &genai.GenerateContentConfig{}
	req := &eywa.OracleRequest{
		UseTools: true,
		Tools:    []eywa.OracleTool{{Name: "search", Description: "search web"}},
	}
	p.addTools(config, req)
	if len(config.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(config.Tools))
	}
}

// --- convertTools ---

func TestConvertTools_EmptyList_ReturnsNil(t *testing.T) {
	p := &GeminiOracle{}
	result := p.convertTools(nil)
	if result != nil {
		t.Error("expected nil for empty tools")
	}
}

func TestConvertTools_MapsAll(t *testing.T) {
	p := &GeminiOracle{}
	tools := []eywa.OracleTool{
		{Name: "t1", Description: "first"},
		{Name: "t2", Description: "second"},
	}
	result := p.convertTools(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 Tool wrapper, got %d", len(result))
	}
	if len(result[0].FunctionDeclarations) != 2 {
		t.Errorf("expected 2 declarations, got %d", len(result[0].FunctionDeclarations))
	}
}

// --- createFunctionDeclaration ---

func TestCreateFunctionDeclaration_SetsFields(t *testing.T) {
	p := &GeminiOracle{}
	tool := eywa.OracleTool{
		Name:        "search",
		Description: "search web",
		Parameters:  map[string]any{"type": "object"},
	}
	decl := p.createFunctionDeclaration(tool)
	if decl.Name != "search" || decl.Description != "search web" {
		t.Errorf("unexpected declaration: %+v", decl)
	}
}

// --- createFunctionResponsePart ---

func TestCreateFunctionResponsePart_SetsNameAndResult(t *testing.T) {
	p := &GeminiOracle{}
	msg := eywa.OracleMessage{ToolName: "search", Content: "found it"}
	part := p.createFunctionResponsePart(msg)
	if part.FunctionResponse == nil {
		t.Fatal("expected FunctionResponse")
	}
	if part.FunctionResponse.Name != "search" {
		t.Errorf("unexpected name: %s", part.FunctionResponse.Name)
	}
	if part.FunctionResponse.Response["result"] != "found it" {
		t.Errorf("unexpected result: %v", part.FunctionResponse.Response)
	}
}

// --- convertToolCallsToParts ---

func TestConvertToolCallsToParts_MapsAll(t *testing.T) {
	p := &GeminiOracle{}
	calls := []eywa.OracleToolCall{
		{ID: "tc-1", Name: "search", Arguments: map[string]any{"q": "test"}},
	}
	parts := p.convertToolCallsToParts(calls)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].FunctionCall == nil || parts[0].FunctionCall.Name != "search" {
		t.Errorf("unexpected part: %+v", parts[0])
	}
}

func TestConvertToolCallsToParts_PreservesThoughtSignature(t *testing.T) {
	p := &GeminiOracle{}
	sig := []byte("sig-data")
	calls := []eywa.OracleToolCall{
		{Name: "fn", ThoughtSignature: sig},
	}
	parts := p.convertToolCallsToParts(calls)
	if string(parts[0].ThoughtSignature) != "sig-data" {
		t.Errorf("expected ThoughtSignature preserved, got %v", parts[0].ThoughtSignature)
	}
}

// --- convertAttachmentsToParts ---

func TestConvertAttachmentsToParts_WithData_ReturnsPart(t *testing.T) {
	p := &GeminiOracle{}
	att := eywa.LLMAttachment{
		Type:     eywa.ArtifactTypeImage,
		Data:     []byte("imgdata"),
		MimeType: "image/jpeg",
	}
	parts := p.convertAttachmentsToParts([]eywa.LLMAttachment{att})
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].InlineData == nil || parts[0].InlineData.MIMEType != "image/jpeg" {
		t.Errorf("unexpected part: %+v", parts[0])
	}
}

func TestConvertAttachmentsToParts_NoData_SkipsAttachment(t *testing.T) {
	p := &GeminiOracle{}
	att := eywa.LLMAttachment{
		Type: eywa.ArtifactTypeImage,
		URL:  "https://example.com/img.jpg",
	}
	parts := p.convertAttachmentsToParts([]eywa.LLMAttachment{att})
	if len(parts) != 0 {
		t.Errorf("expected 0 parts for URL-only attachment, got %d", len(parts))
	}
}

// --- extractContentAndToolCalls ---

func TestExtractContentAndToolCalls_TextOnly(t *testing.T) {
	p := &GeminiOracle{}
	candidate := &genai.Candidate{
		Content: &genai.Content{
			Parts: []*genai.Part{{Text: "hello world"}},
		},
	}
	content, toolCalls := p.extractContentAndToolCalls(candidate)
	if content != "hello world" {
		t.Errorf("expected 'hello world', got %q", content)
	}
	if len(toolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(toolCalls))
	}
}

func TestExtractContentAndToolCalls_WithFunctionCall(t *testing.T) {
	p := &GeminiOracle{}
	candidate := &genai.Candidate{
		Content: &genai.Content{
			Parts: []*genai.Part{
				{
					FunctionCall: &genai.FunctionCall{
						Name: "search",
						Args: map[string]any{"q": "test"},
					},
				},
			},
		},
	}
	content, toolCalls := p.extractContentAndToolCalls(candidate)
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
	if len(toolCalls) != 1 || toolCalls[0].Name != "search" {
		t.Errorf("unexpected tool calls: %+v", toolCalls)
	}
}

func TestExtractContentAndToolCalls_NilContent_ReturnsEmpty(t *testing.T) {
	p := &GeminiOracle{}
	candidate := &genai.Candidate{Content: nil}
	content, toolCalls := p.extractContentAndToolCalls(candidate)
	if content != "" || len(toolCalls) != 0 {
		t.Errorf("expected empty results for nil content")
	}
}

// --- convertPartToToolCall ---

func TestConvertPartToToolCall_MapsFields(t *testing.T) {
	p := &GeminiOracle{}
	sig := []byte("thought-sig")
	part := &genai.Part{
		FunctionCall: &genai.FunctionCall{
			Name: "tool1",
			Args: map[string]any{"x": 1},
		},
		ThoughtSignature: sig,
	}
	tc := p.convertPartToToolCall(part)
	if tc.ID != "tool1" || tc.Name != "tool1" {
		t.Errorf("unexpected ID/Name: %s/%s", tc.ID, tc.Name)
	}
	if string(tc.ThoughtSignature) != "thought-sig" {
		t.Error("expected ThoughtSignature preserved")
	}
}

// --- parseResponse ---

func TestParseResponse_EmptyCandidates_ReturnsError(t *testing.T) {
	p := &GeminiOracle{}
	_, err := p.parseResponse(&genai.GenerateContentResponse{})
	if err == nil {
		t.Error("expected error for empty candidates")
	}
}

func TestParseResponse_ValidResponse_WithText(t *testing.T) {
	p := &GeminiOracle{}
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content:      &genai.Content{Parts: []*genai.Part{{Text: "answer"}}},
				FinishReason: genai.FinishReasonStop,
			},
		},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			TotalTokenCount: 10,
		},
	}
	result, err := p.parseResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "answer" {
		t.Errorf("expected 'answer', got %q", result.Content)
	}
	if result.StopReason != eywa.StopReasonComplete {
		t.Errorf("unexpected stop reason: %s", result.StopReason)
	}
}

func TestParseResponse_WithToolCalls_StopReasonToolCalls(t *testing.T) {
	p := &GeminiOracle{}
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{FunctionCall: &genai.FunctionCall{Name: "search", Args: map[string]any{}}},
					},
				},
				FinishReason: genai.FinishReasonStop,
			},
		},
	}
	result, err := p.parseResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StopReason != eywa.StopReasonToolCalls {
		t.Errorf("expected tool_calls stop reason, got %s", result.StopReason)
	}
}

// --- splitMessagesForChat ---

func TestSplitMessagesForChat_LastUserMessage_IsSendParts(t *testing.T) {
	p := &GeminiOracle{}
	msgs := []eywa.OracleMessage{
		{Role: eywa.RoleUser, Content: "first"},
		{Role: eywa.RoleAssistant, Content: "reply"},
		{Role: eywa.RoleUser, Content: "question"},
	}
	history, sendParts := p.splitMessagesForChat(msgs, nil)
	if len(history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history))
	}
	if len(sendParts) != 1 || sendParts[0].Text != "question" {
		t.Errorf("unexpected send parts: %+v", sendParts)
	}
}

func TestSplitMessagesForChat_TrailingToolMessages_MergedIntoSend(t *testing.T) {
	p := &GeminiOracle{}
	msgs := []eywa.OracleMessage{
		{Role: eywa.RoleUser, Content: "ask"},
		{Role: eywa.RoleTool, Content: "r1", ToolName: "t1"},
		{Role: eywa.RoleTool, Content: "r2", ToolName: "t2"},
	}
	history, sendParts := p.splitMessagesForChat(msgs, nil)
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
	// Two tool messages merged → 2 send parts
	if len(sendParts) != 2 {
		t.Errorf("expected 2 send parts (merged tools), got %d", len(sendParts))
	}
}

func TestSplitMessagesForChat_EmptyMessages_EmptyPlaceholder(t *testing.T) {
	p := &GeminiOracle{}
	_, sendParts := p.splitMessagesForChat(nil, nil)
	// No messages → sendParts fallback to [" "]
	if len(sendParts) != 1 || sendParts[0].Text != " " {
		t.Errorf("expected placeholder send part, got %+v", sendParts)
	}
}

func TestSplitMessagesForChat_WithAttachments_AppendedToSend(t *testing.T) {
	p := &GeminiOracle{}
	msgs := []eywa.OracleMessage{
		{Role: eywa.RoleUser, Content: "look"},
	}
	attachments := []eywa.LLMAttachment{
		{Type: eywa.ArtifactTypeImage, Data: []byte("img"), MimeType: "image/jpeg"},
	}
	_, sendParts := p.splitMessagesForChat(msgs, attachments)
	// text part + image part
	if len(sendParts) != 2 {
		t.Errorf("expected 2 send parts, got %d", len(sendParts))
	}
}

// --- buildSimpleContent ---

func TestBuildSimpleContent_NoSystemNoAttachments_EmptyParts(t *testing.T) {
	p := &GeminiOracle{}
	req := &eywa.OracleRequest{}
	content := p.buildSimpleContent(req)
	if len(content.Parts) != 0 {
		t.Errorf("expected 0 parts, got %d", len(content.Parts))
	}
}

func TestBuildSimpleContent_WithSystemPrompt(t *testing.T) {
	p := &GeminiOracle{}
	req := &eywa.OracleRequest{SystemPrompt: "be helpful"}
	content := p.buildSimpleContent(req)
	if len(content.Parts) != 1 || content.Parts[0].Text != "be helpful" {
		t.Errorf("unexpected parts: %+v", content.Parts)
	}
}

func TestBuildSimpleContent_WithAttachments(t *testing.T) {
	p := &GeminiOracle{}
	req := &eywa.OracleRequest{
		Attachments: []eywa.LLMAttachment{
			{Type: eywa.ArtifactTypeImage, Data: []byte("img"), MimeType: "image/png"},
		},
	}
	content := p.buildSimpleContent(req)
	if len(content.Parts) != 1 || content.Parts[0].InlineData == nil {
		t.Errorf("expected 1 inline data part, got %+v", content.Parts)
	}
}

// --- buildGenerationConfig ---

func TestBuildGenerationConfig_SetsAll(t *testing.T) {
	p := &GeminiOracle{}
	req := &eywa.OracleRequest{
		SystemPrompt:  "helpful",
		Temperature:   0.5,
		UseTools:      true,
		Tools:         []eywa.OracleTool{{Name: "x"}},
		StopSequences: []string{"END"},
	}
	config := p.buildGenerationConfig(req)
	if config.SystemInstruction == nil {
		t.Error("expected SystemInstruction")
	}
	if config.Temperature == nil || *config.Temperature != 0.5 {
		t.Error("expected temperature 0.5")
	}
	if len(config.Tools) != 1 {
		t.Error("expected 1 tool")
	}
}
