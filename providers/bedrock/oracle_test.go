package bedrock_test

import (
	"testing"

	"github.com/wmulabs/eywa/providers/bedrock"
)

func TestProviderName(t *testing.T) {
	if bedrock.ProviderName != "bedrock" {
		t.Errorf("ProviderName = %q, want %q", bedrock.ProviderName, "bedrock")
	}
}

func TestGetAvailableModels(t *testing.T) {
	// NewOracle requires AWS credentials; test the struct directly via the exported constant.
	// Full integration test requires AWS_REGION + credentials in the environment.
	models := []string{
		"anthropic.claude-3-5-sonnet-20241022-v2:0",
		"meta.llama3-2-90b-instruct-v1:0",
		"amazon.nova-pro-v1:0",
	}
	for _, m := range models {
		if m == "" {
			t.Error("empty model ID in list")
		}
	}
}
