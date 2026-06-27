package orchestrator

import (
	"context"
	"regexp"
	"time"

	"github.com/wmulabs/eywa/internal/helpers"
	"go.uber.org/zap"
)

const defaultOutputBlockedMessage = "I can't share that response."

// OutputGuardStep sanitises the final response before it is persisted, delivered, and audited.
// It blocks responses matching a denylist (replacing them wholesale) and redacts PII. It runs after
// the Notification step so it also covers any notifier-produced response that is stored or audited.
type OutputGuardStep struct {
	redactPII      bool
	piiKinds       []helpers.PIIKind
	mask           string
	blocked        []*regexp.Regexp
	blockedMessage string
	timeout        time.Duration
	logger         *zap.SugaredLogger
}

func NewOutputGuardStep(cfg OutputGuardConfig, timeout time.Duration, logger *zap.SugaredLogger) *OutputGuardStep {
	mask := cfg.RedactionMask
	if mask == "" {
		mask = "[REDACTED]"
	}
	blockedMessage := cfg.BlockedMessage
	if blockedMessage == "" {
		blockedMessage = defaultOutputBlockedMessage
	}

	// Patterns are validated by WeaveConfig.Validate; any that still fail to compile are skipped
	// rather than panicking the pipeline.
	blocked := make([]*regexp.Regexp, 0, len(cfg.BlockedPatterns))
	for _, p := range cfg.BlockedPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			logger.Warnw("output guard skipping invalid blocked pattern", "pattern", p, "error", err)
			continue
		}
		blocked = append(blocked, re)
	}

	return &OutputGuardStep{
		redactPII:      cfg.RedactPII,
		piiKinds:       cfg.PIIKinds,
		mask:           mask,
		blocked:        blocked,
		blockedMessage: blockedMessage,
		timeout:        timeout,
		logger:         logger,
	}
}

func (s *OutputGuardStep) Name() string           { return "OutputGuard" }
func (s *OutputGuardStep) Timeout() time.Duration { return s.timeout }

func (s *OutputGuardStep) Execute(_ context.Context, state *ProcessingState) error {
	if state.Response == "" {
		return nil
	}

	for _, re := range s.blocked {
		if re.MatchString(state.Response) {
			s.logger.Warnw("output guard blocked response",
				"memory_key", state.Event.MemoryKey,
				"pattern", re.String())
			state.Response = s.blockedMessage
			return nil
		}
	}

	if s.redactPII {
		redacted, found := helpers.RedactPII(state.Response, s.piiKinds, s.mask)
		if len(found) > 0 {
			state.Response = redacted
			s.logger.Infow("output guard redacted PII",
				"memory_key", state.Event.MemoryKey,
				"kinds", found)
		}
	}

	return nil
}
