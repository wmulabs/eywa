package entities

// ActionDefinition is the wire format sent to an Oracle when describing an Action.
type ActionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}
