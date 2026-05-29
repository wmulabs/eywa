package gemini

import (
	"fmt"

	eywa "github.com/wmulabs/eywa"
	"google.golang.org/genai"
)

const defaultModel = "gemini-3-flash-preview"

func extractTextAndUsage(resp *genai.GenerateContentResponse) (string, eywa.OracleUsage, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return "", eywa.OracleUsage{}, fmt.Errorf("empty response")
	}

	c := resp.Candidates[0]
	if c.Content == nil || len(c.Content.Parts) == 0 {
		return "", eywa.OracleUsage{}, fmt.Errorf("no content in response")
	}

	var text string
	for _, part := range c.Content.Parts {
		if part.Text != "" {
			text = part.Text
			break
		}
	}

	if text == "" {
		return "", eywa.OracleUsage{}, fmt.Errorf("no text in response")
	}

	var usage eywa.OracleUsage
	if m := resp.UsageMetadata; m != nil {
		usage.PromptTokens = int(m.PromptTokenCount)
		usage.CompletionTokens = int(m.CandidatesTokenCount)
		usage.TotalTokens = int(m.TotalTokenCount)
	}

	return text, usage, nil
}
