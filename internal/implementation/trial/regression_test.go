package trial

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

func regressionResult() entities.Response { return entities.Response{FinalResponse: "new answer"} }

func baselineCase() entities.TrialCase {
	return entities.TrialCase{ID: "c1", UserMessage: "q", Baseline: "old answer"}
}

func TestRegressionScorer_GetName(t *testing.T) {
	if NewRegressionScorer(nil, "openai", "gpt").GetName() != "regression" {
		t.Error("expected name 'regression'")
	}
}

func TestRegressionScorer_NoBaseline_Passes(t *testing.T) {
	// nil factory proves no Oracle call happens without a baseline.
	s := NewRegressionScorer(nil, "openai", "gpt")
	got, err := s.Score(context.Background(), entities.TrialCase{ID: "c1"}, regressionResult())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 1.0 {
		t.Errorf("expected 1.0 with no baseline, got %v", got)
	}
}

func TestRegressionScorer_Verdicts(t *testing.T) {
	cases := []struct {
		verdict string
		want    float64
	}{
		{"1", 1.0},
		{"0", 0.0},
		{"1.0", 1.0},
		{"0.8", 1.0},
		{"0.4", 0.0},
		{"0.5", 1.0},
	}
	for _, tc := range cases {
		t.Run(tc.verdict, func(t *testing.T) {
			factory := &stubOracleFactory{oracle: &stubOracle{content: tc.verdict}}
			got, err := NewRegressionScorer(factory, "openai", "gpt").Score(context.Background(), baselineCase(), regressionResult())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("verdict %q → %v, want %v", tc.verdict, got, tc.want)
			}
		})
	}
}

func TestRegressionScorer_FactoryError(t *testing.T) {
	factory := &stubOracleFactory{err: errors.New("no provider")}
	if _, err := NewRegressionScorer(factory, "openai", "gpt").Score(context.Background(), baselineCase(), regressionResult()); err == nil {
		t.Error("expected error when the factory has no provider")
	}
}

func TestRegressionScorer_OracleError(t *testing.T) {
	factory := &stubOracleFactory{oracle: &stubOracle{err: errors.New("api down")}}
	if _, err := NewRegressionScorer(factory, "openai", "gpt").Score(context.Background(), baselineCase(), regressionResult()); err == nil {
		t.Error("expected error when the oracle call fails")
	}
}

func TestRegressionScorer_UnparseableVerdict(t *testing.T) {
	factory := &stubOracleFactory{oracle: &stubOracle{content: "maybe"}}
	if _, err := NewRegressionScorer(factory, "openai", "gpt").Score(context.Background(), baselineCase(), regressionResult()); err == nil {
		t.Error("expected error when the verdict cannot be parsed")
	}
}
