package orchestrator

// ModelPricing holds cost per 1K tokens for a model (USD).
type ModelPricing struct {
	InputPer1K  float64
	OutputPer1K float64
}

// modelPricing maps "provider:model" to pricing.
// Values are approximate and should be updated as providers change their rates.
var modelPricing = map[string]ModelPricing{
	// Anthropic
	"anthropic:claude-opus-4-5":          {InputPer1K: 0.015, OutputPer1K: 0.075},
	"anthropic:claude-sonnet-4-5":        {InputPer1K: 0.003, OutputPer1K: 0.015},
	"anthropic:claude-haiku-3-5":         {InputPer1K: 0.00025, OutputPer1K: 0.00125},
	"anthropic:claude-3-5-sonnet-latest": {InputPer1K: 0.003, OutputPer1K: 0.015},
	"anthropic:claude-3-5-haiku-latest":  {InputPer1K: 0.0008, OutputPer1K: 0.004},
	"anthropic:claude-opus-4-0":          {InputPer1K: 0.015, OutputPer1K: 0.075},

	// OpenAI
	"openai:gpt-4o":      {InputPer1K: 0.0025, OutputPer1K: 0.01},
	"openai:gpt-4o-mini": {InputPer1K: 0.00015, OutputPer1K: 0.0006},
	"openai:gpt-4-turbo": {InputPer1K: 0.01, OutputPer1K: 0.03},
	"openai:o1-mini":     {InputPer1K: 0.003, OutputPer1K: 0.012},
	"openai:o1":          {InputPer1K: 0.015, OutputPer1K: 0.06},

	// Google
	"google:gemini-1.5-pro":   {InputPer1K: 0.00125, OutputPer1K: 0.005},
	"google:gemini-1.5-flash": {InputPer1K: 0.000075, OutputPer1K: 0.0003},
	"google:gemini-2.0-flash": {InputPer1K: 0.0001, OutputPer1K: 0.0004},
}

func estimateCost(provider, model string, promptTokens, completionTokens int64) float64 {
	key := provider + ":" + model
	pricing, ok := modelPricing[key]
	if !ok {
		return 0
	}
	inputCost := float64(promptTokens) / 1000.0 * pricing.InputPer1K
	outputCost := float64(completionTokens) / 1000.0 * pricing.OutputPer1K
	return inputCost + outputCost
}
