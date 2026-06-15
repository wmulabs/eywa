package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

func shapingTestRequest() *ReasoningRequest {
	return &ReasoningRequest{
		Spirit: &entities.Spirit{ModelConfig: entities.SpiritModel{Model: "test-model"}},
	}
}

func TestTruncateToolResult_UnderCap_ReturnedUnchanged(t *testing.T) {
	content := "short result"
	got := truncateToolResult(content, ports.ToolResultLimits{MaxChars: 100})
	if got != content {
		t.Errorf("expected unchanged content, got %q", got)
	}
}

func TestTruncateToolResult_DisabledWhenMaxCharsZero(t *testing.T) {
	content := strings.Repeat("x", 10_000)
	got := truncateToolResult(content, ports.ToolResultLimits{MaxChars: 0})
	if got != content {
		t.Error("MaxChars=0 must disable truncation (content unchanged)")
	}
}

func TestTruncateToolResult_OverCap_KeepsHeadAndTailWithNotice(t *testing.T) {
	head := strings.Repeat("H", 70)
	tail := strings.Repeat("T", 30)
	content := head + strings.Repeat("M", 500) + tail

	got := truncateToolResult(content, ports.ToolResultLimits{
		MaxChars: 100,
		KeepHead: 70,
		KeepTail: 30,
	})

	if !strings.HasPrefix(got, head) {
		t.Errorf("expected output to start with the kept head, got %q", got[:min(80, len(got))])
	}
	if !strings.HasSuffix(got, tail) {
		t.Errorf("expected output to end with the kept tail, got %q", got)
	}
	if !strings.Contains(got, "truncated") {
		t.Error("expected a truncation notice in the output")
	}
	if strings.Contains(got, "M") {
		t.Error("middle content must be dropped")
	}
}

func TestTruncateToolResult_DefaultsHeadTailFromMaxChars(t *testing.T) {
	content := strings.Repeat("a", 1000)
	got := truncateToolResult(content, ports.ToolResultLimits{MaxChars: 100})

	// With no explicit KeepHead/KeepTail, defaults split 70/30 of MaxChars.
	// Output kept content (excluding the notice) must not exceed MaxChars.
	notice := truncationNotice(len(content) - 100)
	kept := strings.Replace(got, notice, "", 1)
	if len(kept) > 100 {
		t.Errorf("kept content %d exceeds MaxChars 100", len(kept))
	}
	if len(got) >= len(content) {
		t.Error("expected output shorter than original")
	}
}

func TestTruncateToolResult_ClampsWhenHeadPlusTailExceedMax(t *testing.T) {
	content := strings.Repeat("a", 1000)
	// KeepHead+KeepTail (90+90=180) > MaxChars (100): must clamp to MaxChars.
	got := truncateToolResult(content, ports.ToolResultLimits{
		MaxChars: 100,
		KeepHead: 90,
		KeepTail: 90,
	})
	notice := truncationNotice(len(content) - 100)
	kept := strings.Replace(got, notice, "", 1)
	if len(kept) > 100 {
		t.Errorf("kept content %d must be clamped to MaxChars 100", len(kept))
	}
}

// shaperAction is a stubAction that overrides the global tool-result limit.
type shaperAction struct {
	stubAction
	limit ports.ToolResultLimits
}

func (a *shaperAction) ResultLimit() ports.ToolResultLimits { return a.limit }

func TestLimitsFor_UsesGlobalWhenActionHasNoOverride(t *testing.T) {
	exec := NewActionExecutor(newRegistry(&stubAction{name: "plain"}), false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "", 0, testLogger(t), noopTracer())
	svc.SetToolResultLimits(ports.ToolResultLimits{MaxChars: 8000})

	got := svc.limitsFor("plain")
	if got.MaxChars != 8000 {
		t.Errorf("expected global MaxChars 8000, got %d", got.MaxChars)
	}
}

func TestLimitsFor_UsesActionOverrideWhenImplemented(t *testing.T) {
	action := &shaperAction{
		stubAction: stubAction{name: "bigtool"},
		limit:      ports.ToolResultLimits{MaxChars: 200},
	}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "", 0, testLogger(t), noopTracer())
	svc.SetToolResultLimits(ports.ToolResultLimits{MaxChars: 8000})

	got := svc.limitsFor("bigtool")
	if got.MaxChars != 200 {
		t.Errorf("expected action override MaxChars 200, got %d", got.MaxChars)
	}
}

func TestLimitsFor_UnknownAction_FallsBackToGlobal(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "", 0, testLogger(t), noopTracer())
	svc.SetToolResultLimits(ports.ToolResultLimits{MaxChars: 5000})

	got := svc.limitsFor("missing")
	if got.MaxChars != 5000 {
		t.Errorf("expected global fallback MaxChars 5000, got %d", got.MaxChars)
	}
}

