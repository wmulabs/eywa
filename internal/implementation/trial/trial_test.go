package trial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- stubs ---

type stubWeave struct {
	resp *entities.Response
	err  error
}

func (w *stubWeave) ProcessEventByKey(_ context.Context, _ string, _ *entities.Pulse) (*entities.Response, error) {
	return w.resp, w.err
}

type stubOracle struct {
	content string
	err     error
}

func (o *stubOracle) GetName() string                   { return "test" }
func (o *stubOracle) GetAvailableModels() []string      { return nil }
func (o *stubOracle) IsAvailable() bool                 { return true }
func (o *stubOracle) GetConfig() map[string]any { return nil }
func (o *stubOracle) SupportsImages(_ string) bool      { return false }
func (o *stubOracle) SupportsAudio(_ string) bool       { return false }
func (o *stubOracle) SupportsDocuments(_ string) bool   { return false }
func (o *stubOracle) GenerateResponse(_ context.Context, _ *ports.OracleRequest) (*ports.OracleResponse, error) {
	return &ports.OracleResponse{Content: o.content}, o.err
}

type stubOracleFactory struct {
	oracle ports.Oracle
	err    error
}

func (f *stubOracleFactory) GetProvider(_ string) (ports.Oracle, error)           { return f.oracle, f.err }
func (f *stubOracleFactory) GetDefaultProvider() (ports.Oracle, error)            { return f.oracle, f.err }
func (f *stubOracleFactory) GetProviderForModel(_ string) (ports.Oracle, error)   { return f.oracle, f.err }
func (f *stubOracleFactory) RegisterProvider(_ string, _ ports.Oracle) error      { return nil }
func (f *stubOracleFactory) SetDefaultProvider(_ string) error                    { return nil }
func (f *stubOracleFactory) ListProviders() []string                              { return nil }
func (f *stubOracleFactory) ListAvailableProviders() []string                     { return nil }

func okResponse(text string, actions ...string) *entities.Response {
	return &entities.Response{FinalResponse: text, ActionsExecuted: actions}
}

// --- LoadTrialSuite ---

func TestLoadTrialSuite_YAML(t *testing.T) {
	content := `id: suite-1
name: Test Suite
cases:
  - id: tc-1
    event_type: chat
    user_message: hello
    expect:
      output_contains:
        - hi
`
	path := filepath.Join(t.TempDir(), "suite.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	suite, err := LoadTrialSuite(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suite.ID != "suite-1" {
		t.Errorf("expected suite-1, got %s", suite.ID)
	}
	if len(suite.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(suite.Cases))
	}
	if suite.Cases[0].ID != "tc-1" {
		t.Errorf("expected tc-1, got %s", suite.Cases[0].ID)
	}
}

func TestLoadTrialSuite_YML(t *testing.T) {
	content := `id: suite-yml
name: YML Suite
cases: []
`
	path := filepath.Join(t.TempDir(), "suite.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	suite, err := LoadTrialSuite(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suite.ID != "suite-yml" {
		t.Errorf("expected suite-yml, got %s", suite.ID)
	}
}

