// Package schemaval validates JSON documents against a JSON Schema. It is a pure utility (no ports,
// no infrastructure) so it can be imported from anywhere inside the module.
package schemaval

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validate checks that data conforms to schema (a JSON Schema object). It returns nil on success and
// a descriptive error otherwise. The schema and data are decoded with the validator's own reader so
// numbers and nested values are typed as the JSON Schema dialect (2020-12) expects.
func Validate(schema map[string]any, data []byte) error {
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("marshal schema: %w", err)
	}

	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	if err != nil {
		return fmt.Errorf("parse schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", schemaDoc); err != nil {
		return fmt.Errorf("add schema: %w", err)
	}

	sch, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse data: %w", err)
	}

	if err := sch.Validate(instance); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	return nil
}