func TestHandleActionSuccess_ShapesContextMessage(t *testing.T) {
	exec := NewActionExecutor(newRegistry(&stubAction{name: "big"}), false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "", 0, testLogger(t), noopTracer())
	svc.SetToolResultLimits(ports.ToolResultLimits{MaxChars: 100, KeepHead: 70, KeepTail: 30})

	big := strings.Repeat("x", 1000)
	wc := []ports.OracleMessage{}
	res := &ReasoningResult{}
	svc.handleActionSuccess(context.Background(), nil, shapingTestRequest(), call("big"), ActionResult{Result: big}, res, &wc)

	if len(wc) != 1 {
		t.Fatalf("expected 1 context message, got %d", len(wc))
	}
	last := wc[0]
	if last.Role != ports.RoleTool {
		t.Errorf("expected RoleTool, got %v", last.Role)
	}
	if len(last.Content) >= len(big) {
		t.Error("context message should be shaped (shorter than full result)")
	}
	if !strings.Contains(last.Content, "truncated") {
		t.Error("expected truncation notice in the shaped context message")
	}
}

func TestHandleActionSuccess_NoLimits_KeepsFullContent(t *testing.T) {
	exec := NewActionExecutor(newRegistry(&stubAction{name: "big"}), false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "", 0, testLogger(t), noopTracer())
	// No SetToolResultLimits → disabled.

	big := strings.Repeat("x", 1000)
	wc := []ports.OracleMessage{}
	svc.handleActionSuccess(context.Background(), nil, shapingTestRequest(), call("big"), ActionResult{Result: big}, &ReasoningResult{}, &wc)

	if wc[0].Content != big {
		t.Error("with shaping disabled the full result must reach the context")
	}
}

func TestHandleActionSuccess_SummarizeStrategy_UsesOracleSummary(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{
		Content:    "CONDENSED SUMMARY",
		TokensUsed: ports.OracleUsage{TotalTokens: 9},
	}}
	exec := NewActionExecutor(newRegistry(&stubAction{name: "big"}), false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "", 0, testLogger(t), noopTracer())
	svc.SetToolResultLimits(ports.ToolResultLimits{MaxChars: 100, Strategy: ports.ToolShapeSummarize})

	big := strings.Repeat("x", 1000)
	wc := []ports.OracleMessage{}
	res := &ReasoningResult{}
	svc.handleActionSuccess(context.Background(), oracle, shapingTestRequest(), call("big"), ActionResult{Result: big}, res, &wc)

	if !strings.Contains(wc[0].Content, "CONDENSED SUMMARY") {
		t.Errorf("expected the oracle summary in context, got %q", wc[0].Content)
	}
	if res.TokensUsed.TotalTokens != 9 {
		t.Errorf("expected summarize tokens accumulated (9), got %d", res.TokensUsed.TotalTokens)
	}
}

func TestHandleActionSuccess_SummarizeFails_FallsBackToTruncate(t *testing.T) {
	oracle := &stubOracle{err: errors.New("oracle down")}
	exec := NewActionExecutor(newRegistry(&stubAction{name: "big"}), false, testLogger(t), noopTracer())
	svc := NewReasoningService(&stubOracleFactory{}, exec, nil, 5, "", 0, testLogger(t), noopTracer())
	svc.SetToolResultLimits(ports.ToolResultLimits{MaxChars: 100, KeepHead: 70, KeepTail: 30, Strategy: ports.ToolShapeSummarize})

	big := strings.Repeat("x", 1000)
	wc := []ports.OracleMessage{}
	svc.handleActionSuccess(context.Background(), oracle, shapingTestRequest(), call("big"), ActionResult{Result: big}, &ReasoningResult{}, &wc)

	if !strings.Contains(wc[0].Content, "truncated") {
		t.Errorf("expected truncation fallback when summarize fails, got %q", wc[0].Content)
	}
}

func TestBuildActionCallLog_KeepsFullResultForAudit(t *testing.T) {
	big := strings.Repeat("x", 1000)
	log := buildActionCallLog(call("big"), ActionResult{Result: big})
	if log.Result != big {
		t.Error("audit log must retain the full, unshaped result")
	}
}

func TestTruncateToolResult_CutsOnRuneBoundary(t *testing.T) {
	// Multibyte runes (each 'é' is 2 bytes). Truncation must not split a rune.
	content := strings.Repeat("é", 500)
	got := truncateToolResult(content, ports.ToolResultLimits{MaxChars: 100, KeepHead: 70, KeepTail: 30})
	if !utf8.ValidString(got) {
		t.Error("truncation produced invalid UTF-8 (split a rune)")
	}
}
