package openai

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/openai/openai-go"
	eywa "github.com/wmulabs/eywa"
)

// --- constructors ---

func TestNewOracle_DefaultName(t *testing.T) {
	p := NewOracle("key")
	if p.GetName() != ProviderName {
		t.Errorf("expected %q, got %q", ProviderName, p.GetName())
	}
}

func TestNewOracle_DefaultMaxRetries(t *testing.T) {
	p := NewOracle("key")
	if p.config.MaxRetries != defaultMaxRetries {
		t.Errorf("expected %d, got %d", defaultMaxRetries, p.config.MaxRetries)
	}
}

func TestNewOracle_DefaultTimeout(t *testing.T) {
	p := NewOracle("key")
	if p.config.Timeout != defaultTimeout {
		t.Errorf("expected %d, got %d", defaultTimeout, p.config.Timeout)
	}
}

func TestNewOracleWithConfig_ZeroMaxRetries_SetsDefault(t *testing.T) {
	p := NewOracleWithConfig(Config{APIKey: "k", MaxRetries: 0})
	if p.config.MaxRetries != defaultMaxRetries {
		t.Errorf("expected %d, got %d", defaultMaxRetries, p.config.MaxRetries)
	}
}

func TestNewOracleWithConfig_ZeroTimeout_SetsDefault(t *testing.T) {
	p := NewOracleWithConfig(Config{APIKey: "k", Timeout: 0})
	if p.config.Timeout != defaultTimeout {
		t.Errorf("expected %d, got %d", defaultTimeout, p.config.Timeout)
	}
}

func TestNewOracleWithConfig_EmptyName_SetsProviderName(t *testing.T) {
	p := NewOracleWithConfig(Config{APIKey: "k"})
	if p.config.Name != ProviderName {
		t.Errorf("expected %q, got %q", ProviderName, p.config.Name)
	}
}

func TestNewOracleWithConfig_CustomName(t *testing.T) {
	p := NewOracleWithConfig(Config{Name: "myprovider", APIKey: "k"})
	if p.config.Name != "myprovider" {
		t.Errorf("expected myprovider, got %s", p.config.Name)
	}
}

func TestNewOracleWithConfig_BaseURL(t *testing.T) {
	p := NewOracleWithConfig(Config{APIKey: "k", BaseURL: "http://localhost/v1"})
	if p.config.BaseURL != "http://localhost/v1" {
		t.Errorf("unexpected BaseURL: %s", p.config.BaseURL)
	}
}

func TestNewOracleWithConfig_OrgID(t *testing.T) {
	// OrgID triggers the WithOrganization option path in createOpenAIClient.
	p := NewOracleWithConfig(Config{APIKey: "k", OrgID: "org-abc"})
	if p.config.OrgID != "org-abc" {
		t.Error("expected OrgID to be stored")
	}
	if p.client == nil {
		t.Error("expected non-nil client even with OrgID")
	}
}

func TestNewOllamaOracle_Name(t *testing.T) {
	p := NewOllamaOracle("http://localhost:11434")
	if p.GetName() != ProviderNameOllama {
		t.Errorf("expected %q, got %q", ProviderNameOllama, p.GetName())
	}
}

func TestNewGroqOracle_Name(t *testing.T) {
	p := NewGroqOracle("key")
	if p.GetName() != ProviderNameGroq {
		t.Errorf("expected %q, got %q", ProviderNameGroq, p.GetName())
	}
}

func TestNewMistralOracle_Name(t *testing.T) {
	p := NewMistralOracle("key")
	if p.GetName() != ProviderNameMistral {
		t.Errorf("expected %q, got %q", ProviderNameMistral, p.GetName())
	}
}

func TestNewTogetherOracle_Name(t *testing.T) {
	p := NewTogetherOracle("key")
	if p.GetName() != ProviderNameTogether {
		t.Errorf("expected %q, got %q", ProviderNameTogether, p.GetName())
	}
}

func TestNewOpenRouterOracle_Name(t *testing.T) {
	p := NewOpenRouterOracle("key")
	if p.GetName() != ProviderNameOpenRouter {
		t.Errorf("expected %q, got %q", ProviderNameOpenRouter, p.GetName())
	}
}

