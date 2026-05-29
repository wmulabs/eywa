package gemini

import (
	"context"
	"fmt"

	eywa "github.com/wmulabs/eywa"
	"google.golang.org/genai"
)

type GeminiImageAnalyzer struct {
	client *genai.Client
	model  string
}

func NewGeminiImageAnalyzer(ctx context.Context, apiKey string, model string) (*GeminiImageAnalyzer, error) {
	if model == "" {
		model = defaultModel
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini image analyzer: %w", err)
	}

	return &GeminiImageAnalyzer{client: client, model: model}, nil
}

func (a *GeminiImageAnalyzer) Analyze(ctx context.Context, imageData []byte, mimeType string) (string, eywa.OracleUsage, error) {
	if len(imageData) == 0 {
		return "", eywa.OracleUsage{}, fmt.Errorf("gemini image analyzer: empty image data")
	}

	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	content := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{InlineData: &genai.Blob{MIMEType: mimeType, Data: imageData}},
			{Text: "Describe the content of this image in detail. Include objects, visible text, colors, context, and any relevant information. Be objective and precise."},
		},
	}

	resp, err := a.client.Models.GenerateContent(ctx, a.model, []*genai.Content{content}, nil)
	if err != nil {
		return "", eywa.OracleUsage{}, fmt.Errorf("gemini image analyzer: %w", err)
	}

	text, usage, err := extractTextAndUsage(resp)
	return text, usage, err
}
