package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- ReasoningResult.accumulateTokens ---

func TestAccumulateTokens(t *testing.T) {
	r := &ReasoningResult{}
	r.accumulateTokens(ports.OracleUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15})
	r.accumulateTokens(ports.OracleUsage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5})
	if r.TokensUsed.PromptTokens != 13 {
		t.Errorf("expected 13, got %d", r.TokensUsed.PromptTokens)
	}
	if r.TokensUsed.CompletionTokens != 7 {
		t.Errorf("expected 7, got %d", r.TokensUsed.CompletionTokens)
	}
	if r.TokensUsed.TotalTokens != 20 {
		t.Errorf("expected 20, got %d", r.TokensUsed.TotalTokens)
	}
}

// --- isTerminalResponse ---

func newTestReasoningService(t *testing.T) *ReasoningService {
	t.Helper()
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	return NewReasoningService(
		&stubOracleFactory{},
		exec,
		nil,
		5,
		"max iterations reached",
		0,
		testLogger(t),
		noopTracer(),
	)
}

func TestIsTerminalResponse_Complete(t *testing.T) {
	svc := newTestReasoningService(t)
	if !svc.isTerminalResponse(&ports.OracleResponse{StopReason: ports.StopReasonComplete}) {
		t.Error("expected terminal for StopReasonComplete")
	}
}

func TestIsTerminalResponse_ToolCalls(t *testing.T) {
	svc := newTestReasoningService(t)
	if svc.isTerminalResponse(&ports.OracleResponse{StopReason: ports.StopReasonToolCalls}) {
		t.Error("expected non-terminal for StopReasonToolCalls")
	}
}

func TestIsTerminalResponse_Length(t *testing.T) {
	svc := newTestReasoningService(t)
	if !svc.isTerminalResponse(&ports.OracleResponse{StopReason: ports.StopReasonLength}) {
		t.Error("expected terminal for StopReasonLength")
	}
}

func TestIsTerminalResponse_ContentFilter(t *testing.T) {
	svc := newTestReasoningService(t)
	if !svc.isTerminalResponse(&ports.OracleResponse{StopReason: ports.StopReasonContentFilter}) {
		t.Error("expected terminal for StopReasonContentFilter")
	}
}

func TestIsTerminalResponse_Unknown(t *testing.T) {
	svc := newTestReasoningService(t)
	if !svc.isTerminalResponse(&ports.OracleResponse{StopReason: "some-unknown-reason"}) {
		t.Error("expected terminal for unknown stop reason")
	}
}

// --- filterBannedActions ---