func TestNewXAIOracle_Name(t *testing.T) {
	p := NewXAIOracle("key")
	if p.GetName() != ProviderNameXAI {
		t.Errorf("expected %q, got %q", ProviderNameXAI, p.GetName())
	}
}

// --- GetName / GetAvailableModels / IsAvailable / GetConfig ---

func TestGetName_ReturnsConfigName(t *testing.T) {
	p := NewOracleWithConfig(Config{Name: "custom", APIKey: "k"})
	if p.GetName() != "custom" {
		t.Errorf("expected custom, got %s", p.GetName())
	}
}

func TestGetAvailableModels_NonEmpty(t *testing.T) {
	p := NewOracle("key")
	models := p.GetAvailableModels()
	if len(models) == 0 {
		t.Error("expected non-empty model list")
	}
}

func TestIsAvailable_True(t *testing.T) {
	p := NewOracle("key")
	if !p.IsAvailable() {
		t.Error("expected IsAvailable() to be true")
	}
}

func TestGetConfig_ContainsExpectedKeys(t *testing.T) {
	p := NewOracleWithConfig(Config{Name: "groq", APIKey: "mykey", BaseURL: "http://x"})
	cfg := p.GetConfig()
	if cfg["provider"] != "groq" {
		t.Errorf("unexpected provider: %v", cfg["provider"])
	}
	if cfg["has_api_key"] != true {
		t.Errorf("expected has_api_key=true, got %v", cfg["has_api_key"])
	}
	if cfg["base_url"] != "http://x" {
		t.Errorf("unexpected base_url: %v", cfg["base_url"])
	}
}

func TestGetConfig_NoAPIKey(t *testing.T) {
	p := NewOracle("")
	cfg := p.GetConfig()
	if cfg["has_api_key"] != false {
		t.Errorf("expected has_api_key=false, got %v", cfg["has_api_key"])
	}
}

// --- SupportsImages ---

func TestSupportsImages_GPT4o_True(t *testing.T) {
	p := NewOracle("")
	if !p.SupportsImages("gpt-4o") {
		t.Error("gpt-4o should support images")
	}
}

func TestSupportsImages_GPT35_False(t *testing.T) {
	p := NewOracle("")
	if p.SupportsImages("gpt-3.5-turbo") {
		t.Error("gpt-3.5 should not support images")
	}
}

func TestSupportsImages_TextDavinci_False(t *testing.T) {
	p := NewOracle("")
	if p.SupportsImages("text-davinci-003") {
		t.Error("text-davinci should not support images")
	}
}

func TestSupportsImages_TextAda_False(t *testing.T) {
	p := NewOracle("")
	if p.SupportsImages("text-ada-001") {
		t.Error("text-ada should not support images")
	}
}

func TestSupportsImages_TextBabbage_False(t *testing.T) {
	p := NewOracle("")
	if p.SupportsImages("text-babbage-001") {
		t.Error("text-babbage should not support images")
	}
}

func TestSupportsImages_TextCurie_False(t *testing.T) {
	p := NewOracle("")
	if p.SupportsImages("text-curie-001") {
		t.Error("text-curie should not support images")
	}
}

func TestSupportsImages_EmptyModel_False(t *testing.T) {
	p := NewOracle("")
	if p.SupportsImages("") {
		t.Error("empty model should not support images")
	}
}

// --- SupportsAudio / SupportsDocuments ---

func TestSupportsAudio_AlwaysFalse(t *testing.T) {
	p := NewOracle("")
	if p.SupportsAudio("gpt-4o") {
		t.Error("expected SupportsAudio=false")
	}
}

func TestSupportsDocuments_AlwaysFalse(t *testing.T) {
	p := NewOracle("")
	if p.SupportsDocuments("gpt-4o") {
		t.Error("expected SupportsDocuments=false")
	}
}

// --- normalizeStopReason ---

func TestNormalizeStopReason_Stop(t *testing.T) {
	p := NewOracle("")
	if got := p.normalizeStopReason("stop"); got != eywa.StopReasonComplete {
		t.Errorf("expected %q, got %q", eywa.StopReasonComplete, got)
	}
}

func TestNormalizeStopReason_Length(t *testing.T) {
	p := NewOracle("")
	if got := p.normalizeStopReason("length"); got != eywa.StopReasonLength {
		t.Errorf("expected %q, got %q", eywa.StopReasonLength, got)
	}
}

