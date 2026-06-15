package orchestrator

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

var errContext = errors.New("compress boom")

// compressionOracle returns tool calls for the first reasoningTurns reasoning requests, then a
// terminal response; compression requests (SystemPrompt == ledgerInstruction) return a fixed ledger.
type compressionOracle struct {
	mu            sync.Mutex
	reasoningTurn int
	ledgerCalls   int
	maxToolTurns  int
}

func (o *compressionOracle) GetName() string                 { return "compress" }
func (o *compressionOracle) GetAvailableModels() []string    { return nil }
func (o *compressionOracle) IsAvailable() bool               { return true }
func (o *compressionOracle) GetConfig() map[string]any       { return nil }
func (o *compressionOracle) SupportsImages(_ string) bool    { return false }
func (o *compressionOracle) SupportsAudio(_ string) bool     { return false }
func (o *compressionOracle) SupportsDocuments(_ string) bool { return false }
func (o *compressionOracle) GenerateResponse(_ context.Context, req *ports.OracleRequest) (*ports.OracleResponse, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if req.SystemPrompt == ledgerInstruction {
		o.ledgerCalls++
		return &ports.OracleResponse{Content: "LEDGER SUMMARY", StopReason: ports.StopReasonComplete}, nil
	}
	o.reasoningTurn++
	if o.reasoningTurn <= o.maxToolTurns {
		return toolCallResp("lookup", map[string]any{"n": o.reasoningTurn}), nil
	}
	return &ports.OracleResponse{Content: "done", StopReason: ports.StopReasonComplete}, nil
}

func TestExecute_ContextCompression_CollapsesOldIterations(t *testing.T) {
	bigResult := strings.Repeat("x", 100)
	action := &stubAction{name: "lookup", execResult: bigResult, category: ports.ActionRetrieval}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())

	oracle := &compressionOracle{maxToolTurns: 2}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetCompressionPolicy(CompressionPolicy{Enabled: true, MaxContextChars: 50, KeepRecent: 1})

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "done" {
		t.Errorf("expected terminal response 'done', got %q", result.FinalResponse)
	}
	if oracle.ledgerCalls != 1 {
		t.Errorf("expected exactly 1 compression call, got %d", oracle.ledgerCalls)
	}

	hasLedger := false
	for _, m := range result.WorkingContext {
		if strings.HasPrefix(m.Content, ledgerPrefix) {
			hasLedger = true
		}
	}
	if !hasLedger {
		t.Error("expected an evidence-ledger message in the working context after compression")
	}
}

func newCompressionSvc(t *testing.T, oracle ports.Oracle, policy CompressionPolicy) *ReasoningService {
	t.Helper()
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetCompressionPolicy(policy)
	return svc
}

func bigMessages(n, size int) []ports.OracleMessage {
	msgs := make([]ports.OracleMessage, n)
	for i := range msgs {
		msgs[i] = ports.OracleMessage{Role: ports.RoleTool, Content: strings.Repeat("x", size)}
	}
	return msgs
}

func TestMaybeCompress_UnderThreshold_Unchanged(t *testing.T) {
	svc := newCompressionSvc(t, &stubOracle{}, CompressionPolicy{Enabled: true, MaxContextChars: 100000, KeepRecent: 1})
	wc := []ports.OracleMessage{{Role: ports.RoleUser, Content: "hi"}}

	got, bounds := svc.maybeCompress(context.Background(), &stubOracle{}, testRequest(), wc, 0, []int{1}, &ReasoningResult{})
	if len(got) != 1 || len(bounds) != 1 {
		t.Errorf("under threshold must return context and boundaries unchanged, got %d msgs / %d bounds", len(got), len(bounds))
	}
}