func TestFilterBannedActions_EmptyBanned_ReturnsAll(t *testing.T) {
	actions := []ports.OracleTool{{Name: "a"}, {Name: "b"}}
	result := filterBannedActions(actions, map[string]bool{})
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestFilterBannedActions_RemovesBanned(t *testing.T) {
	actions := []ports.OracleTool{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	banned := map[string]bool{"b": true}
	result := filterBannedActions(actions, banned)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	for _, a := range result {
		if a.Name == "b" {
			t.Error("banned action b must not be in result")
		}
	}
}

func TestFilterBannedActions_AllBanned_ReturnsEmpty(t *testing.T) {
	actions := []ports.OracleTool{{Name: "a"}, {Name: "b"}}
	banned := map[string]bool{"a": true, "b": true}
	result := filterBannedActions(actions, banned)
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

// --- resolveIterationActions ---

func TestResolveIterationActions_InfraTerminal_ReturnsNilAndInfraHint(t *testing.T) {
	actions := []ports.OracleTool{{Name: "a"}}
	result, hint := resolveIterationActions(actions, nil, true)
	if result != nil {
		t.Error("expected nil actions for infraTerminal")
	}
	if hint != infraClosingInstruction {
		t.Errorf("unexpected hint: %q", hint)
	}
}

func TestResolveIterationActions_BannedSet_FiltersAndReturnsHint(t *testing.T) {
	actions := []ports.OracleTool{{Name: "a"}, {Name: "b"}}
	banned := map[string]bool{"a": true}
	result, hint := resolveIterationActions(actions, banned, false)
	if len(result) != 1 || result[0].Name != "b" {
		t.Errorf("expected only b, got %v", result)
	}
	if hint != closingInstruction {
		t.Errorf("unexpected hint: %q", hint)
	}
}

func TestResolveIterationActions_NoBan_NoInfra_ReturnsAllWithEmptyHint(t *testing.T) {
	actions := []ports.OracleTool{{Name: "a"}, {Name: "b"}}
	result, hint := resolveIterationActions(actions, map[string]bool{}, false)
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
	if hint != "" {
		t.Errorf("expected empty hint, got %q", hint)
	}
}

// --- actionErrorMessage ---

func TestActionErrorMessage_CriticalBusinessError(t *testing.T) {
	result := ActionResult{
		IsCritical: true,
		Error:      domainerrors.NewBusinessError("insufficient funds"),
	}
	msg := actionErrorMessage("call-1", "pay", result)
	if !msg.IsError {
		t.Error("expected IsError=true")
	}
	if msg.Role != ports.RoleTool {
		t.Errorf("expected tool role, got %s", msg.Role)
	}
}

func TestActionErrorMessage_CriticalInfraError(t *testing.T) {
	result := ActionResult{
		IsCritical: true,
		Error:      domainerrors.NewInfrastructureError("db down", nil),
	}
	msg := actionErrorMessage("call-1", "save", result)
	if msg.Content != "This operation failed due to a technical error." {
		t.Errorf("unexpected content: %q", msg.Content)
	}
}

func TestActionErrorMessage_NonCriticalBusinessError(t *testing.T) {
	result := ActionResult{
		IsCritical: false,
		Error:      domainerrors.NewBusinessError("order not found"),
	}
	msg := actionErrorMessage("call-1", "find", result)
	if msg.ToolCallID != "call-1" {
		t.Errorf("expected call-1, got %s", msg.ToolCallID)
	}
}

func TestActionErrorMessage_NonCriticalGenericError(t *testing.T) {
	result := ActionResult{
		IsCritical: false,
		Error:      errors.New("generic error"),
	}
	msg := actionErrorMessage("call-1", "do", result)
	if msg.Content == "" {
		t.Error("expected non-empty content")
	}
}

// --- buildActionCallLog ---

func TestBuildActionCallLog_Success(t *testing.T) {
	call := ports.OracleToolCall{Name: "pay", Arguments: map[string]any{"amount": 100}}
	result := ActionResult{IsCritical: true, Result: "paid", DurationMs: 42}
	log := buildActionCallLog(call, result)
	if log.ActionName != "pay" {
		t.Errorf("expected pay, got %s", log.ActionName)
	}
	if log.IsError {
		t.Error("expected IsError=false")
	}
	if log.Result != "paid" {
		t.Errorf("expected paid, got %s", log.Result)
	}
	if log.DurationMs != 42 {
		t.Errorf("expected 42ms, got %d", log.DurationMs)
	}
}

func TestBuildActionCallLog_Error(t *testing.T) {
	call := ports.OracleToolCall{Name: "pay"}
	result := ActionResult{Error: errors.New("failed"), IsCritical: false}
	log := buildActionCallLog(call, result)
	if !log.IsError {
		t.Error("expected IsError=true")
	}
	if log.ErrorMessage != "failed" {
		t.Errorf("unexpected error message: %q", log.ErrorMessage)
	}
}

// --- chatMessagesToLLM ---

func TestChatMessagesToLLM_MapsAllFields(t *testing.T) {
	threads := []entities.Thread{
		{Role: "user", Content: "hello", ImageURLs: []string{"img1"}, AudioURLs: []string{"audio1"}},
		{Role: "assistant", Content: "hi"},
	}
	msgs := chatMessagesToLLM(threads)
	if len(msgs) != 2 {
		t.Fatalf("expected 2, got %d", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("expected user, got %s", msgs[0].Role)
	}
	if msgs[0].Content != "hello" {
		t.Errorf("expected hello, got %s", msgs[0].Content)
	}
	if len(msgs[0].ImageURLs) != 1 || msgs[0].ImageURLs[0] != "img1" {
		t.Error("expected ImageURLs preserved")
	}
	if len(msgs[0].AudioURLs) != 1 || msgs[0].AudioURLs[0] != "audio1" {
		t.Error("expected AudioURLs preserved")
	}
}

func TestChatMessagesToLLM_Empty(t *testing.T) {
	msgs := chatMessagesToLLM(nil)
	if len(msgs) != 0 {
		t.Errorf("expected 0, got %d", len(msgs))
	}
}

// --- sessionTopicKey ---

func TestSessionTopicKey_Nil(t *testing.T) {
	if got := sessionTopicKey(nil); got != "" {
		t.Errorf("expected empty for nil session, got %q", got)
	}
}

func TestSessionTopicKey_ReturnsSubjectKey(t *testing.T) {
	mem := &entities.Memory{SubjectKey: "order:42"}
	if got := sessionTopicKey(mem); got != "order:42" {
		t.Errorf("expected order:42, got %q", got)
	}
}

func TestSessionTopicKey_EmptySubjectKey(t *testing.T) {
	mem := &entities.Memory{SubjectKey: ""}
	if got := sessionTopicKey(mem); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// --- summonSpiritTool ---

func TestSummonSpiritTool_ContainsSubSpirits(t *testing.T) {
	tool := summonSpiritTool([]string{"support", "billing"})
	if tool.Name != summonSpiritActionName {
		t.Errorf("expected %s, got %s", summonSpiritActionName, tool.Name)
	}
	if tool.Description == "" {
		t.Error("expected non-empty description")
	}
	if tool.Parameters == nil {
		t.Error("expected non-nil parameters")
	}
}

func TestSummonSpiritTool_EmptySubSpirits(t *testing.T) {
	tool := summonSpiritTool([]string{})
	if tool.Name != summonSpiritActionName {
		t.Errorf("expected %s, got %s", summonSpiritActionName, tool.Name)
	}
}

// --- applyIsCriticalOverrides ---

func TestApplyIsCriticalOverrides_NoOverrides_ReturnsOriginal(t *testing.T) {
	calls := []ports.OracleToolCall{{Name: "pay"}, {Name: "notify"}}
	result := applyIsCriticalOverrides(calls, []entities.AllowedAction{
		{Name: "pay"}, // IsCritical nil = no override
	})
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
	if result[0].IsCriticalOverride != nil {
		t.Error("expected nil override")
	}
}

func TestApplyIsCriticalOverrides_WithOverride(t *testing.T) {
	critical := true
	calls := []ports.OracleToolCall{{Name: "pay"}, {Name: "notify"}}
	allowed := []entities.AllowedAction{
		{Name: "pay", IsCritical: &critical},
	}
	result := applyIsCriticalOverrides(calls, allowed)
	if result[0].IsCriticalOverride == nil || !*result[0].IsCriticalOverride {
		t.Error("expected IsCriticalOverride=true for pay")
	}
	if result[1].IsCriticalOverride != nil {
		t.Error("expected no override for notify")
	}
}

func TestApplyIsCriticalOverrides_EmptyAllowed_ReturnsOriginal(t *testing.T) {
	calls := []ports.OracleToolCall{{Name: "pay"}}
	result := applyIsCriticalOverrides(calls, nil)
	if len(result) != 1 || result[0].IsCriticalOverride != nil {
		t.Error("expected unchanged calls for empty allowed")
	}
}

// --- availableActions ---

func TestAvailableActions_RegisteredAction_Included(t *testing.T) {
	action := &stubAction{name: "send_msg", execResult: "ok"}
	registry := newRegistry(action)
	exec := NewActionExecutor(registry, false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "", 0, testLogger(t), noopTracer())

	req := &ReasoningRequest{
		Spirit: &entities.Spirit{
			Name: "support",
			Type: entities.SpiritTypeConversational,
			AllowedActions: []entities.AllowedAction{
				{Name: "send_msg"},
			},
		},
	}
	tools := svc.availableActions(req)
	if len(tools) != 1 || tools[0].Name != "send_msg" {
		t.Errorf("expected [send_msg], got %v", tools)
	}
}

func TestAvailableActions_UnregisteredAction_Skipped(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "", 0, testLogger(t), noopTracer())

	req := &ReasoningRequest{
		Spirit: &entities.Spirit{
			Name: "support",
			AllowedActions: []entities.AllowedAction{{Name: "missing_action"}},
		},
	}
	tools := svc.availableActions(req)
	if len(tools) != 0 {
		t.Errorf("expected empty, got %v", tools)
	}
}

func TestAvailableActions_DescriptionOverride(t *testing.T) {
	action := &stubAction{name: "pay"}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "", 0, testLogger(t), noopTracer())

	req := &ReasoningRequest{
		Spirit: &entities.Spirit{
			AllowedActions: []entities.AllowedAction{
				{Name: "pay", DescriptionOverride: "Custom payment desc"},
			},
		},
	}
	tools := svc.availableActions(req)
	if len(tools) != 1 || tools[0].Description != "Custom payment desc" {
		t.Errorf("expected override desc, got %q", tools[0].Description)
	}
}

func TestAvailableActions_OrchestratorWithSubSpirits_AddsSummonTool(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "", 0, testLogger(t), noopTracer())
	svc.summonService = &SummonService{} // non-nil to trigger summon tool injection

	req := &ReasoningRequest{
		Spirit: &entities.Spirit{
			Name: "orchestrator",
			Type: entities.SpiritTypeOrchestrator,
			OrchestratorConfig: entities.OrchestratorConfig{
				SubSpirits: []string{"support", "billing"},
			},
		},
	}
	tools := svc.availableActions(req)
	found := false
	for _, t := range tools {
		if t.Name == summonSpiritActionName {
			found = true
		}
	}
	if !found {
		t.Error("expected summon_spirit tool for orchestrator spirit")
	}
}

// --- NewReasoningService ---

func TestNewReasoningService_ReturnsNonNil(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "max", 10, testLogger(t), noopTracer())
	if svc == nil {
		t.Fatal("expected non-nil")
	}
}

func TestReasoningService_SetSummonService(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "", 0, testLogger(t), noopTracer())
	summon := &SummonService{}
	svc.SetSummonService(summon)
	if svc.summonService != summon {
		t.Error("expected summonService to be set")
	}
}

// Avoid "imported and not used" for fmt and time
var _ = fmt.Sprintf
var _ = time.Second
var _ = context.Background