func TestNormalizeStopReason_ToolCalls(t *testing.T) {
	p := NewOracle("")
	if got := p.normalizeStopReason("tool_calls"); got != eywa.StopReasonToolCalls {
		t.Errorf("expected %q, got %q", eywa.StopReasonToolCalls, got)
	}
}

func TestNormalizeStopReason_ContentFilter(t *testing.T) {
	p := NewOracle("")
	if got := p.normalizeStopReason("content_filter"); got != eywa.StopReasonContentFilter {
		t.Errorf("expected %q, got %q", eywa.StopReasonContentFilter, got)
	}
}

func TestNormalizeStopReason_Unknown_Passthrough(t *testing.T) {
	p := NewOracle("")
	if got := p.normalizeStopReason("function_call"); got != "function_call" {
		t.Errorf("expected passthrough, got %q", got)
	}
}

// --- extractTokenUsage ---

func TestExtractTokenUsage_MapsFields(t *testing.T) {
	p := NewOracle("")
	usage := openai.CompletionUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}
	got := p.extractTokenUsage(usage)
	if got.PromptTokens != 100 || got.CompletionTokens != 50 || got.TotalTokens != 150 {
		t.Errorf("unexpected usage: %+v", got)
	}
}

// --- validateResponse ---

func TestValidateResponse_EmptyChoices_ReturnsError(t *testing.T) {
	p := NewOracle("")
	err := p.validateResponse(&openai.ChatCompletion{})
	if err == nil {
		t.Error("expected error for empty choices")
	}
}

func TestValidateResponse_WithChoices_NoError(t *testing.T) {
	p := NewOracle("")
	resp := &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Content: "hi"}},
		},
	}
	if err := p.validateResponse(resp); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- extractToolCalls ---

func TestExtractToolCalls_EmptySlice_ReturnsNil(t *testing.T) {
	p := NewOracle("")
	result, err := p.extractToolCalls(nil)
	if err != nil || result != nil {
		t.Errorf("expected nil,nil; got %v,%v", result, err)
	}
}

func TestExtractToolCalls_ValidCall(t *testing.T) {
	p := NewOracle("")
	calls := []openai.ChatCompletionMessageToolCall{
		{
			ID: "call-1",
			Function: openai.ChatCompletionMessageToolCallFunction{
				Name:      "search",
				Arguments: `{"query":"test"}`,
			},
		},
	}
	result, err := p.extractToolCalls(calls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].Name != "search" {
		t.Errorf("unexpected result: %+v", result)
	}
	if result[0].Arguments["query"] != "test" {
		t.Errorf("unexpected args: %v", result[0].Arguments)
	}
}

func TestExtractToolCalls_InvalidJSON_ReturnsError(t *testing.T) {
	p := NewOracle("")
	calls := []openai.ChatCompletionMessageToolCall{
		{
			ID: "call-1",
			Function: openai.ChatCompletionMessageToolCallFunction{
				Name:      "bad",
				Arguments: `not-json`,
			},
		},
	}
	_, err := p.extractToolCalls(calls)
	if err == nil {
		t.Error("expected error for invalid JSON arguments")
	}
}

// --- parseToolCall ---

