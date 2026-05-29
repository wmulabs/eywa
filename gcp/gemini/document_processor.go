package gemini

import (
	"context"
	"fmt"

	eywa "github.com/wmulabs/eywa"
	"google.golang.org/genai"
)

type GeminiDocumentExtractor struct {
	client *genai.Client
	model  string
}

func NewGeminiDocumentExtractor(apiKey string, model string) (*GeminiDocumentExtractor, error) {
	if model == "" {
		model = defaultModel
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini document extractor: %w", err)
	}

	return &GeminiDocumentExtractor{client: client, model: model}, nil
}

func (e *GeminiDocumentExtractor) Extract(ctx context.Context, data []byte, mimeType string) (string, eywa.OracleUsage, error) {
	if len(data) == 0 {
		return "", eywa.OracleUsage{}, fmt.Errorf("gemini document extractor: empty document data")
	}

	if mimeType == "" {
		mimeType = "application/pdf"
	}

	content := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{InlineData: &genai.Blob{MIMEType: mimeType, Data: data}},
			{Text: "Extract and return all readable text content from this document. Return only the extracted text, preserving structure where possible. No explanations or additional formatting."},
		},
	}

	resp, err := e.client.Models.GenerateContent(ctx, e.model, []*genai.Content{content}, nil)
	if err != nil {
		return "", eywa.OracleUsage{}, fmt.Errorf("gemini document extractor: %w", err)
	}

	text, usage, err := extractTextAndUsage(resp)
	return text, usage, err
}
