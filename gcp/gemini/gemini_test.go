package gemini

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	eywa "github.com/wmulabs/eywa"
	"google.golang.org/genai"
)

// skipIfNoGCP skips the test when no GCP Application Default Credentials are
// discoverable. Constructors that call genai.NewClient resolve ADC at build time.
func skipIfNoGCP(t *testing.T) {
	t.Helper()
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" {
		return
	}
	home, _ := os.UserHomeDir()
	adc := filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
	if _, err := os.Stat(adc); err == nil {
		return
	}
	t.Skip("no GCP credentials: set GOOGLE_APPLICATION_CREDENTIALS or run `gcloud auth application-default login`")
}

// --- extractTextAndUsage ---

func TestExtractTextAndUsage_NilResponse_ReturnsError(t *testing.T) {
	_, _, err := extractTextAndUsage(nil)
	if err == nil {
		t.Error("expected error for nil response")
	}
}

func TestExtractTextAndUsage_EmptyCandidates_ReturnsError(t *testing.T) {
	_, _, err := extractTextAndUsage(&genai.GenerateContentResponse{})
	if err == nil {
		t.Error("expected error for empty candidates")
	}
}

func TestExtractTextAndUsage_NilContent_ReturnsError(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{Content: nil}},
	}
	_, _, err := extractTextAndUsage(resp)
	if err == nil {
		t.Error("expected error for nil content")
	}
}

func TestExtractTextAndUsage_EmptyParts_ReturnsError(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{Content: &genai.Content{Parts: []*genai.Part{}}},
		},
	}
	_, _, err := extractTextAndUsage(resp)
	if err == nil {
		t.Error("expected error for empty parts")
	}
}

func TestExtractTextAndUsage_NoTextPart_ReturnsError(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{Content: &genai.Content{
				Parts: []*genai.Part{
					{InlineData: &genai.Blob{MIMEType: "image/jpeg", Data: []byte("img")}},
				},
			}},
		},
	}
	_, _, err := extractTextAndUsage(resp)
	if err == nil {
		t.Error("expected error when no text part present")
	}
}

func TestExtractTextAndUsage_ValidResponse_ReturnsText(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{Content: &genai.Content{
				Parts: []*genai.Part{{Text: "hello world"}},
			}},
		},
	}
	text, usage, err := extractTextAndUsage(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "hello world" {
		t.Errorf("expected 'hello world', got %q", text)
	}
	if usage != (eywa.OracleUsage{}) {
		t.Errorf("expected zero usage, got %+v", usage)
	}
}

func TestExtractTextAndUsage_WithUsageMetadata(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{Content: &genai.Content{
				Parts: []*genai.Part{{Text: "answer"}},
			}},
		},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 5,
			TotalTokenCount:      15,
		},
	}
	_, usage, err := extractTextAndUsage(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.PromptTokens != 10 || usage.CompletionTokens != 5 || usage.TotalTokens != 15 {
		t.Errorf("unexpected usage: %+v", usage)
	}
}

func TestExtractTextAndUsage_FirstTextPartWins(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{Content: &genai.Content{
				Parts: []*genai.Part{
					{Text: "first"},
					{Text: "second"},
				},
			}},
		},
	}
	text, _, err := extractTextAndUsage(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "first" {
		t.Errorf("expected 'first', got %q", text)
	}
}

// --- GeminiAudioTranscriber guard clause ---

func TestTranscribe_EmptyAudioData_ReturnsError(t *testing.T) {
	tr := &GeminiAudioTranscriber{client: nil, model: defaultModel}
	_, _, err := tr.Transcribe(context.Background(), nil, "audio/ogg")
	if err == nil {
		t.Error("expected error for empty audio data")
	}
}

func TestNewGeminiAudioTranscriber_EmptyModel_SetsDefault(t *testing.T) {
	skipIfNoGCP(t)
	tr, err := NewGeminiAudioTranscriber(t.Context(), "test-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.model != defaultModel {
		t.Errorf("expected default model %q, got %q", defaultModel, tr.model)
	}
}

func TestNewGeminiAudioTranscriber_CustomModel(t *testing.T) {
	skipIfNoGCP(t)
	tr, err := NewGeminiAudioTranscriber(t.Context(), "test-key", "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.model != "gemini-2.5-flash" {
		t.Errorf("expected gemini-2.5-flash, got %q", tr.model)
	}
}

// --- GeminiImageAnalyzer guard clause ---

func TestAnalyze_EmptyImageData_ReturnsError(t *testing.T) {
	a := &GeminiImageAnalyzer{client: nil, model: defaultModel}
	_, _, err := a.Analyze(context.Background(), nil, "image/jpeg")
	if err == nil {
		t.Error("expected error for empty image data")
	}
}

func TestNewGeminiImageAnalyzer_EmptyModel_SetsDefault(t *testing.T) {
	skipIfNoGCP(t)
	a, err := NewGeminiImageAnalyzer(t.Context(), "test-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.model != defaultModel {
		t.Errorf("expected default model %q, got %q", defaultModel, a.model)
	}
}

func TestNewGeminiImageAnalyzer_CustomModel(t *testing.T) {
	skipIfNoGCP(t)
	a, err := NewGeminiImageAnalyzer(t.Context(), "test-key", "gemini-2.5-pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.model != "gemini-2.5-pro" {
		t.Errorf("expected gemini-2.5-pro, got %q", a.model)
	}
}

// --- GeminiDocumentExtractor guard clause ---

func TestExtract_EmptyDocumentData_ReturnsError(t *testing.T) {
	e := &GeminiDocumentExtractor{client: nil, model: defaultModel}
	_, _, err := e.Extract(context.Background(), nil, "application/pdf")
	if err == nil {
		t.Error("expected error for empty document data")
	}
}

func TestNewGeminiDocumentExtractor_EmptyModel_SetsDefault(t *testing.T) {
	e, err := NewGeminiDocumentExtractor("test-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.model != defaultModel {
		t.Errorf("expected default model %q, got %q", defaultModel, e.model)
	}
}

func TestNewGeminiDocumentExtractor_CustomModel(t *testing.T) {
	e, err := NewGeminiDocumentExtractor("test-key", "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.model != "gemini-2.5-flash" {
		t.Errorf("expected gemini-2.5-flash, got %q", e.model)
	}
}