func TestParseToolCall_Success(t *testing.T) {
	p := NewOracle("")
	tc := openai.ChatCompletionMessageToolCall{
		ID: "tc-42",
		Function: openai.ChatCompletionMessageToolCallFunction{
			Name:      "weather",
			Arguments: `{"city":"NYC"}`,
		},
	}
	got, err := p.parseToolCall(tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "tc-42" || got.Name != "weather" {
		t.Errorf("unexpected tool call: %+v", got)
	}
	if got.Arguments["city"] != "NYC" {
		t.Errorf("unexpected args: %v", got.Arguments)
	}
}

// --- buildStopSequencesParam ---

func TestBuildStopSequencesParam_Single_UsesOfString(t *testing.T) {
	p := NewOracle("")
	result := p.buildStopSequencesParam([]string{"STOP"})
	if !result.OfString.Valid() || result.OfString.Value != "STOP" {
		t.Errorf("expected OfString=STOP, got %+v", result)
	}
}

func TestBuildStopSequencesParam_Multiple_UsesArray(t *testing.T) {
	p := NewOracle("")
	result := p.buildStopSequencesParam([]string{"END", "STOP"})
	if len(result.OfStringArray) != 2 {
		t.Errorf("expected OfStringArray with 2 elements, got %+v", result)
	}
}

// --- addSamplingParameters ---

func TestAddSamplingParameters_AllFields(t *testing.T) {
	p := NewOracle("")
	params := openai.ChatCompletionNewParams{}
	req := &eywa.OracleRequest{
		Temperature:   0.7,
		MaxTokens:     100,
		TopP:          0.9,
		StopSequences: []string{"END"},
	}
	p.addSamplingParameters(&params, req)

	if params.Temperature.Value != 0.7 {
		t.Errorf("unexpected temperature: %v", params.Temperature)
	}
	if params.MaxTokens.Value != 100 {
		t.Errorf("unexpected max_tokens: %v", params.MaxTokens)
	}
	if params.TopP.Value != 0.9 {
		t.Errorf("unexpected top_p: %v", params.TopP)
	}
}

func TestAddSamplingParameters_ZeroValues_NotSet(t *testing.T) {
	p := NewOracle("")
	params := openai.ChatCompletionNewParams{}
	req := &eywa.OracleRequest{}
	p.addSamplingParameters(&params, req)

	if params.Temperature.Valid() {
		t.Error("temperature should not be set")
	}
	if params.MaxTokens.Valid() {
		t.Error("max_tokens should not be set")
	}
	if params.TopP.Valid() {
		t.Error("top_p should not be set")
	}
}

// --- addTools ---

func TestAddTools_UseToolsFalse_NoTools(t *testing.T) {
	p := NewOracle("")
	params := openai.ChatCompletionNewParams{}
	req := &eywa.OracleRequest{
		UseTools: false,
		Tools:    []eywa.OracleTool{{Name: "search"}},
	}
	p.addTools(&params, req)
	if len(params.Tools) != 0 {
		t.Error("expected no tools when UseTools=false")
	}
}

func TestAddTools_EmptyTools_NoTools(t *testing.T) {
	p := NewOracle("")
	params := openai.ChatCompletionNewParams{}
	req := &eywa.OracleRequest{UseTools: true, Tools: nil}
	p.addTools(&params, req)
	if len(params.Tools) != 0 {
		t.Error("expected no tools for empty tool list")
	}
}

func TestAddTools_WithTools_SetsParams(t *testing.T) {
	p := NewOracle("")
	params := openai.ChatCompletionNewParams{}
	req := &eywa.OracleRequest{
		UseTools: true,
		Tools: []eywa.OracleTool{
			{Name: "search", Description: "Search the web"},
		},
	}
	p.addTools(&params, req)
	if len(params.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(params.Tools))
	}
	if params.Tools[0].Function.Name != "search" {
		t.Errorf("unexpected tool name: %s", params.Tools[0].Function.Name)
	}
}

// --- convertTools / createToolParam ---

func TestConvertTools_MapsAll(t *testing.T) {
	p := NewOracle("")
	tools := []eywa.OracleTool{
		{Name: "t1", Description: "first"},
		{Name: "t2", Description: "second"},
	}
	result := p.convertTools(tools)
	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}
	if result[0].Function.Name != "t1" || result[1].Function.Name != "t2" {
		t.Errorf("unexpected tool names: %+v", result)
	}
}

func TestCreateToolParam_WithDescription(t *testing.T) {
	p := NewOracle("")
	tool := eywa.OracleTool{Name: "foo", Description: "does foo"}
	param := p.createToolParam(tool)
	if param.Function.Name != "foo" {
		t.Errorf("unexpected name: %s", param.Function.Name)
	}
	if !param.Function.Description.Valid() || param.Function.Description.Value != "does foo" {
		t.Errorf("unexpected description: %v", param.Function.Description)
	}
}

// --- convertMessages ---

func TestConvertMessages_WithSystemPrompt_PrependSystem(t *testing.T) {
	p := NewOracle("")
	req := &eywa.OracleRequest{
		SystemPrompt: "You are helpful.",
		Messages: []eywa.OracleMessage{
			{Role: eywa.RoleUser, Content: "hello"},
		},
	}
	msgs := p.convertMessages(req)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system+user), got %d", len(msgs))
	}
}

