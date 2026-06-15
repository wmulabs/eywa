package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

// ReflectionPolicy enables a self-critique pass before a draft answer is delivered. When enabled and
// rounds remain, the model reviews its own draft; a "revise" verdict feeds the critique back for one
// more reasoning iteration. Disabled by default. Reflection always fails open — a critique that
// errors or cannot be parsed never blocks delivery.
type ReflectionPolicy struct {
	Enabled   bool     `json:"enabled"`
	MaxRounds int      `json:"max_rounds"`
	Model     string   `json:"model"`
	Criteria  []string `json:"criteria"`
}

const reflectionBaseInstruction = "You are reviewing a draft answer an agent is about to send to " +
	"the user. Check that it directly answers the user's request, that every factual claim is " +
	"supported by the tool results in the conversation, and that no tool error was ignored."

const reflectionVerdictRequest = "Review the draft answer above against the criteria. Reply with ONLY " +
	"a JSON object: {\"pass\": boolean, \"issues\": [string]}. Set pass=false only if the draft must " +
	"be revised."

// parseReflectionVerdict extracts the {pass, issues} verdict from a critique response. It tolerates
// surrounding prose/markdown and fails open (pass=true) on any parse failure, so a malformed verdict
// never blocks delivery.
func parseReflectionVerdict(content string) (pass bool, issues []string) {
	raw := extractJSONObject(content)
	if raw == "" {
		return true, nil
	}
	var verdict struct {
		Pass   bool     `json:"pass"`
		Issues []string `json:"issues"`
	}
	if err := json.Unmarshal([]byte(raw), &verdict); err != nil {
		return true, nil
	}
	return verdict.Pass, verdict.Issues
}

func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < start {
		return ""
	}
	return s[start : end+1]
}

// reflect runs one critique pass over the current draft. Returns the verdict and token usage.
// On any error it fails open (pass=true) so delivery is never blocked by a reflection failure.
func (r *ReasoningService) reflect(
	ctx context.Context,
	provider ports.Oracle,
	req *ReasoningRequest,
	workingContext []ports.OracleMessage,
) (pass bool, issues []string, usage ports.OracleUsage) {
	model := req.Spirit.ModelConfig.Model
	if r.reflectionPolicy.Model != "" {
		model = r.reflectionPolicy.Model
	}

	systemPrompt := reflectionBaseInstruction
	for _, c := range r.reflectionPolicy.Criteria {
		systemPrompt += "\n- " + c
	}

	messages := append([]ports.OracleMessage{}, workingContext...)
	messages = append(messages, ports.OracleMessage{Role: ports.RoleUser, Content: reflectionVerdictRequest})

	resp, err := provider.GenerateResponse(ctx, &ports.OracleRequest{
		Model:        model,
		SystemPrompt: systemPrompt,
		Messages:     messages,
		Temperature:  0,
	})
	if err != nil {
		r.logger.Warnw("reflection critique call failed, passing draft through", "error", err)
		return true, nil, ports.OracleUsage{}
	}
	pass, issues = parseReflectionVerdict(resp.Content)
	return pass, issues, resp.TokensUsed
}

func reflectionRevisionMessage(issues []string) ports.OracleMessage {
	return ports.OracleMessage{
		Role: ports.RoleUser,
		Content: fmt.Sprintf(
			"A self-review found issues with your draft answer: %s. Revise your answer to address them. "+
				"Do not mention this review to the user.",
			strings.Join(issues, "; "),
		),
	}
}
