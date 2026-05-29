package ports

import "context"

// Ear converts audio into text. Used as a fallback when the Oracle does not support audio natively.
type Ear interface {
	Transcribe(ctx context.Context, audioData []byte, mimeType string) (string, OracleUsage, error)
}

// Eye converts images into descriptive text. Used as a fallback when the Oracle does not support images natively.
type Eye interface {
	Analyze(ctx context.Context, imageData []byte, mimeType string) (string, OracleUsage, error)
}

// Implementations may use Google Document AI, Azure Form Recognizer, AWS Textract, etc.
type DocumentProcessor interface {
	Process(ctx context.Context, documentData []byte, mimeType string) (string, error)
}