func TestConvertMessages_NoSystemPrompt_OnlyMessages(t *testing.T) {
	p := NewOracle("")
	req := &eywa.OracleRequest{
		Messages: []eywa.OracleMessage{
			{Role: eywa.RoleUser, Content: "hi"},
		},
	}
	msgs := p.convertMessages(req)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestConvertMessages_UnknownRole_Skipped(t *testing.T) {
	p := NewOracle("")
	req := &eywa.OracleRequest{
		Messages: []eywa.OracleMessage{
			{Role: "unknown", Content: "???"},
		},
	}
	msgs := p.convertMessages(req)
	if len(msgs) != 0 {
		t.Errorf("expected unknown role to be skipped, got %d messages", len(msgs))
	}
}

// --- convertMessage ---

func TestConvertMessage_ToolRole_ReturnsToolMessage(t *testing.T) {
	p := NewOracle("")
	msg := eywa.OracleMessage{Role: eywa.RoleTool, Content: "result", ToolName: "search"}
	req := &eywa.OracleRequest{Messages: []eywa.OracleMessage{msg}}
	result := p.convertMessage(msg, 0, req)
	if result == nil {
		t.Error("expected non-nil for tool role")
	}
	if result.OfTool == nil {
		t.Error("expected OfTool to be set")
	}
}

func TestConvertMessage_ToolRole_EmptyContent_FallsThrough(t *testing.T) {
	p := NewOracle("")
	msg := eywa.OracleMessage{Role: eywa.RoleTool, Content: ""}
	req := &eywa.OracleRequest{Messages: []eywa.OracleMessage{msg}}
	// Empty content falls through to convertMessageByRole → unknown "tool" role → nil
	result := p.convertMessage(msg, 0, req)
	if result != nil {
		t.Error("expected nil for tool role with empty content (falls to convertMessageByRole)")
	}
}

func TestConvertMessage_WithToolCalls_ReturnsAssistant(t *testing.T) {
	p := NewOracle("")
	msg := eywa.OracleMessage{
		Role:    eywa.RoleAssistant,
		Content: "I'll search",
		ToolCalls: []eywa.OracleToolCall{
			{ID: "tc-1", Name: "search", Arguments: map[string]any{"q": "test"}},
		},
	}
	req := &eywa.OracleRequest{Messages: []eywa.OracleMessage{msg}}
	result := p.convertMessage(msg, 0, req)
	if result == nil || result.OfAssistant == nil {
		t.Error("expected assistant message with tool calls")
	}
}

// --- convertMessageByRole ---

func TestConvertMessageByRole_User(t *testing.T) {
	p := NewOracle("")
	msg := eywa.OracleMessage{Role: eywa.RoleUser, Content: "hello"}
	req := &eywa.OracleRequest{Messages: []eywa.OracleMessage{msg}}
	result := p.convertMessageByRole(msg, 0, req)
	if result == nil || result.OfUser == nil {
		t.Error("expected user message")
	}
}

func TestConvertMessageByRole_Assistant(t *testing.T) {
	p := NewOracle("")
	msg := eywa.OracleMessage{Role: eywa.RoleAssistant, Content: "hi"}
	req := &eywa.OracleRequest{Messages: []eywa.OracleMessage{msg}}
	result := p.convertMessageByRole(msg, 0, req)
	if result == nil || result.OfAssistant == nil {
		t.Error("expected assistant message")
	}
}

func TestConvertMessageByRole_System(t *testing.T) {
	p := NewOracle("")
	msg := eywa.OracleMessage{Role: eywa.RoleSystem, Content: "be helpful"}
	req := &eywa.OracleRequest{Messages: []eywa.OracleMessage{msg}}
	result := p.convertMessageByRole(msg, 0, req)
	if result == nil || result.OfSystem == nil {
		t.Error("expected system message")
	}
}

func TestConvertMessageByRole_Unknown_ReturnsNil(t *testing.T) {
	p := NewOracle("")
	msg := eywa.OracleMessage{Role: "unknown", Content: "?"}
	req := &eywa.OracleRequest{Messages: []eywa.OracleMessage{msg}}
	result := p.convertMessageByRole(msg, 0, req)
	if result != nil {
		t.Error("expected nil for unknown role")
	}
}

// --- createUserMessage ---

func TestCreateUserMessage_LastWithAttachments_Multimodal(t *testing.T) {
	p := NewOracle("")
	msg := eywa.OracleMessage{Role: eywa.RoleUser, Content: "describe"}
	req := &eywa.OracleRequest{
		Messages: []eywa.OracleMessage{msg},
		Attachments: []eywa.LLMAttachment{
			{Type: eywa.ArtifactTypeImage, URL: "https://example.com/img.png"},
		},
	}
	result := p.createUserMessage(msg, 0, req) // index 0 == len(Messages)-1
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.OfUser == nil {
		t.Error("expected OfUser for multimodal message")
	}
}

func TestCreateUserMessage_NotLast_PlainText(t *testing.T) {
	p := NewOracle("")
	msg1 := eywa.OracleMessage{Role: eywa.RoleUser, Content: "first"}
	msg2 := eywa.OracleMessage{Role: eywa.RoleUser, Content: "last"}
	req := &eywa.OracleRequest{
		Messages: []eywa.OracleMessage{msg1, msg2},
		Attachments: []eywa.LLMAttachment{
			{Type: eywa.ArtifactTypeImage, URL: "https://example.com/img.png"},
		},
	}
	// index 0 is not the last message (len=2), so plain text
	result := p.createUserMessage(msg1, 0, req)
	if result == nil || result.OfUser == nil {
		t.Fatal("expected user message")
	}
}

// --- buildMultimodalContentParts ---

func TestBuildMultimodalContentParts_TextAndImage(t *testing.T) {
	p := NewOracle("")
	att := eywa.LLMAttachment{
		Type: eywa.ArtifactTypeImage,
		URL:  "https://example.com/img.jpg",
	}
	parts := p.buildMultimodalContentParts("describe this", []eywa.LLMAttachment{att})
	if len(parts) != 2 {
		t.Errorf("expected 2 parts (text+image), got %d", len(parts))
	}
}

func TestBuildMultimodalContentParts_EmptyText_NoTextPart(t *testing.T) {
	p := NewOracle("")
	att := eywa.LLMAttachment{
		Type: eywa.ArtifactTypeImage,
		URL:  "https://example.com/img.jpg",
	}
	parts := p.buildMultimodalContentParts("", []eywa.LLMAttachment{att})
	if len(parts) != 1 {
		t.Errorf("expected 1 part (image only), got %d", len(parts))
	}
}

func TestBuildMultimodalContentParts_NonImageAttachment_Ignored(t *testing.T) {
	p := NewOracle("")
	att := eywa.LLMAttachment{
		Type: eywa.ArtifactType("document"),
		URL:  "https://example.com/doc.pdf",
	}
	parts := p.buildMultimodalContentParts("text", []eywa.LLMAttachment{att})
	// Only text part — non-image skipped
	if len(parts) != 1 {
		t.Errorf("expected 1 part (text only), got %d", len(parts))
	}
}

// --- createTextContentPart ---

func TestCreateTextContentPart(t *testing.T) {
	p := NewOracle("")
	part := p.createTextContentPart("hello")
	if part.OfText == nil || part.OfText.Text != "hello" {
		t.Errorf("unexpected text part: %+v", part)
	}
}

// --- isImageAttachment ---

func TestIsImageAttachment_ImageType_True(t *testing.T) {
	p := NewOracle("")
	att := eywa.LLMAttachment{Type: eywa.ArtifactTypeImage}
	if !p.isImageAttachment(att) {
		t.Error("expected image attachment")
	}
}

func TestIsImageAttachment_OtherType_False(t *testing.T) {
	p := NewOracle("")
	att := eywa.LLMAttachment{Type: eywa.ArtifactType("audio")}
	if p.isImageAttachment(att) {
		t.Error("expected non-image attachment")
	}
}

// --- createImageContentPart ---

func TestCreateImageContentPart_WithData_UsesBase64(t *testing.T) {
	p := NewOracle("")
	data := []byte("imagedata")
	att := eywa.LLMAttachment{
		Type:     eywa.ArtifactTypeImage,
		Data:     data,
		MimeType: "image/png",
	}
	result := p.createImageContentPart(att)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.OfImageURL == nil {
		t.Fatal("expected OfImageURL")
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	expected := fmt.Sprintf("data:image/png;base64,%s", encoded)
	if result.OfImageURL.ImageURL.URL != expected {
		t.Errorf("unexpected URL: %s", result.OfImageURL.ImageURL.URL)
	}
}

func TestCreateImageContentPart_WithData_NoMimeType_UsesDefault(t *testing.T) {
	p := NewOracle("")
	att := eywa.LLMAttachment{
		Type: eywa.ArtifactTypeImage,
		Data: []byte("x"),
	}
	result := p.createImageContentPart(att)
	if result == nil || result.OfImageURL == nil {
		t.Fatal("expected image URL part")
	}
	if !strings.HasPrefix(result.OfImageURL.ImageURL.URL, "data:image/jpeg;base64,") {
		t.Errorf("expected default mime type, got: %s", result.OfImageURL.ImageURL.URL)
	}
}

func TestCreateImageContentPart_WithURL_UsesURL(t *testing.T) {
	p := NewOracle("")
	att := eywa.LLMAttachment{
		Type: eywa.ArtifactTypeImage,
		URL:  "https://example.com/img.png",
	}
	result := p.createImageContentPart(att)
	if result == nil || result.OfImageURL == nil {
		t.Fatal("expected OfImageURL")
	}
	if result.OfImageURL.ImageURL.URL != "https://example.com/img.png" {
		t.Errorf("unexpected URL: %s", result.OfImageURL.ImageURL.URL)
	}
}

func TestCreateImageContentPart_NoDataNoURL_ReturnsNil(t *testing.T) {
	p := NewOracle("")
	att := eywa.LLMAttachment{Type: eywa.ArtifactTypeImage}
	result := p.createImageContentPart(att)
	if result != nil {
		t.Error("expected nil for attachment with no data and no URL")
	}
}

// --- convertToolCallsToParams ---

func TestConvertToolCallsToParams_MarshalArgs(t *testing.T) {
	p := NewOracle("")
	calls := []eywa.OracleToolCall{
		{
			ID:        "tc-1",
			Name:      "search",
			Arguments: map[string]any{"q": "test"},
		},
	}
	params := p.convertToolCallsToParams(calls)
	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(params))
	}
	if params[0].ID != "tc-1" || params[0].Function.Name != "search" {
		t.Errorf("unexpected param: %+v", params[0])
	}
	if !strings.Contains(params[0].Function.Arguments, `"q"`) {
		t.Errorf("expected JSON args, got: %s", params[0].Function.Arguments)
	}
}

