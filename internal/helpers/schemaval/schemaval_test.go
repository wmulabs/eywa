package schemaval

import "testing"

func objectSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"age":  map[string]any{"type": "integer"},
		},
		"required":             []any{"name"},
		"additionalProperties": false,
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		schema  map[string]any
		data    string
		wantErr bool
	}{
		{"valid", objectSchema(), `{"name":"ann","age":30}`, false},
		{"valid_optional_omitted", objectSchema(), `{"name":"ann"}`, false},
		{"missing_required", objectSchema(), `{"age":30}`, true},
		{"wrong_type", objectSchema(), `{"name":"ann","age":"old"}`, true},
		{"additional_property", objectSchema(), `{"name":"ann","extra":1}`, true},
		{"data_not_json", objectSchema(), `not-json`, true},
		{"data_wrong_root", map[string]any{"type": "object"}, `[1,2,3]`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.schema, []byte(tc.data))
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidate_InvalidSchema(t *testing.T) {
	// "type" must be a string or array of strings; a number is not a valid schema.
	badSchema := map[string]any{"type": 123}
	if err := Validate(badSchema, []byte(`{}`)); err == nil {
		t.Error("expected an error compiling an invalid schema")
	}
}

func TestValidate_UnmarshalableSchema(t *testing.T) {
	// A channel cannot be marshalled to JSON — exercises the marshal-schema error path.
	badSchema := map[string]any{"x": make(chan int)}
	if err := Validate(badSchema, []byte(`{}`)); err == nil {
		t.Error("expected an error when the schema cannot be marshalled")
	}
}
