package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

// CompressionPolicy bounds the reasoning working-context size. When the context exceeds
// MaxContextChars, the oldest completed iterations are summarized into a single "evidence ledger"
// message while the most recent KeepRecent iterations are kept verbatim. Disabled by default.
type CompressionPolicy struct {
	Enabled         bool `json:"enabled"`
	MaxContextChars int  `json:"max_context_chars"`
	KeepRecent      int  `json:"keep_recent"`
}

const ledgerInstruction = "Compress the following agent tool interactions into a dense evidence " +
	"ledger. Preserve every concrete fact, identifier, value, decision, and unresolved error. Drop " +
	"verbosity. Output only a bulleted ledger an agent can act on."

const ledgerPrefix = "[EVIDENCE LEDGER]\n"

func contextChars(messages []ports.OracleMessage) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content)
	}
	return total
}

func renderMessagesForLedger(messages []ports.OracleMessage) string {
	var b strings.Builder
	for _, m := range messages {
		label := string(m.Role)
		if m.ToolName != "" {
			label += "/" + m.ToolName
		}
		fmt.Fprintf(&b, "%s: %s\n", label, m.Content)
	}
	return b.String()
}

// maybeCompress collapses the compressible span of the working context (everything between the
// pinned prefix at conversationOffset and the most recent KeepRecent iterations) into one ledger
// message. It returns the (possibly unchanged) working context and the iteration-boundary list to
// continue tracking from. The pinned prefix and conversationOffset are preserved.
func (r *ReasoningService) maybeCompress(
	ctx context.Context,
	provider ports.Oracle,
	req *ReasoningRequest,
	workingContext []ports.OracleMessage,
	conversationOffset int,
	boundaries []int,
	result *ReasoningResult,
) ([]ports.OracleMessage, []int) {
	if contextChars(workingContext) <= r.compressionPolicy.MaxContextChars {
		return workingContext, boundaries
	}

	keep := r.compressionPolicy.KeepRecent
	if keep < 1 {
		keep = 1
	}
	if len(boundaries) <= keep {
		return workingContext, boundaries
	}

	recentStart := boundaries[len(boundaries)-keep]
	if recentStart <= conversationOffset {
		return workingContext, boundaries
	}

	compressible := workingContext[conversationOffset:recentStart]
	summary, usage, err := r.summarizeMessages(ctx, provider, req, compressible)
	if err != nil || strings.TrimSpace(summary) == "" {
		r.logger.Warnw("working-context compression failed, keeping full context", "error", err)
		return workingContext, boundaries
	}
	result.accumulateTokens(usage)

	ledger := ports.OracleMessage{Role: ports.RoleAssistant, Content: ledgerPrefix + summary}
	recent := workingContext[recentStart:]

	compressed := make([]ports.OracleMessage, 0, conversationOffset+1+len(recent))
	compressed = append(compressed, workingContext[:conversationOffset]...)
	compressed = append(compressed, ledger)
	compressed = append(compressed, recent...)

	r.logger.Infow("compressed reasoning working context",
		"memory_key", req.Event.MemoryKey,
		"before_messages", len(workingContext),
		"after_messages", len(compressed),
	)

	// Reset boundary tracking to the new baseline; future iterations append fresh boundaries.
	return compressed, []int{len(compressed)}
}

func (r *ReasoningService) summarizeMessages(
	ctx context.Context,
	provider ports.Oracle,
	req *ReasoningRequest,
	messages []ports.OracleMessage,
) (string, ports.OracleUsage, error) {
	resp, err := provider.GenerateResponse(ctx, &ports.OracleRequest{
		Model:        req.Spirit.ModelConfig.Model,
		SystemPrompt: ledgerInstruction,
		Messages:     []ports.OracleMessage{{Role: ports.RoleUser, Content: renderMessagesForLedger(messages)}},
		Temperature:  0,
	})
	if err != nil {
		return "", ports.OracleUsage{}, fmt.Errorf("compress working context: %w", err)
	}
	return resp.Content, resp.TokensUsed, nil
}
