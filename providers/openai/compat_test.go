package openai_test

import (
	"testing"

	eywaopenai "github.com/wmulabs/eywa/providers/openai"
)

func TestCompatOracleNames(t *testing.T) {
	cases := []struct {
		name   string
		oracle interface{ GetName() string }
		want   string
	}{
		{"ollama", eywaopenai.NewOllamaOracle("http://localhost:11434"), eywaopenai.ProviderNameOllama},
		{"groq", eywaopenai.NewGroqOracle("test-key"), eywaopenai.ProviderNameGroq},
		{"mistral", eywaopenai.NewMistralOracle("test-key"), eywaopenai.ProviderNameMistral},
		{"together", eywaopenai.NewTogetherOracle("test-key"), eywaopenai.ProviderNameTogether},
		{"openrouter", eywaopenai.NewOpenRouterOracle("test-key"), eywaopenai.ProviderNameOpenRouter},
		{"xai", eywaopenai.NewXAIOracle("test-key"), eywaopenai.ProviderNameXAI},
		{"openai default", eywaopenai.NewOracle("test-key"), eywaopenai.ProviderName},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.oracle.GetName(); got != tc.want {
				t.Errorf("GetName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCompatOracleCustomConfig(t *testing.T) {
	oracle := eywaopenai.NewOracleWithConfig(eywaopenai.Config{
		Name:    "my-custom-provider",
		APIKey:  "key",
		BaseURL: "http://my-server/v1",
	})

	if got := oracle.GetName(); got != "my-custom-provider" {
		t.Errorf("GetName() = %q, want %q", got, "my-custom-provider")
	}
}