// --- buildChatCompletionParams ---

func TestBuildChatCompletionParams_SetsModelAndMessages(t *testing.T) {
	p := NewOracle("")
	req := &eywa.OracleRequest{
		Model: "gpt-4o",
		Messages: []eywa.OracleMessage{
			{Role: eywa.RoleUser, Content: "hello"},
		},
	}
	params := p.buildChatCompletionParams(req)
	if params.Model != "gpt-4o" {
		t.Errorf("unexpected model: %s", params.Model)
	}
	if len(params.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(params.Messages))
	}
}

// --- parseResponse ---

func TestParseResponse_EmptyChoices_ReturnsError(t *testing.T) {
	p := NewOracle("")
	_, err := p.parseResponse(&openai.ChatCompletion{})
	if err == nil {
		t.Error("expected error for empty choices")
	}
}

func TestParseResponse_InvalidToolCallArgs_ReturnsError(t *testing.T) {
	p := NewOracle("")
	resp := &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Content: "calling tool",
					ToolCalls: []openai.ChatCompletionMessageToolCall{
						{
							ID: "tc-bad",
							Function: openai.ChatCompletionMessageToolCallFunction{
								Name:      "bad",
								Arguments: `not-json`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
	}
	_, err := p.parseResponse(resp)
	if err == nil {
		t.Error("expected error for invalid tool call JSON")
	}
}

func TestParseResponse_ValidResponse(t *testing.T) {
	p := NewOracle("")
	resp := &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{
				Message:      openai.ChatCompletionMessage{Content: "hello"},
				FinishReason: "stop",
			},
		},
		Usage: openai.CompletionUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}
	result, err := p.parseResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "hello" {
		t.Errorf("expected 'hello', got %q", result.Content)
	}
	if result.StopReason != eywa.StopReasonComplete {
		t.Errorf("unexpected stop reason: %s", result.StopReason)
	}
	if result.TokensUsed.TotalTokens != 15 {
		t.Errorf("unexpected total tokens: %d", result.TokensUsed.TotalTokens)
	}
}
