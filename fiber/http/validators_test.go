package http

import (
	"strings"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

// --- ValidationErrors ---

func TestValidationErrors_AddAndHasErrors(t *testing.T) {
	v := &ValidationErrors{}
	if v.HasErrors() {
		t.Fatal("expected no errors initially")
	}
	v.Add("field1", "msg1")
	v.Add("field2", "msg2")
	if !v.HasErrors() {
		t.Fatal("expected errors after Add")
	}
	if len(v.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(v.Errors))
	}
}

func TestValidationErrors_Error_Format(t *testing.T) {
	v := &ValidationErrors{}
	v.Add("name", "required")
	v.Add("provider", "invalid")
	got := v.Error()
	if !strings.Contains(got, "name: required") {
		t.Errorf("expected 'name: required' in %q", got)
	}
	if !strings.Contains(got, "provider: invalid") {
		t.Errorf("expected 'provider: invalid' in %q", got)
	}
}

// --- ValidateSpiritCreate ---

func TestValidateSpiritCreate_Valid(t *testing.T) {
	model := eywa.SpiritModel{Provider: "openai", Model: "gpt-4o", Temperature: 0.7}
	errs := ValidateSpiritCreate("my_bot", "You are a helpful assistant.", model)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %s", errs.Error())
	}
}

func TestValidateSpiritCreate_EmptyName(t *testing.T) {
	model := eywa.SpiritModel{Provider: "openai", Model: "gpt-4o"}
	errs := ValidateSpiritCreate("", "You are a helpful assistant.", model)
	if !errs.HasErrors() {
		t.Fatal("expected error for empty name")
	}
}

func TestValidateSpiritCreate_NameTooLong(t *testing.T) {
	longName := strings.Repeat("a", 101)
	model := eywa.SpiritModel{Provider: "openai", Model: "gpt-4o"}
	errs := ValidateSpiritCreate(longName, "You are a helpful assistant.", model)
	if !errs.HasErrors() {
		t.Fatal("expected error for name > 100 chars")
	}
}

func TestValidateSpiritCreate_InvalidNameFormat(t *testing.T) {
	model := eywa.SpiritModel{Provider: "openai", Model: "gpt-4o"}
	errs := ValidateSpiritCreate("My Bot!", "You are a helpful assistant.", model)
	if !errs.HasErrors() {
		t.Fatal("expected error for name with invalid chars")
	}
}

func TestValidateSpiritCreate_ShortSystemPrompt(t *testing.T) {
	model := eywa.SpiritModel{Provider: "openai", Model: "gpt-4o"}
	errs := ValidateSpiritCreate("bot", "short", model)
	if !errs.HasErrors() {
		t.Fatal("expected error for prompt < 10 chars")
	}
}

func TestValidateSpiritCreate_EmptySystemPrompt(t *testing.T) {
	model := eywa.SpiritModel{Provider: "openai", Model: "gpt-4o"}
	errs := ValidateSpiritCreate("bot", "   ", model)
	if !errs.HasErrors() {
		t.Fatal("expected error for blank system prompt")
	}
}

// --- ValidateSpiritUpdate ---

func TestValidateSpiritUpdate_Valid(t *testing.T) {
	model := eywa.SpiritModel{Provider: "anthropic", Model: "claude-3-5-sonnet-20241022"}
	errs := ValidateSpiritUpdate("You are a helpful assistant.", model)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %s", errs.Error())
	}
}

func TestValidateSpiritUpdate_InvalidModel(t *testing.T) {
	model := eywa.SpiritModel{Provider: "openai", Model: ""}
	errs := ValidateSpiritUpdate("You are a helpful assistant.", model)
	if !errs.HasErrors() {
		t.Fatal("expected error for empty model")
	}
}

// --- ValidateVersion ---

func TestValidateVersion_Valid(t *testing.T) {
	errs := ValidateVersion(1)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %s", errs.Error())
	}
}

func TestValidateVersion_Zero(t *testing.T) {
	errs := ValidateVersion(0)
	if !errs.HasErrors() {
		t.Fatal("expected error for version=0")
	}
}

func TestValidateVersion_Negative(t *testing.T) {
	errs := ValidateVersion(-1)
	if !errs.HasErrors() {
		t.Fatal("expected error for negative version")
	}
}

