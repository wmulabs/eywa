package trial

import (
	"context"
	"fmt"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type Weave interface {
	ProcessEventByKey(ctx context.Context, eventKey string, event *entities.Pulse) (*entities.Response, error)
}

type TrialRunner struct {
	weave   Weave
	scorers []ports.Scorer
}

func NewTrialRunner(weave Weave, scorers ...ports.Scorer) *TrialRunner {
	return &TrialRunner{weave: weave, scorers: scorers}
}

func (r *TrialRunner) Run(ctx context.Context, suite entities.TrialSuite) (entities.TrialReport, error) {
	report := entities.TrialReport{
		SuiteID: suite.ID,
		RunAt:   time.Now().UTC(),
		Results: make([]entities.TrialResult, 0, len(suite.Cases)),
	}

	passed := 0

	for _, tc := range suite.Cases {
		result := r.runCase(ctx, tc)
		report.Results = append(report.Results, result)
		if result.Passed {
			passed++
		}
	}

	if len(suite.Cases) > 0 {
		report.PassRate = float64(passed) / float64(len(suite.Cases))
	}

	return report, nil
}

func (r *TrialRunner) runCase(ctx context.Context, tc entities.TrialCase) entities.TrialResult {
	sessionKey := entities.MemoryKey{Channel: "trial", User: tc.ID}
	pulse := entities.NewPulse(sessionKey).
		WithEventType(tc.EventType).
		WithUserMessage(tc.UserMessage).
		Build()

	for k, v := range tc.Knowledge {
		pulse.Knowledge[k] = v
	}

	start := time.Now()
	resp, err := r.weave.ProcessEventByKey(ctx, tc.EventType, pulse)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		return entities.TrialResult{
			CaseID:     tc.ID,
			Passed:     false,
			Failures:   []string{fmt.Sprintf("processing error: %v", err)},
			DurationMs: durationMs,
		}
	}

	resp.ProcessingTimeMs = durationMs

	scores := make(map[string]float64)
	var failures []string

	for _, scorer := range r.scorers {
		score, scoreErr := scorer.Score(ctx, tc, *resp)
		if scoreErr != nil {
			failures = append(failures, fmt.Sprintf("[%s] error: %v", scorer.GetName(), scoreErr))
			scores[scorer.GetName()] = 0.0
			continue
		}
		scores[scorer.GetName()] = score

		threshold := 1.0
		if tc.Expect.LLMJudge != nil && scorer.GetName() == "llm_judge" {
			threshold = tc.Expect.LLMJudge.MinScore
		}

		if score < threshold {
			failures = append(failures, fmt.Sprintf("[%s] score %.2f below threshold %.2f", scorer.GetName(), score, threshold))
		}
	}

	return entities.TrialResult{
		CaseID:     tc.ID,
		Passed:     len(failures) == 0,
		Scores:     scores,
		Failures:   failures,
		Response:   *resp,
		DurationMs: durationMs,
	}
}
