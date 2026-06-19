package orchestrator

import (
	"context"
	"encoding/json"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// memoEntry records a successful Action result keyed by its (name, arguments) signature. On a durable
// turn it lets an identical call reuse the prior result instead of re-executing — so a side effect that
// already happened (a message sent, a charge made) is not repeated when a turn resumes after a crash.
type memoEntry struct {
	Result          string `json:"result"`
	IsVoiceDelivery bool   `json:"is_voice_delivery,omitempty"`
}

// turnSnapshot is the serializable state of an in-progress reasoning turn. It carries only data — the
// provider, emitter, and policies are re-injected by newTurnState on resume. The audit IterationLogs
// are intentionally excluded: they overlap heavily with WorkingContext and would bloat the record; the
// pre-crash attempt's audit lives in its own (failed) Chronicle entry.
type turnSnapshot struct {
	IterationsUsed     int                   `json:"iterations_used"`
	WorkingContext     []ports.OracleMessage `json:"working_context"`
	ActiveTopic        string                `json:"active_topic,omitempty"`
	ConversationOffset int                   `json:"conversation_offset"`
	BannedSignatures   []string              `json:"banned_signatures,omitempty"`
	InfraTerminal      bool                  `json:"infra_terminal,omitempty"`
	TotalActionCalls   int                   `json:"total_action_calls,omitempty"`
	IterBoundaries     []int                 `json:"iter_boundaries,omitempty"`
	TopicSwitched      bool                  `json:"topic_switched,omitempty"`
	ReflectionRounds   int                   `json:"reflection_rounds,omitempty"`
	GroundingRevised   bool                  `json:"grounding_revised,omitempty"`
	PlanNudged         bool                  `json:"plan_nudged,omitempty"`
	CriticalErrors     int                   `json:"critical_errors,omitempty"`
	PlanItems          []entities.PlanItem   `json:"plan_items,omitempty"`

	// Carried across the resume boundary so spend already paid is reflected in the final result and a
	// final answer already delivered pre-crash is not delivered twice.
	TokensUsed        ports.OracleUsage            `json:"tokens_used"`
	TokensByModel     map[string]ports.OracleUsage `json:"tokens_by_model,omitempty"`
	ResponseDelivered bool                         `json:"response_delivered,omitempty"`

	// ToolResults memoizes successful Action calls so a resumed turn reuses them instead of re-running.
	ToolResults map[string]memoEntry `json:"tool_results,omitempty"`
}

func (ts *turnState) snapshot() turnSnapshot {
	banned := make([]string, 0, len(ts.bannedSignatures))
	for sig := range ts.bannedSignatures {
		banned = append(banned, sig)
	}

	snap := turnSnapshot{
		IterationsUsed:     ts.result.IterationsUsed,
		WorkingContext:     ts.workingContext,
		ActiveTopic:        ts.activeTopic,
		ConversationOffset: ts.conversationOffset,
		BannedSignatures:   banned,
		InfraTerminal:      ts.infraTerminal,
		TotalActionCalls:   ts.totalActionCalls,
		IterBoundaries:     ts.iterBoundaries,
		TopicSwitched:      ts.topicSwitched,
		ReflectionRounds:   ts.reflectionRounds,
		GroundingRevised:   ts.groundingRevised,
		PlanNudged:         ts.planNudged,
		CriticalErrors:     ts.criticalErrors,
		TokensUsed:         ts.result.TokensUsed,
		TokensByModel:      ts.result.TokensByModel,
		ResponseDelivered:  ts.result.ResponseDelivered,
		ToolResults:        ts.toolMemo,
	}
	if ts.plan != nil {
		snap.PlanItems = ts.plan.items
	}
	return snap
}

func (ts *turnState) restore(snap turnSnapshot) {
	ts.workingContext = snap.WorkingContext
	ts.activeTopic = snap.ActiveTopic
	ts.conversationOffset = snap.ConversationOffset
	ts.infraTerminal = snap.InfraTerminal
	ts.totalActionCalls = snap.TotalActionCalls
	ts.iterBoundaries = snap.IterBoundaries
	ts.topicSwitched = snap.TopicSwitched
	ts.reflectionRounds = snap.ReflectionRounds
	ts.groundingRevised = snap.GroundingRevised
	ts.planNudged = snap.PlanNudged
	ts.criticalErrors = snap.CriticalErrors

	ts.bannedSignatures = make(map[string]bool, len(snap.BannedSignatures))
	for _, sig := range snap.BannedSignatures {
		ts.bannedSignatures[sig] = true
	}

	// plan is non-nil only when the plan policy is enabled for this turn; if it changed between crash
	// and resume the loaded items are simply dropped.
	if ts.plan != nil {
		ts.plan.items = snap.PlanItems
	}

	ts.toolMemo = snap.ToolResults

	ts.result.IterationsUsed = snap.IterationsUsed
	ts.result.WorkingContext = snap.WorkingContext
	ts.result.TokensUsed = snap.TokensUsed
	ts.result.TokensByModel = snap.TokensByModel
	ts.result.ResponseDelivered = snap.ResponseDelivered
}

// checkpointID derives the stable per-turn key from the Pulse. It uses the idempotency key (stable
// across redeliveries) and falls back to the event ID. durable is false when no store is wired or no
// stable key exists, in which case the loop runs with zero checkpointing overhead.
func (r *ReasoningService) checkpointID(req *ReasoningRequest) (turnID string, durable bool) {
	if r.checkpointStore == nil || req.Event == nil {
		return "", false
	}
	id := req.Event.IdempotencyKey
	if id == "" {
		id = req.Event.ID
	}
	if id == "" {
		return "", false
	}
	return id, true
}

// resumeFromCheckpoint loads and restores a prior checkpoint into ts. It fails open: any load or decode
// error logs a warning and starts the turn fresh rather than aborting.
func (r *ReasoningService) resumeFromCheckpoint(ctx context.Context, turnID string, ts *turnState) bool {
	data, found, err := r.checkpointStore.Load(ctx, turnID)
	if err != nil {
		r.logger.Warnw("checkpoint load failed, starting turn fresh", "turn_id", turnID, "error", err)
		return false
	}
	if !found {
		return false
	}
	var snap turnSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		r.logger.Warnw("checkpoint decode failed, starting turn fresh", "turn_id", turnID, "error", err)
		return false
	}
	ts.restore(snap)
	return true
}

// saveCheckpoint persists the current turn state. Best-effort: an encode or store failure logs a
// warning and is swallowed so durability never breaks the synchronous turn.
func (r *ReasoningService) saveCheckpoint(ctx context.Context, turnID string, ts *turnState) {
	data, err := json.Marshal(ts.snapshot())
	if err != nil {
		r.logger.Warnw("checkpoint encode failed, skipping save", "turn_id", turnID, "error", err)
		return
	}
	if err := r.checkpointStore.Save(ctx, turnID, data); err != nil {
		r.logger.Warnw("checkpoint save failed", "turn_id", turnID, "error", err)
	}
}

func (r *ReasoningService) deleteCheckpoint(ctx context.Context, turnID string) {
	if err := r.checkpointStore.Delete(ctx, turnID); err != nil {
		r.logger.Warnw("checkpoint delete failed", "turn_id", turnID, "error", err)
	}
}
