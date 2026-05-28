package http

import (
	"fmt"
	"strings"

	eywa "github.com/wmulabs/eywa"
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func (v *ValidationErrors) Add(field, message string) {
	v.Errors = append(v.Errors, ValidationError{Field: field, Message: message})
}

func (v *ValidationErrors) HasErrors() bool {
	return len(v.Errors) > 0
}

func (v *ValidationErrors) Error() string {
	var msgs []string
	for _, err := range v.Errors {
		msgs = append(msgs, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return strings.Join(msgs, "; ")
}

func ValidateSpiritCreate(name, systemPrompt string, model eywa.SpiritModel) *ValidationErrors {
	errs := &ValidationErrors{}
	validateSpiritName(name, errs)
	validateSystemPrompt(systemPrompt, errs)
	validateModelConfig(model, errs)
	return errs
}

func ValidateSpiritUpdate(systemPrompt string, model eywa.SpiritModel) *ValidationErrors {
	errs := &ValidationErrors{}
	validateSystemPrompt(systemPrompt, errs)
	validateModelConfig(model, errs)
	return errs
}

func ValidateVersion(version int) *ValidationErrors {
	errs := &ValidationErrors{}
	if version <= 0 {
		errs.Add("version", fmt.Sprintf("version must be greater than 0 (got: %d)", version))
	}
	return errs
}

func validateSpiritName(name string, errs *ValidationErrors) {
	if strings.TrimSpace(name) == "" {
		errs.Add("name", "name is required and cannot be empty")
		return
	}
	if len(name) > 100 {
		errs.Add("name", "name cannot exceed 100 characters")
	}
	if !isValidSpiritNameFormat(name) {
		errs.Add("name", "name can only contain lowercase letters, numbers, and underscores")
	}
}

func validateSystemPrompt(systemPrompt string, errs *ValidationErrors) {
	if strings.TrimSpace(systemPrompt) == "" {
		errs.Add("system_prompt", "system_prompt is required and cannot be empty")
		return
	}
	if len(systemPrompt) < 10 {
		errs.Add("system_prompt", "system_prompt must be at least 10 characters")
	}
}

func validateModelConfig(config eywa.SpiritModel, errs *ValidationErrors) {
	validateProvider(config.Provider, errs)
	validateModel(config.Model, errs)
	validateTemperature(config.Temperature, errs)
	validateMaxTokens(config.MaxTokens, errs)
	validateTopP(config.TopP, errs)
}

func validateProvider(provider string, errs *ValidationErrors) {
	if strings.TrimSpace(provider) == "" {
		errs.Add("model_config.provider", "provider is required and cannot be empty")
		return
	}
	valid := map[string]bool{
		"openai":    true,
		"anthropic": true,
		"gemini":    true,
		"bedrock":   true,
		"vertexai":  true,
		// OpenAI-compatible providers
		"ollama":     true,
		"groq":       true,
		"mistral":    true,
		"together":   true,
		"openrouter": true,
		"xai":        true,
	}
	if !valid[strings.ToLower(provider)] {
		errs.Add("model_config.provider", fmt.Sprintf("provider must be one of: openai, anthropic, gemini, bedrock, vertexai, ollama, groq, mistral, together, openrouter, xai (got: %s)", provider))
	}
}

func validateModel(model string, errs *ValidationErrors) {
	if strings.TrimSpace(model) == "" {
		errs.Add("model_config.model", "model is required and cannot be empty")
	}
}

func validateTemperature(temperature float64, errs *ValidationErrors) {
	if temperature < 0.0 || temperature > 2.0 {
		errs.Add("model_config.temp", fmt.Sprintf("temperature must be between 0.0 and 2.0 (got: %.2f)", temperature))
	}
}

func validateMaxTokens(maxTokens int, errs *ValidationErrors) {
	if maxTokens < 0 {
		errs.Add("model_config.max_tokens", "max_tokens cannot be negative")
	}
}

func validateTopP(topP float64, errs *ValidationErrors) {
	if topP != 0 && (topP < 0.0 || topP > 1.0) {
		errs.Add("model_config.top_p", fmt.Sprintf("top_p must be between 0.0 and 1.0 (got: %.2f)", topP))
	}
}

func isValidSpiritNameFormat(name string) bool {
	if len(name) == 0 {
		return false
	}
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_') {
			return false
		}
	}
	return true
}
