package entities

type HTTPTool struct {
	ID               string            `bson:"_id" json:"id"`
	Name             string            `bson:"name" json:"name"`
	Description      string            `bson:"description" json:"description"`
	Method           string            `bson:"method" json:"method"`
	URL              string            `bson:"url" json:"url"`
	Headers          map[string]string `bson:"headers" json:"headers"`
	BodyTemplate     string            `bson:"body_template" json:"body_template"`
	Parameters       []HTTPToolParam   `bson:"parameters" json:"parameters"`
	TimeoutMS        int               `bson:"timeout_ms" json:"timeout_ms"`
	MaxResponseBytes int               `bson:"max_response_bytes" json:"max_response_bytes"`
	IsCritical       bool              `bson:"is_critical" json:"is_critical"`
	SpiritIDs        []string          `bson:"spirit_ids" json:"spirit_ids"`
}

type HTTPToolParam struct {
	Name        string `bson:"name" json:"name"`
	Type        string `bson:"type" json:"type"`
	Description string `bson:"description" json:"description"`
	Required    bool   `bson:"required" json:"required"`
}
