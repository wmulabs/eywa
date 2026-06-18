package trial

import (
	"context"
	"fmt"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// ChronicleReader is the read-only slice of the Chronicle (audit log) that replay needs. The MongoDB
// ChronicleRepository satisfies it; segregated so replay never touches the write path.
type ChronicleReader interface {
	FindByMemoryKey(ctx context.Context, memoryKey string) ([]*entities.Chronicle, error)
	FindByTimeRange(ctx context.Context, start, end time.Time) ([]*entities.Chronicle, error)
}

// ReplayOptions selects which recorded interactions to replay. Provide a MemoryKey to replay one
// conversation, or a time range to replay a window. MemoryKey takes precedence when both are set.
type ReplayOptions struct {
	MemoryKey string
	Start     time.Time
	End       time.Time
}

// LoadReplayCases reads recorded interactions from the Chronicle and converts them into TrialCases,
// each carrying the original user input and the recorded answer as Baseline. Feed the cases to a
// TrialRunner to re-run real history against the current Weave (e.g. a new Spirit version) and score
// for regressions. Interactions with no user input are skipped — there is nothing to replay.
func LoadReplayCases(ctx context.Context, reader ChronicleReader, opts ReplayOptions) ([]entities.TrialCase, error) {
	var (
		chronicles []*entities.Chronicle
		err        error
	)
	switch {
	case opts.MemoryKey != "":
		chronicles, err = reader.FindByMemoryKey(ctx, opts.MemoryKey)
	case !opts.Start.IsZero() || !opts.End.IsZero():
		chronicles, err = reader.FindByTimeRange(ctx, opts.Start, opts.End)
	default:
		return nil, fmt.Errorf("replay: provide a MemoryKey or a time range")
	}
	if err != nil {
		return nil, fmt.Errorf("replay: loading chronicle: %w", err)
	}

	return CasesFromChronicles(chronicles), nil
}

// CasesFromChronicles converts recorded interactions into replayable TrialCases. It is pure (no I/O),
// so callers that already hold Chronicle entries can convert them directly.
func CasesFromChronicles(chronicles []*entities.Chronicle) []entities.TrialCase {
	cases := make([]entities.TrialCase, 0, len(chronicles))
	for _, c := range chronicles {
		if c == nil || c.Event.UserInput == "" {
			continue
		}
		cases = append(cases, entities.TrialCase{
			ID:          c.ID,
			Name:        fmt.Sprintf("replay %s", c.ID),
			EventType:   c.EventKey,
			UserMessage: c.Event.UserInput,
			Baseline:    c.Processing.FinalResponse,
		})
	}
	return cases
}
