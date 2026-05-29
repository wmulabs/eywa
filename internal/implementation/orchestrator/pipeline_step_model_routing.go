package orchestrator

import (
	"context"
	"strings"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"go.uber.org/zap"
)

// ModelRoutingStep evaluates ModelRoutingRules in order and replaces the Spirit's
// ModelConfig with the first matching rule's model. No-op when no rules are configured.
type ModelRoutingStep struct {
	rules   []entities.ModelRoutingRule
	timeout time.Duration
	logger  *zap.SugaredLogger
}

func NewModelRoutingStep(
	rules []entities.ModelRoutingRule,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *ModelRoutingStep {
	return &ModelRoutingStep{rules: rules, timeout: timeout, logger: logger}
}

func (s *ModelRoutingStep) Name() string           { return "ModelRouting" }
func (s *ModelRoutingStep) Timeout() time.Duration { return s.timeout }

func (s *ModelRoutingStep) Execute(ctx context.Context, state *ProcessingState) error {
	if len(s.rules) == 0 || state.Spirit == nil {
		return nil
	}

	for _, rule := range s.rules {
		if matchesRoutingCondition(rule.Condition, state) {
			s.logger.Infow("model routing rule matched",
				"rule", rule.Name,
				"from_model", state.Spirit.ModelConfig.Model,
				"to_model", rule.Model.Model,
				"memory_key", state.Event.MemoryKey)
			state.Spirit.ModelConfig = rule.Model
			return nil
		}
	}

	return nil
}

func matchesRoutingCondition(cond entities.ModelRoutingCondition, state *ProcessingState) bool {
	msg := state.Event.UserMessage

	if cond.InputLengthGte > 0 && len(msg) < cond.InputLengthGte {
		return false
	}
	if cond.InputLengthLt > 0 && len(msg) >= cond.InputLengthLt {
		return false
	}
	if cond.HasAttachments && len(state.Event.Attachments) == 0 {
		return false
	}
	if cond.EventType != "" && state.Event.EventType != cond.EventType {
		return false
	}
	if cond.TopicPrefix != "" && !strings.HasPrefix(state.Event.SubjectKey, cond.TopicPrefix) {
		return false
	}
	if cond.UserTier != "" {
		tier, _ := state.Event.Knowledge["user_tier"].(string)
		if tier != cond.UserTier {
			return false
		}
	}
	return true
}
