// Package schemaval validates JSON documents against a JSON Schema. It is a pure utility (no ports,
// no infrastructure) so it can be imported from anywhere inside the module.
package schemaval

import (
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validate checks that data conforms to schema (a JSON Schema object, draft 2020-12). It returns nil
// on success and a descriptive error otherwise.
func Validate(schema map[string]any, data []byte) error {
	sch, err := compile(schema)
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}

	var instance any
	if err := json.Unmarshal(data, &instance); err != nil {
		return fmt.Errorf("parse data: %w", err)
	}

	if err := sch.Validate(instance); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	return nil
}

func compile(schema map[string]any) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	// AddResource only fails on a malformed resource URL or a duplicate registration; a fixed URL
	// with a single in-memory schema can trigger neither, so any schema defect surfaces at Compile.
	_ = compiler.AddResource("schema.json", schema) //nolint:errcheck // unreachable for in-memory schema (see comment)
	sch, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}
	return sch, nil
}