func TestMaybeCompress_NotEnoughIterations_Unchanged(t *testing.T) {
	svc := newCompressionSvc(t, &stubOracle{}, CompressionPolicy{Enabled: true, MaxContextChars: 5, KeepRecent: 1})
	wc := bigMessages(2, 10)

	// Only one boundary recorded (<= keep) -> nothing to compress yet.
	got, _ := svc.maybeCompress(context.Background(), &stubOracle{}, testRequest(), wc, 0, []int{2}, &ReasoningResult{})
	if len(got) != 2 {
		t.Errorf("with too few iterations the context must be unchanged, got %d", len(got))
	}
}

func TestMaybeCompress_NothingCompressible_Unchanged(t *testing.T) {
	svc := newCompressionSvc(t, &stubOracle{}, CompressionPolicy{Enabled: true, MaxContextChars: 5, KeepRecent: 1})
	wc := bigMessages(3, 10)

	// offset==recentStart -> compressible span empty.
	got, _ := svc.maybeCompress(context.Background(), &stubOracle{}, testRequest(), wc, 2, []int{1, 2}, &ReasoningResult{})
	if len(got) != 3 {
		t.Errorf("empty compressible span must leave context unchanged, got %d", len(got))
	}
}

func TestMaybeCompress_KeepRecentDefaultsToOne(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "LEDGER", TokensUsed: ports.OracleUsage{TotalTokens: 3}}}
	svc := newCompressionSvc(t, oracle, CompressionPolicy{Enabled: true, MaxContextChars: 5, KeepRecent: 0})
	wc := bigMessages(2, 10)

	got, _ := svc.maybeCompress(context.Background(), oracle, testRequest(), wc, 0, []int{1, 2}, &ReasoningResult{})
	if len(got) == 0 || !strings.HasPrefix(got[0].Content, ledgerPrefix) {
		t.Error("KeepRecent=0 must default to 1 and still compress into a ledger")
	}
}

func TestMaybeCompress_SummarizeFails_KeepsFullContext(t *testing.T) {
	oracle := &stubOracle{err: errContext}
	svc := newCompressionSvc(t, oracle, CompressionPolicy{Enabled: true, MaxContextChars: 5, KeepRecent: 1})
	wc := bigMessages(3, 10)

	got, bounds := svc.maybeCompress(context.Background(), oracle, testRequest(), wc, 0, []int{1, 2, 3}, &ReasoningResult{})
	if len(got) != 3 || len(bounds) != 3 {
		t.Error("a failed summarize must keep the full context and boundaries")
	}
}

func TestExecute_ContextCompression_Disabled_NoCompression(t *testing.T) {
	bigResult := strings.Repeat("x", 100)
	action := &stubAction{name: "lookup", execResult: bigResult, category: ports.ActionRetrieval}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())

	oracle := &compressionOracle{maxToolTurns: 2}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	// No SetCompressionPolicy -> disabled.

	if _, err := svc.Execute(context.Background(), testRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if oracle.ledgerCalls != 0 {
		t.Errorf("compression must not run when disabled, got %d calls", oracle.ledgerCalls)
	}
}

func TestContextChars_SumsMessageContent(t *testing.T) {
	msgs := []ports.OracleMessage{
		{Role: ports.RoleUser, Content: "abc"},
		{Role: ports.RoleAssistant, Content: "de"},
		{Role: ports.RoleTool, Content: "fghij"},
	}
	if got := contextChars(msgs); got != 10 {
		t.Errorf("expected 10, got %d", got)
	}
}

func TestContextChars_Empty(t *testing.T) {
	if got := contextChars(nil); got != 0 {
		t.Errorf("expected 0 for nil, got %d", got)
	}
}

func TestRenderMessagesForLedger_IncludesRoleAndContent(t *testing.T) {
	msgs := []ports.OracleMessage{
		{Role: ports.RoleAssistant, ToolName: "lookup", Content: "calling"},
		{Role: ports.RoleTool, ToolName: "lookup", Content: "result payload"},
	}
	out := renderMessagesForLedger(msgs)
	if !strings.Contains(out, "result payload") || !strings.Contains(out, "lookup") {
		t.Errorf("rendered ledger input must retain tool name and content, got %q", out)
	}
}