// --- validateProvider ---

func TestValidateProvider_InvalidProvider(t *testing.T) {
	model := eywa.SpiritModel{Provider: "unknown_llm", Model: "x"}
	errs := ValidateSpiritCreate("bot", "You are a helpful assistant.", model)
	if !errs.HasErrors() {
		t.Fatal("expected error for unknown provider")
	}
}

func TestValidateProvider_EmptyProvider(t *testing.T) {
	model := eywa.SpiritModel{Provider: "", Model: "gpt-4o"}
	errs := ValidateSpiritCreate("bot", "You are a helpful assistant.", model)
	if !errs.HasErrors() {
		t.Fatal("expected error for empty provider")
	}
}

func TestValidateProvider_AllValid(t *testing.T) {
	providers := []string{"openai", "anthropic", "gemini", "bedrock", "vertexai",
		"ollama", "groq", "mistral", "together", "openrouter", "xai"}
	for _, p := range providers {
		t.Run(p, func(t *testing.T) {
			model := eywa.SpiritModel{Provider: p, Model: "some-model"}
			errs := ValidateSpiritCreate("bot", "You are a helpful assistant.", model)
			hasProviderErr := false
			for _, e := range errs.Errors {
				if e.Field == "model_config.provider" {
					hasProviderErr = true
				}
			}
			if hasProviderErr {
				t.Errorf("provider %q should be valid", p)
			}
		})
	}
}

// --- validateTemperature ---

func TestValidateTemperature_OutOfRange(t *testing.T) {
	model := eywa.SpiritModel{Provider: "openai", Model: "gpt-4o", Temperature: 2.5}
	errs := ValidateSpiritCreate("bot", "You are a helpful assistant.", model)
	if !errs.HasErrors() {
		t.Fatal("expected error for temperature > 2.0")
	}
}

func TestValidateTemperature_Negative(t *testing.T) {
	model := eywa.SpiritModel{Provider: "openai", Model: "gpt-4o", Temperature: -0.1}
	errs := ValidateSpiritCreate("bot", "You are a helpful assistant.", model)
	if !errs.HasErrors() {
		t.Fatal("expected error for negative temperature")
	}
}

// --- validateMaxTokens ---

func TestValidateMaxTokens_Negative(t *testing.T) {
	model := eywa.SpiritModel{Provider: "openai", Model: "gpt-4o", MaxTokens: -1}
	errs := ValidateSpiritCreate("bot", "You are a helpful assistant.", model)
	if !errs.HasErrors() {
		t.Fatal("expected error for negative max_tokens")
	}
}

// --- validateTopP ---

func TestValidateTopP_OutOfRange(t *testing.T) {
	model := eywa.SpiritModel{Provider: "openai", Model: "gpt-4o", TopP: 1.5}
	errs := ValidateSpiritCreate("bot", "You are a helpful assistant.", model)
	if !errs.HasErrors() {
		t.Fatal("expected error for top_p > 1.0")
	}
}

func TestValidateTopP_ZeroAllowed(t *testing.T) {
	model := eywa.SpiritModel{Provider: "openai", Model: "gpt-4o", TopP: 0}
	errs := ValidateSpiritCreate("bot", "You are a helpful assistant.", model)
	hasTopPErr := false
	for _, e := range errs.Errors {
		if e.Field == "model_config.top_p" {
			hasTopPErr = true
		}
	}
	if hasTopPErr {
		t.Error("top_p=0 should be allowed (means unset)")
	}
}

// --- isValidSpiritNameFormat ---

func TestIsValidSpiritNameFormat(t *testing.T) {
	cases := []struct {
		name  string
		input string
		valid bool
	}{
		{"lowercase", "my_bot", true},
		{"numbers", "bot123", true},
		{"underscore", "my_bot_2", true},
		{"uppercase", "MyBot", false},
		{"space", "my bot", false},
		{"dash", "my-bot", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidSpiritNameFormat(tc.input)
			if got != tc.valid {
				t.Errorf("isValidSpiritNameFormat(%q) = %v, want %v", tc.input, got, tc.valid)
			}
		})
	}
}
