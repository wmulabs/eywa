package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

const toolSummarizeInstruction = "Summarize the following tool result for an AI agent that will " +
	"continue a task. Preserve every concrete fact, identifier, numeric value, and error. Be dense " +
	"and drop filler. Output only the summary."

// shapeResultForContext bounds an Action result before it enters the reasoning context.
// It returns the full result when shaping is disabled or the result already fits. For the Summarize
// strategy it asks the Oracle to condense the result (tokens accounted into result), falling back to
// truncation on any error or empty summary.
func (r *ReasoningService) shapeResultForContext(
	ctx context.Context,
	provider ports.Oracle,
	req *ReasoningRequest,
	actionName, full string,
	result *ReasoningResult,
) string {
	limits := r.limitsFor(actionName)
	if limits.MaxChars <= 0 || len([]rune(full)) <= limits.MaxChars {
		return full
	}

	if limits.Strategy == ports.ToolShapeSummarize && provider != nil && req != nil {
		if summary, usage, err := r.summarizeToolResult(ctx, provider, req, full); err == nil && strings.TrimSpace(summary) != "" {
			result.accumulateTokens(usage)
			return truncateToolResult(summary, limits)
		}
		r.logger.Warnw("tool-result summarize failed, falling back to truncation", "action", actionName)
	}

	return truncateToolResult(full, limits)
}

func (r *ReasoningService) summarizeToolResult(
	ctx context.Context,
	provider ports.Oracle,
	req *ReasoningRequest,
	full string,
) (string, ports.OracleUsage, error) {
	resp, err := provider.GenerateResponse(ctx, &ports.OracleRequest{
		Model:        req.Spirit.ModelConfig.Model,
		SystemPrompt: toolSummarizeInstruction,
		Messages:     []ports.OracleMessage{{Role: ports.RoleUser, Content: full}},
		Temperature:  0,
	})
	if err != nil {
		return "", ports.OracleUsage{}, fmt.Errorf("summarize tool result: %w", err)
	}
	return resp.Content, resp.TokensUsed, nil
}

// truncateToolResult bounds an Action result to limits.MaxChars by keeping a head and tail and
// replacing the dropped middle with a notice. It is rune-safe and a no-op when MaxChars is zero
// or the content already fits.
func truncateToolResult(content string, limits ports.ToolResultLimits) string {
	if limits.MaxChars <= 0 {
		return content
	}

	runes := []rune(content)
	if len(runes) <= limits.MaxChars {
		return content
	}

	keepHead, keepTail := resolveHeadTail(limits)
	omitted := len(runes) - (keepHead + keepTail)

	return string(runes[:keepHead]) + truncationNotice(omitted) + string(runes[len(runes)-keepTail:])
}

// resolveHeadTail derives the head/tail rune counts, defaulting to a 70/30 split of MaxChars and
// clamping so their sum never exceeds MaxChars.
func resolveHeadTail(limits ports.ToolResultLimits) (head, tail int) {
	head, tail = limits.KeepHead, limits.KeepTail
	if head <= 0 && tail <= 0 {
		head = limits.MaxChars * 70 / 100
		tail = limits.MaxChars - head
	}
	if head+tail > limits.MaxChars {
		head = limits.MaxChars * 70 / 100
		tail = limits.MaxChars - head
	}
	return head, tail
}

func truncationNotice(omitted int) string {
	return fmt.Sprintf("\n...[result truncated: %d chars omitted; refine your query for detail]...\n", omitted)
}
