package entities

// ResponseFormat describes a JSON Schema the model's response must conform to. Set it on a Spirit to
// make its final answer a validated structured object.
type ResponseFormat struct {
	Name   string         `bson:"name,omitempty" json:"name,omitempty"`     // schema name (provider-facing label)
	Schema map[string]any `bson:"schema,omitempty" json:"schema,omitempty"` // JSON Schema (object)
	Strict bool           `bson:"strict,omitempty" json:"strict,omitempty"` // reject fields not in the schema
}
