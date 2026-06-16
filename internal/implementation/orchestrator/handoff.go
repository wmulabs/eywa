package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

// handoffOperatorID marks an unassigned auto-raised seat that an operator console claims.
const handoffOperatorID = "system:auto-handoff"

const defaultHandoffSeatTTL = 30 * time.Minute

// vigilHandoffSink raises a human-takeover seat via the Vigil repository. It is the engine-side
// adapter for HandoffSink, keeping the reasoning service free of infrastructure imports.
type vigilHandoffSink struct {
	vigilRepo ports.VigilRepository
	ttl       time.Duration
}

func newVigilHandoffSink(vigilRepo ports.VigilRepository, ttl time.Duration) *vigilHandoffSink {
	if ttl <= 0 {
		ttl = defaultHandoffSeatTTL
	}
	return &vigilHandoffSink{vigilRepo: vigilRepo, ttl: ttl}
}

func (s *vigilHandoffSink) RaiseTakeover(ctx context.Context, memoryKey string) error {
	// Don't double-raise if a human already holds the seat.
	if existing, _ := s.vigilRepo.Get(ctx, memoryKey); existing != nil {
		return nil
	}
	if err := s.vigilRepo.Acquire(ctx, memoryKey, handoffOperatorID, s.ttl); err != nil {
		return fmt.Errorf("acquire vigil seat: %w", err)
	}
	return nil
}

// Confidence is a coarse, rule-based assessment of how trustworthy a turn's answer is.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

func confidenceRank(c Confidence) int {
	switch c {
	case ConfidenceLow:
		return 0
	case ConfidenceMedium:
		return 1
	case ConfidenceHigh:
		return 2
	default:
		return 1
	}
}

// HandoffMode selects what happens when a turn's confidence falls below the threshold.
type HandoffMode string

const (
	// HandoffRaiseVigil acquires a human-takeover seat and replies with a holding message.
	HandoffRaiseVigil HandoffMode = "raise_vigil"
	// HandoffAnnotateOnly delivers the answer but records the low confidence in the result.
	HandoffAnnotateOnly HandoffMode = "annotate"
)

// HandoffPolicy escalates low-confidence turns to a human instead of shipping a weak answer.
// Disabled by default. When the HandoffSink is unavailable it degrades to annotate-only.
type HandoffPolicy struct {
	Enabled            bool        `json:"enabled"`
	MinConfidence      Confidence  `json:"min_confidence"`
	Mode               HandoffMode `json:"mode"`
	HoldingMessage     string      `json:"holding_message"`
	UncertaintyMarkers []string    `json:"uncertainty_markers"`
}

// HandoffSink raises a human-in-the-loop takeover for a session. Implemented by the engine over the
// Vigil repository so the reasoning service stays infrastructure-agnostic.
type HandoffSink interface {
	RaiseTakeover(ctx context.Context, memoryKey string) error
}

const defaultHoldingMessage = "Thanks for your patience — a member of our team will follow up with you shortly."

// confidenceSignals are the per-turn observations feeding scoreConfidence.
type confidenceSignals struct {
	criticalErrors   int
	reflectionFailed bool
	grounded         bool
	uncertainContent bool
}

// scoreConfidence maps turn signals to a coarse confidence band. A clean turn is High; critical
// errors, a failed self-critique, or hedging language degrade it; grounding nudges it back up.
func scoreConfidence(s confidenceSignals) Confidence {
	score := -s.criticalErrors
	if s.reflectionFailed {
		score -= 2
	}
	if s.uncertainContent {
		score--
	}
	if s.grounded {
		score++
	}

	switch {
	case score >= 0:
		return ConfidenceHigh
	case score == -1:
		return ConfidenceMedium
	default:
		return ConfidenceLow
	}
}

// maybeHandoff scores the turn's confidence and, when below the policy threshold, raises a human
// takeover (RaiseVigil) or just records the low confidence (AnnotateOnly / degraded). It records
// result.Confidence either way. Returns the holding message and raised=true only when the turn
// should end with a takeover instead of the model's answer.
func (r *ReasoningService) maybeHandoff(
	ctx context.Context,
	req *ReasoningRequest,
	result *ReasoningResult,
	content string,
	criticalErrors, reflectionRounds int,
) (holding string, raised bool) {
	// Handoff is a conversational concern: only a conversational Spirit has a human conversation a
	// person could take over. Executors/notifiers never hand off, regardless of the policy.
	if !req.Spirit.IsConversational() {
		return "", false
	}

	sig := confidenceSignals{
		criticalErrors:   criticalErrors,
		reflectionFailed: r.reflectionPolicy.Enabled && r.reflectionPolicy.MaxRounds > 0 && reflectionRounds >= r.reflectionPolicy.MaxRounds,
		grounded:         len(result.Citations) > 0,
		uncertainContent: matchesUncertainty(content, r.handoffPolicy.UncertaintyMarkers),
	}
	conf := scoreConfidence(sig)
	result.Confidence = conf

	minConf := r.handoffPolicy.MinConfidence
	if minConf == "" {
		minConf = ConfidenceMedium // deliver autonomously only at Medium+; hand off on Low
	}
	if confidenceRank(conf) >= confidenceRank(minConf) {
		return "", false
	}

	// AnnotateOnly: keep the answer, just flag low confidence (already recorded above).
	if r.handoffPolicy.Mode == HandoffAnnotateOnly {
		r.logger.Infow("low-confidence turn annotated", "confidence", conf, "memory_key", req.Event.MemoryKey)
		return "", false
	}

	// RaiseVigil (default): need a sink; degrade to annotate if missing or it fails.
	if r.handoffSink == nil {
		r.logger.Warnw("handoff policy enabled but no sink wired; delivering with low-confidence annotation",
			"confidence", conf, "memory_key", req.Event.MemoryKey)
		return "", false
	}
	if err := r.handoffSink.RaiseTakeover(ctx, req.Event.MemoryKey); err != nil {
		r.logger.Warnw("failed to raise takeover; delivering with low-confidence annotation",
			"error", err, "memory_key", req.Event.MemoryKey)
		return "", false
	}

	result.HandoffRaised = true
	msg := r.handoffPolicy.HoldingMessage
	if msg == "" {
		msg = defaultHoldingMessage
	}
	r.logger.Infow("low-confidence turn handed off to a human", "confidence", conf, "memory_key", req.Event.MemoryKey)
	return msg, true
}

func matchesUncertainty(content string, markers []string) bool {
	lc := strings.ToLower(content)
	for _, m := range markers {
		if m != "" && strings.Contains(lc, strings.ToLower(m)) {
			return true
		}
	}
	return false
}
