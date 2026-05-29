package ports

type OracleConfig struct {
	Name            string // "openai" | "anthropic" | "gemini" | "ollama"
	Enabled         bool
	APIKey          string
	BaseURL         string // override for custom endpoints or local services
	OrgID           string // OpenAI only
	ProjectID       string // GCP only
	Location        string // GCP only
	DefaultModel    string
	AvailableModels []string
	Settings        map[string]any
}
