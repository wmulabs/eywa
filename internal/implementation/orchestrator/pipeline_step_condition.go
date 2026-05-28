package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"go.uber.org/zap"
)

// ConditionEvaluationStep evaluates NotifierConfig.Conditions against Pulse.Knowledge.
// All conditions must pass (AND logic). On failure, state.Skipped is set to true
// and the pipeline continues — the Pulse is audited as "skipped", not failed.
// Only active for notifier Spirits.
type ConditionEvaluationStep struct {
	timeout time.Duration
	logger  *zap.SugaredLogger
}

func NewConditionEvaluationStep(timeout time.Duration, logger *zap.SugaredLogger) *ConditionEvaluationStep {
	return &ConditionEvaluationStep{timeout: timeout, logger: logger}
}

func (s *ConditionEvaluationStep) Name() string           { return "ConditionEvaluation" }
func (s *ConditionEvaluationStep) Timeout() time.Duration { return s.timeout }

func (s *ConditionEvaluationStep) Execute(ctx context.Context, state *ProcessingState) error {
	if state.Spirit == nil || !state.Spirit.IsNotifier() {
		return nil
	}
	if len(state.Spirit.NotifierConfig.Conditions) == 0 {
		return nil
	}

	for _, cond := range state.Spirit.NotifierConfig.Conditions {
		val := resolveKnowledgeField(state.Event, cond.Field)
		if !evaluateCondition(cond, val) {
			reason := fmt.Sprintf("condition failed: %s (%v) does not satisfy %s %v",
				cond.Field, val, cond.Operator, cond.Value)
			s.logger.Infow("notifier condition not met, skipping delivery",
				"reason", reason,
				"memory_key", state.Event.MemoryKey)
			state.Skipped = true
			state.ProcessingStatus = "skipped"
			return nil
		}
	}

	return nil
}

func resolveKnowledgeField(pulse *entities.Pulse, field string) any {
	parts := strings.SplitN(field, ".", 2)
	top := parts[0]

	switch top {
	case "knowledge":
		if len(parts) == 2 {
			return pulse.Knowledge[parts[1]]
		}
		return pulse.Knowledge
	case "user_message":
		return pulse.UserMessage
	case "event_type":
		return pulse.EventType
	case "source":
		return pulse.Source
	case "contact_phone":
		return pulse.ContactPhone
	default:
		val, ok := pulse.Knowledge[field]
		if !ok {
			return nil
		}
		return val
	}
}

func evaluateCondition(cond entities.NotifierCondition, actual any) bool {
	if actual == nil {
		return cond.Operator == "not_exists"
	}

	switch cond.Operator {
	case "exists":
		return true
	case "not_exists":
		return false
	case "eq":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", cond.Value)
	case "neq":
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", cond.Value)
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", actual), fmt.Sprintf("%v", cond.Value))
	case "gt", "gte", "lt", "lte":
		return compareNumeric(cond.Operator, actual, cond.Value)
	case "in":
		return isInSlice(actual, cond.Value)
	case "not_in":
		return !isInSlice(actual, cond.Value)
	default:
		return false
	}
}

func compareNumeric(op string, actual, expected any) bool {
	a, okA := toFloat(actual)
	b, okB := toFloat(expected)
	if !okA || !okB {
		return false
	}
	switch op {
	case "gt":
		return a > b
	case "gte":
		return a >= b
	case "lt":
		return a < b
	case "lte":
		return a <= b
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	}
	return 0, false
}

func isInSlice(actual, expected any) bool {
	actualStr := fmt.Sprintf("%v", actual)
	switch s := expected.(type) {
	case []any:
		for _, v := range s {
			if fmt.Sprintf("%v", v) == actualStr {
				return true
			}
		}
	case []string:
		for _, v := range s {
			if v == actualStr {
				return true
			}
		}
	}
	return false
}
