// Package vertexai is a convenience re-export of the eywa Gemini provider
// for Vertex AI (Google Cloud) usage via Application Default Credentials (ADC).
//
// This package adds no new functionality over providers/gemini. It exists to
// provide a distinct import path for Vertex AI users. The exported Oracle type
// is identical to gemini.GeminiOracle — users who need the full Gemini provider
// API (e.g. multi-model configurations) should import providers/gemini directly.
//
// Set up ADC before use:
//
//	gcloud auth application-default login
//	# or set GOOGLE_APPLICATION_CREDENTIALS=/path/to/sa-key.json
//
// Spirit.ModelConfig.Provider must be set to "vertexai".
package vertexai

import (
	"context"

	gemini "github.com/wmulabs/eywa/providers/gemini"
)

const ProviderName = "vertexai"

// Oracle is a Vertex AI-backed Oracle. Identical to gemini.GeminiOracle but
// authenticated via ADC (BackendVertexAI) instead of an API key.
type Oracle = gemini.GeminiOracle

// NewOracle creates an Oracle for Vertex AI using ADC.
// project is your GCP project ID; location is the region (e.g. "us-central1").
func NewOracle(ctx context.Context, project, location string) (*Oracle, error) {
	return gemini.NewVertexOracle(ctx, project, location)
}
