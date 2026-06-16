package errors

import "errors"

// ErrStructuredOutput is returned when a model's response could not be coerced into the requested
// schema after the bounded repair attempt. Callers can errors.Is against it and inspect the raw
// content carried by StructuredOutputError.
var ErrStructuredOutput = errors.New("structured output did not match schema")

// StructuredOutputError wraps ErrStructuredOutput with the raw model content and the validation
// failure, so callers can recover or log the offending output.
type StructuredOutputError struct {
	Raw    string
	Reason string
}

func (e *StructuredOutputError) Error() string {
	return "structured output did not match schema: " + e.Reason
}

func (e *StructuredOutputError) Is(target error) bool {
	return target == ErrStructuredOutput
}
