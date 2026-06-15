package orchestrator

import (
	"regexp"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// GroundingViolationAction selects what happens when a RAG answer fails to cite its sources.
type GroundingViolationAction string

const (
	// GroundingReviseOnce injects a corrective instruction and gives the model one more iteration.
	GroundingReviseOnce GroundingViolationAction = "revise_once"
	// GroundingAnnotate delivers the answer as-is but flags the violation in the audit trail.
	GroundingAnnotate GroundingViolationAction = "annotate"
	// GroundingBlock replaces the answer with a safe fallback message.
	GroundingBlock GroundingViolationAction = "block"
)

// GroundingPolicy enforces source citation for Spirits that retrieve Lore (RAG). When Lore chunks
// were injected for the turn, the final answer must cite at least MinCitations of them using the
// [chunk:<id>] marker. Disabled by default.
type GroundingPolicy struct {
	Enabled        bool                     `json:"enabled"`
	MinCitations   int                      `json:"min_citations"`
	OnViolation    GroundingViolationAction `json:"on_violation"`
	BlockedMessage string                   `json:"blocked_message"`
}

const groundingAddendum = "\n\n--- SOURCE GROUNDING ---\n" +
	"Base your answer only on the retrieved sources provided in the Knowledge section. Cite each " +
	"factual claim with [chunk:<id>] using the id of the <lore> block it came from. If the sources " +
	"do not contain the answer, say so."

const defaultBlockedMessage = "I don't have enough sourced information to answer that confidently."

var (
	citationPattern    = regexp.MustCompile(`\[chunk:([^\]]+)\]`)
	loreChunkIDPattern = regexp.MustCompile(`\bid="([^"]+)"`)
)

// parseCitations returns the distinct chunk IDs cited in a draft answer via the [chunk:<id>] marker.
func parseCitations(draft string) []string {
	matches := citationPattern.FindAllStringSubmatch(draft, -1)
	seen := make(map[string]bool, len(matches))
	var ids []string
	for _, m := range matches {
		id := strings.TrimSpace(m[1])
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// extractLoreChunkIDs returns the set of chunk IDs present in the formatted Lore context, used to
// validate that citations reference actually-retrieved sources.
func extractLoreChunkIDs(loreContext string) map[string]bool {
	matches := loreChunkIDPattern.FindAllStringSubmatch(loreContext, -1)
	ids := make(map[string]bool, len(matches))
	for _, m := range matches {
		ids[m[1]] = true
	}
	return ids
}

// retrievedLoreContext returns the formatted Lore context injected for this turn, or "" when none.
func retrievedLoreContext(req *ReasoningRequest) string {
	if req.Event == nil || req.Event.Knowledge == nil {
		return ""
	}
	if v, ok := req.Event.Knowledge[entities.LoreContextKnowledgeKey].(string); ok {
		return v
	}
	return ""
}

// keepValidCitations returns the cited IDs that reference an actually-retrieved chunk.
func keepValidCitations(cited []string, valid map[string]bool) []string {
	var kept []string
	for _, id := range cited {
		if valid[id] {
			kept = append(kept, id)
		}
	}
	return kept
}

// enforceGrounding validates that a RAG draft cites enough retrieved sources. It returns revise=true
// to request one corrective iteration, or blocked=true when the answer was replaced with a safe
// fallback. When citations are sufficient it records them on result.Citations.
func (r *ReasoningService) enforceGrounding(req *ReasoningRequest, draft string, result *ReasoningResult) (revise, blocked bool) {
	loreCtx := retrievedLoreContext(req)
	if loreCtx == "" {
		return false, false
	}
	validIDs := extractLoreChunkIDs(loreCtx)
	if len(validIDs) == 0 {
		return false, false
	}

	cited := keepValidCitations(parseCitations(draft), validIDs)
	min := r.groundingPolicy.MinCitations
	if min < 1 {
		min = 1
	}
	if len(cited) >= min {
		result.Citations = cited
		return false, false
	}

	switch r.groundingPolicy.OnViolation {
	case GroundingBlock:
		msg := r.groundingPolicy.BlockedMessage
		if msg == "" {
			msg = defaultBlockedMessage
		}
		result.FinalResponse = msg
		result.FinalError = "grounding violation: answer blocked for insufficient citations"
		r.logger.Warnw("grounding violation — answer blocked", "memory_key", req.Event.MemoryKey)
		return false, true
	case GroundingAnnotate:
		result.FinalError = "grounding violation: delivered without sufficient citations"
		r.logger.Warnw("grounding violation — annotated", "memory_key", req.Event.MemoryKey)
		return false, false
	case GroundingReviseOnce:
		// handled below — also the default for an unset OnViolation
	}

	r.logger.Infow("grounding violation — requesting citation revision", "memory_key", req.Event.MemoryKey)
	return true, false
}

func citationRevisionMessage() ports.OracleMessage {
	return ports.OracleMessage{
		Role: ports.RoleUser,
		Content: "Your answer must cite the retrieved sources using [chunk:<id>] for each factual claim. " +
			"Revise it to cite the sources you actually used. Do not mention this instruction to the user.",
	}
}