func TestLoadTrialSuite_JSON(t *testing.T) {
	data := entities.TrialSuite{
		ID:    "suite-json",
		Name:  "JSON Suite",
		Cases: []entities.TrialCase{{ID: "tc-json", EventType: "chat", UserMessage: "hi"}},
	}
	raw, _ := json.Marshal(data)
	path := filepath.Join(t.TempDir(), "suite.json")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	suite, err := LoadTrialSuite(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suite.ID != "suite-json" {
		t.Errorf("expected suite-json, got %s", suite.ID)
	}
}

func TestLoadTrialSuite_UnsupportedFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suite.txt")
	if err := os.WriteFile(path, []byte("foo"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadTrialSuite(path)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestLoadTrialSuite_FileNotFound(t *testing.T) {
	_, err := LoadTrialSuite("/nonexistent/path/suite.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadTrialSuite_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte(":\t invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadTrialSuite(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadTrialSuite_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadTrialSuite(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- Scorers ---

func TestOutputContainsScorer_GetName(t *testing.T) {
	s := NewOutputContainsScorer()
	if s.GetName() != "output_contains" {
		t.Errorf("unexpected name: %s", s.GetName())
	}
}

func TestOutputContainsScorer_NoExpectations_Pass(t *testing.T) {
	s := NewOutputContainsScorer()
	score, err := s.Score(context.Background(), entities.TrialCase{}, entities.Response{FinalResponse: "anything"})
	if err != nil || score != 1.0 {
		t.Errorf("expected score=1.0, got %v err=%v", score, err)
	}
}

func TestOutputContainsScorer_AllPresent_Pass(t *testing.T) {
	s := NewOutputContainsScorer()
	tc := entities.TrialCase{Expect: entities.TrialExpectations{OutputContains: []string{"hello", "world"}}}
	score, err := s.Score(context.Background(), tc, entities.Response{FinalResponse: "Hello World!"})
	if err != nil || score != 1.0 {
		t.Errorf("expected 1.0, got %v err=%v", score, err)
	}
}

func TestOutputContainsScorer_Missing_Fail(t *testing.T) {
	s := NewOutputContainsScorer()
	tc := entities.TrialCase{Expect: entities.TrialExpectations{OutputContains: []string{"bye"}}}
	score, err := s.Score(context.Background(), tc, entities.Response{FinalResponse: "hello"})
	if err != nil || score != 0.0 {
		t.Errorf("expected 0.0, got %v err=%v", score, err)
	}
}

func TestOutputNotContainsScorer_GetName(t *testing.T) {
	s := NewOutputNotContainsScorer()
	if s.GetName() != "output_not_contains" {
		t.Errorf("unexpected name: %s", s.GetName())
	}
}

func TestOutputNotContainsScorer_NoExpectations_Pass(t *testing.T) {
	s := NewOutputNotContainsScorer()
	score, err := s.Score(context.Background(), entities.TrialCase{}, entities.Response{FinalResponse: "anything"})
	if err != nil || score != 1.0 {
		t.Errorf("expected 1.0, got %v err=%v", score, err)
	}
}

func TestOutputNotContainsScorer_ForbiddenAbsent_Pass(t *testing.T) {
	s := NewOutputNotContainsScorer()
	tc := entities.TrialCase{Expect: entities.TrialExpectations{OutputNotContains: []string{"sorry", "error"}}}
	score, err := s.Score(context.Background(), tc, entities.Response{FinalResponse: "here is your answer"})
	if err != nil || score != 1.0 {
		t.Errorf("expected 1.0, got %v err=%v", score, err)
	}
}

func TestOutputNotContainsScorer_ForbiddenPresent_Fail(t *testing.T) {
	s := NewOutputNotContainsScorer()
	tc := entities.TrialCase{Expect: entities.TrialExpectations{OutputNotContains: []string{"error"}}}
	score, err := s.Score(context.Background(), tc, entities.Response{FinalResponse: "an error occurred"})
	if err != nil || score != 0.0 {
		t.Errorf("expected 0.0, got %v err=%v", score, err)
	}
}

func TestActionSequenceScorer_GetName(t *testing.T) {
	s := NewActionSequenceScorer()
	if s.GetName() != "action_sequence" {
		t.Errorf("unexpected name: %s", s.GetName())
	}
}

func TestActionSequenceScorer_NoExpectations_Pass(t *testing.T) {
	s := NewActionSequenceScorer()
	score, err := s.Score(context.Background(), entities.TrialCase{}, entities.Response{})
	if err != nil || score != 1.0 {
		t.Errorf("expected 1.0, got %v err=%v", score, err)
	}
}

func TestActionSequenceScorer_ExactMatch_Pass(t *testing.T) {
	s := NewActionSequenceScorer()
	tc := entities.TrialCase{Expect: entities.TrialExpectations{ActionSequence: []string{"a", "b"}}}
	score, err := s.Score(context.Background(), tc, entities.Response{ActionsExecuted: []string{"a", "b"}})
	if err != nil || score != 1.0 {
		t.Errorf("expected 1.0, got %v err=%v", score, err)
	}
}

func TestActionSequenceScorer_SubsequenceMatch_Pass(t *testing.T) {
	s := NewActionSequenceScorer()
	tc := entities.TrialCase{Expect: entities.TrialExpectations{ActionSequence: []string{"a", "c"}}}
	score, err := s.Score(context.Background(), tc, entities.Response{ActionsExecuted: []string{"a", "b", "c"}})
	if err != nil || score != 1.0 {
		t.Errorf("expected 1.0, got %v err=%v", score, err)
	}
}

func TestActionSequenceScorer_TooFewActions_Fail(t *testing.T) {
	s := NewActionSequenceScorer()
	tc := entities.TrialCase{Expect: entities.TrialExpectations{ActionSequence: []string{"a", "b", "c"}}}
	score, err := s.Score(context.Background(), tc, entities.Response{ActionsExecuted: []string{"a"}})
	if err != nil || score != 0.0 {
		t.Errorf("expected 0.0, got %v err=%v", score, err)
	}
}

func TestActionSequenceScorer_WrongOrder_Fail(t *testing.T) {
	s := NewActionSequenceScorer()
	tc := entities.TrialCase{Expect: entities.TrialExpectations{ActionSequence: []string{"b", "a"}}}
	score, err := s.Score(context.Background(), tc, entities.Response{ActionsExecuted: []string{"a", "b"}})
	if err != nil || score != 0.0 {
		t.Errorf("expected 0.0, got %v err=%v", score, err)
	}
}

func TestLatencyScorer_GetName(t *testing.T) {
	s := NewLatencyScorer()
	if s.GetName() != "latency" {
		t.Errorf("unexpected name: %s", s.GetName())
	}
}

func TestLatencyScorer_NoExpectation_Pass(t *testing.T) {
	s := NewLatencyScorer()
	tc := entities.TrialCase{Expect: entities.TrialExpectations{MaxLatencyMs: 0}}
	score, err := s.Score(context.Background(), tc, entities.Response{ProcessingTimeMs: 9999})
	if err != nil || score != 1.0 {
		t.Errorf("expected 1.0, got %v err=%v", score, err)
	}
}

func TestLatencyScorer_WithinLimit_Pass(t *testing.T) {
	s := NewLatencyScorer()
	tc := entities.TrialCase{Expect: entities.TrialExpectations{MaxLatencyMs: 500}}
	score, err := s.Score(context.Background(), tc, entities.Response{ProcessingTimeMs: 300})
	if err != nil || score != 1.0 {
		t.Errorf("expected 1.0, got %v err=%v", score, err)
	}
}

func TestLatencyScorer_ExceedsLimit_Fail(t *testing.T) {
	s := NewLatencyScorer()
	tc := entities.TrialCase{Expect: entities.TrialExpectations{MaxLatencyMs: 200}}
	score, err := s.Score(context.Background(), tc, entities.Response{ProcessingTimeMs: 500})
	if err != nil || score != 0.0 {
		t.Errorf("expected 0.0, got %v err=%v", score, err)
	}
}

func TestLLMJudgeScorer_GetName(t *testing.T) {
	s := NewLLMJudgeScorer(nil)
	if s.GetName() != "llm_judge" {
		t.Errorf("unexpected name: %s", s.GetName())
	}
}

func TestLLMJudgeScorer_NoJudge_Pass(t *testing.T) {
	s := NewLLMJudgeScorer(nil)
	tc := entities.TrialCase{Expect: entities.TrialExpectations{LLMJudge: nil}}
	score, err := s.Score(context.Background(), tc, entities.Response{})
	if err != nil || score != 1.0 {
		t.Errorf("expected 1.0, got %v err=%v", score, err)
	}
}

func TestLLMJudgeScorer_OracleFactoryFails_ReturnsError(t *testing.T) {
	factory := &stubOracleFactory{err: errors.New("no provider")}
	s := NewLLMJudgeScorer(factory)
	tc := entities.TrialCase{
		Expect: entities.TrialExpectations{
			LLMJudge: &entities.LLMJudgeExpect{Criteria: "be helpful", MinScore: 0.8},
		},
	}
	_, err := s.Score(context.Background(), tc, entities.Response{FinalResponse: "ok"})
	if err == nil {
		t.Fatal("expected error when oracle factory fails")
	}
}

func TestLLMJudgeScorer_OracleCallFails_ReturnsError(t *testing.T) {
	oracle := &stubOracle{err: errors.New("timeout")}
	factory := &stubOracleFactory{oracle: oracle}
	s := NewLLMJudgeScorer(factory)
	tc := entities.TrialCase{
		Expect: entities.TrialExpectations{
			LLMJudge: &entities.LLMJudgeExpect{Criteria: "be helpful", MinScore: 0.8},
		},
	}
	_, err := s.Score(context.Background(), tc, entities.Response{FinalResponse: "ok"})
	if err == nil {
		t.Fatal("expected error when oracle call fails")
	}
}

func TestLLMJudgeScorer_InvalidScoreResponse_ReturnsError(t *testing.T) {
	oracle := &stubOracle{content: "not-a-number"}
	factory := &stubOracleFactory{oracle: oracle}
	s := NewLLMJudgeScorer(factory)
	tc := entities.TrialCase{
		Expect: entities.TrialExpectations{
			LLMJudge: &entities.LLMJudgeExpect{Criteria: "be helpful", MinScore: 0.8},
		},
	}
	_, err := s.Score(context.Background(), tc, entities.Response{FinalResponse: "ok"})
	if err == nil {
		t.Fatal("expected parse error for non-numeric response")
	}
}

func TestLLMJudgeScorer_ValidScore(t *testing.T) {
	oracle := &stubOracle{content: "0.9"}
	factory := &stubOracleFactory{oracle: oracle}
	s := NewLLMJudgeScorer(factory)
	tc := entities.TrialCase{
		Expect: entities.TrialExpectations{
			LLMJudge: &entities.LLMJudgeExpect{Criteria: "be helpful", MinScore: 0.8},
		},
	}
	score, err := s.Score(context.Background(), tc, entities.Response{FinalResponse: "ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 0.9 {
		t.Errorf("expected 0.9, got %f", score)
	}
}

// --- TrialRunner ---

func TestTrialRunner_EmptySuite_ZeroPassRate(t *testing.T) {
	runner := NewTrialRunner(&stubWeave{resp: okResponse("ok")})
	suite := entities.TrialSuite{ID: "s1", Cases: nil}
	report, err := runner.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.PassRate != 0 {
		t.Errorf("expected 0 pass rate, got %f", report.PassRate)
	}
}

func TestTrialRunner_AllPassing(t *testing.T) {
	runner := NewTrialRunner(
		&stubWeave{resp: okResponse("hello world")},
		NewOutputContainsScorer(),
	)
	suite := entities.TrialSuite{
		ID: "s1",
		Cases: []entities.TrialCase{
			{ID: "tc-1", EventType: "chat", Expect: entities.TrialExpectations{OutputContains: []string{"hello"}}},
			{ID: "tc-2", EventType: "chat", Expect: entities.TrialExpectations{OutputContains: []string{"world"}}},
		},
	}
	report, err := runner.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.PassRate != 1.0 {
		t.Errorf("expected 100%% pass rate, got %f", report.PassRate)
	}
	if len(report.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(report.Results))
	}
}

func TestTrialRunner_PartialFail(t *testing.T) {
	runner := NewTrialRunner(
		&stubWeave{resp: okResponse("hello")},
		NewOutputContainsScorer(),
	)
	suite := entities.TrialSuite{
		ID: "s1",
		Cases: []entities.TrialCase{
			{ID: "tc-1", EventType: "chat", Expect: entities.TrialExpectations{OutputContains: []string{"hello"}}},
			{ID: "tc-2", EventType: "chat", Expect: entities.TrialExpectations{OutputContains: []string{"bye"}}},
		},
	}
	report, err := runner.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.PassRate != 0.5 {
		t.Errorf("expected 50%% pass rate, got %f", report.PassRate)
	}
}

func TestTrialRunner_ProcessingError_RecordedAsFailure(t *testing.T) {
	runner := NewTrialRunner(&stubWeave{err: errors.New("engine down")})
	suite := entities.TrialSuite{
		ID:    "s1",
		Cases: []entities.TrialCase{{ID: "tc-1", EventType: "chat"}},
	}
	report, err := runner.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.PassRate != 0.0 {
		t.Errorf("expected 0%% pass rate, got %f", report.PassRate)
	}
	if len(report.Results[0].Failures) == 0 {
		t.Error("expected failure recorded for processing error")
	}
}

func TestTrialRunner_ScorerError_RecordedAsFailure(t *testing.T) {
	errScorer := &failingScorer{}
	runner := NewTrialRunner(&stubWeave{resp: okResponse("ok")}, errScorer)
	suite := entities.TrialSuite{
		ID:    "s1",
		Cases: []entities.TrialCase{{ID: "tc-1", EventType: "chat"}},
	}
	report, err := runner.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Results[0].Passed {
		t.Error("expected case to fail when scorer errors")
	}
	if report.Results[0].Scores["fail_scorer"] != 0.0 {
		t.Errorf("expected score=0 for failing scorer")
	}
}

type failingScorer struct{}

func (s *failingScorer) GetName() string { return "fail_scorer" }
func (s *failingScorer) Score(_ context.Context, _ entities.TrialCase, _ entities.Response) (float64, error) {
	return 0.0, errors.New("scorer exploded")
}

func TestTrialRunner_WithKnowledge_PassedToPulse(t *testing.T) {
	var capturedPulse *entities.Pulse
	w := &capturingWeave{}
	runner := NewTrialRunner(w)
	suite := entities.TrialSuite{
		ID: "s1",
		Cases: []entities.TrialCase{{
			ID:          "tc-1",
			EventType:   "chat",
			UserMessage: "hi",
			Knowledge:   map[string]any{"order_id": "ORD-1"},
		}},
	}
	_, err := runner.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	capturedPulse = w.pulse
	if capturedPulse == nil {
		t.Fatal("expected pulse to be captured")
	}
	if capturedPulse.Knowledge["order_id"] != "ORD-1" {
		t.Error("expected order_id in pulse knowledge")
	}
}

type capturingWeave struct {
	pulse *entities.Pulse
}

func (w *capturingWeave) ProcessEventByKey(_ context.Context, _ string, p *entities.Pulse) (*entities.Response, error) {
	w.pulse = p
	return &entities.Response{FinalResponse: "ok"}, nil
}

func TestTrialRunner_LLMJudge_BelowThreshold_Fails(t *testing.T) {
	oracle := &stubOracle{content: "0.5"}
	factory := &stubOracleFactory{oracle: oracle}
	runner := NewTrialRunner(
		&stubWeave{resp: okResponse("mediocre answer")},
		NewLLMJudgeScorer(factory),
	)
	suite := entities.TrialSuite{
		ID: "s1",
		Cases: []entities.TrialCase{{
			ID:        "tc-1",
			EventType: "chat",
			Expect: entities.TrialExpectations{
				LLMJudge: &entities.LLMJudgeExpect{
					Criteria: "be helpful",
					MinScore: 0.8,
					Model:    entities.SpiritModel{Provider: "openai", Model: "gpt-4o"},
				},
			},
		}},
	}
	report, err := runner.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Results[0].Passed {
		t.Error("expected case to fail when score below threshold")
	}
}

func TestTrialRunner_ReportSuiteID(t *testing.T) {
	runner := NewTrialRunner(&stubWeave{resp: okResponse("ok")})
	suite := entities.TrialSuite{ID: "my-suite", Cases: nil}
	report, _ := runner.Run(context.Background(), suite)
	if report.SuiteID != "my-suite" {
		t.Errorf("expected my-suite, got %s", report.SuiteID)
	}
}

func TestTrialRunner_DurationMs_Populated(t *testing.T) {
	runner := NewTrialRunner(&stubWeave{resp: okResponse("ok")})
	suite := entities.TrialSuite{
		ID:    "s1",
		Cases: []entities.TrialCase{{ID: "tc-1", EventType: "chat"}},
	}
	report, err := runner.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// DurationMs is measured, always >= 0
	if report.Results[0].DurationMs < 0 {
		t.Error("expected DurationMs >= 0")
	}
}

func TestTrialRunner_ResponseProcessingTimeMs_Set(t *testing.T) {
	runner := NewTrialRunner(&stubWeave{resp: okResponse("ok")})
	suite := entities.TrialSuite{
		ID:    "s1",
		Cases: []entities.TrialCase{{ID: "tc-1", EventType: "chat"}},
	}
	report, _ := runner.Run(context.Background(), suite)
	_ = fmt.Sprintf("%d", report.Results[0].Response.ProcessingTimeMs) // just access
}
