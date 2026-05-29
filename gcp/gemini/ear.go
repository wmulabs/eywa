package gemini

import (
	"context"
	"fmt"

	eywa "github.com/wmulabs/eywa"
	"google.golang.org/genai"
)

type GeminiAudioTranscriber struct {
	client *genai.Client
	model  string
}

func NewGeminiAudioTranscriber(ctx context.Context, apiKey string, model string) (*GeminiAudioTranscriber, error) {
	if model == "" {
		model = defaultModel
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini audio transcriber: %w", err)
	}

	return &GeminiAudioTranscriber{client: client, model: model}, nil
}

func (t *GeminiAudioTranscriber) Transcribe(ctx context.Context, audioData []byte, mimeType string) (string, eywa.OracleUsage, error) {
	if len(audioData) == 0 {
		return "", eywa.OracleUsage{}, fmt.Errorf("gemini audio transcriber: empty audio data")
	}

	if mimeType == "" {
		mimeType = "audio/ogg"
	}

	content := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{InlineData: &genai.Blob{MIMEType: mimeType, Data: audioData}},
			{Text: "Transcribe the following audio. Return only the transcribed text, no explanations or additional formatting."},
		},
	}

	resp, err := t.client.Models.GenerateContent(ctx, t.model, []*genai.Content{content}, nil)
	if err != nil {
		return "", eywa.OracleUsage{}, fmt.Errorf("gemini audio transcriber: %w", err)
	}

	text, usage, err := extractTextAndUsage(resp)
	return text, usage, err
}
