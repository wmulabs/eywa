package trial

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// OutputContainsScorer passes when all expected substrings appear in the LLM response.
type OutputContainsScorer struct{}

func NewOutputContainsScorer() ports.Scorer { return &OutputContainsScorer{} }

func (s *OutputContainsScorer) GetName() string { return "output_contains" }

func (s *OutputContainsScorer) Score(_ context.Context, tc entities.TrialCase, result entities.Response) (float64, error) {
	if len(tc.Expect.OutputContains) == 0 {
		return 1.0, nil
	}
	text := strings.ToLower(result.FinalResponse)
	for _, expected := range tc.Expect.OutputContains {
		if !strings.Contains(text, strings.ToLower(expected)) {
			return 0.0, nil
		}
	}
	return 1.0, nil
}

// OutputNotContainsScorer passes when none of the forbidden substrings appear in the response.
type OutputNotContainsScorer struct{}

func NewOutputNotContainsScorer() ports.Scorer { return &OutputNotContainsScorer{} }

func (s *OutputNotContainsScorer) GetName() string { return "output_not_contains" }

func (s *OutputNotContainsScorer) Score(_ context.Context, tc entities.TrialCase, result entities.Response) (float64, error) {
	if len(tc.Expect.OutputNotContains) == 0 {
		return 1.0, nil
	}
	text := strings.ToLower(result.FinalResponse)
	for _, forbidden := range tc.Expect.OutputNotContains {
		if strings.Contains(text, strings.ToLower(forbidden)) {
			return 0.0, nil
		}
	}
	return 1.0, nil
}

// ActionSequenceScorer passes when the executed actions match the expected sequence in order.
type ActionSequenceScorer struct{}

func NewActionSequenceScorer() ports.Scorer { return &ActionSequenceScorer{} }

func (s *ActionSequenceScorer) GetName() string { return "action_sequence" }

func (s *ActionSequenceScorer) Score(_ context.Context, tc entities.TrialCase, result entities.Response) (float64, error) {
	if len(tc.Expect.ActionSequence) == 0 {
		return 1.0, nil
	}
	expected := tc.Expect.ActionSequence
	actual := result.ActionsExecuted

	if len(actual) < len(expected) {
		return 0.0, nil
	}

	// Subsequence match — expected must appear in order within actual.
	ei := 0
	for _, act := range actual {
		if ei < len(expected) && act == expected[ei] {
			ei++
		}
	}
	if ei < len(expected) {
		return 0.0, nil
	}
	return 1.0, nil
}

// LatencyScorer passes when processing time is within the configured limit.
type LatencyScorer struct{}

func NewLatencyScorer() ports.Scorer { return &LatencyScorer{} }

func (s *LatencyScorer) GetName() string { return "latency" }

func (s *LatencyScorer) Score(_ context.Context, tc entities.TrialCase, result entities.Response) (float64, error) {
	if tc.Expect.MaxLatencyMs <= 0 {
		return 1.0, nil
	}
	if result.ProcessingTimeMs <= int64(tc.Expect.MaxLatencyMs) {
		return 1.0, nil
	}
	return 0.0, nil
}

// LLMJudgeScorer calls an Oracle to evaluate the response against qualitative criteria.
type LLMJudgeScorer struct {
	factory ports.OracleFactory
}

func NewLLMJudgeScorer(factory ports.OracleFactory) ports.Scorer {
	return &LLMJudgeScorer{factory: factory}
}

func (s *LLMJudgeScorer) GetName() string { return "llm_judge" }

func (s *LLMJudgeScorer) Score(ctx context.Context, tc entities.TrialCase, result entities.Response) (float64, error) {
	if tc.Expect.LLMJudge == nil {
		return 1.0, nil
	}

	judge := tc.Expect.LLMJudge

	prompt := fmt.Sprintf(`You are an evaluator. Score the assistant response on a scale from 0.0 to 1.0.

Criteria: %s

User message: %s

Assistant response: %s

Respond with only a number between 0.0 and 1.0.`, judge.Criteria, tc.UserMessage, result.FinalResponse)

	req := &ports.OracleRequest{
		Model: judge.Model.Model,
		Messages: []ports.OracleMessage{
			{Role: ports.RoleUser, Content: prompt},
		},
		MaxTokens:   64,
		Temperature: 0.0,
	}

	oracle, err := s.factory.GetProvider(judge.Model.Provider)
	if err != nil {
		return 0.0, fmt.Errorf("llm judge oracle: %w", err)
	}

	resp, err := oracle.GenerateResponse(ctx, req)
	if err != nil {
		return 0.0, fmt.Errorf("llm judge call: %w", err)
	}

	score, err := strconv.ParseFloat(strings.TrimSpace(resp.Content), 64)
	if err != nil {
		return 0.0, fmt.Errorf("llm judge parse score %q: %w", resp.Content, err)
	}

	return score, nil
}
