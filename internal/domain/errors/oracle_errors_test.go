package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestStructuredOutputError(t *testing.T) {
	err := &StructuredOutputError{Raw: `{"x":1}`, Reason: "missing field name"}

	if !errors.Is(err, ErrStructuredOutput) {
		t.Error("expected errors.Is(err, ErrStructuredOutput) to be true")
	}
	if got := err.Error(); got != "structured output did not match schema: missing field name" {
		t.Errorf("unexpected message: %q", got)
	}

	// Wrapped chains must still match the sentinel.
	wrapped := fmt.Errorf("reasoning: %w", err)
	if !errors.Is(wrapped, ErrStructuredOutput) {
		t.Error("expected wrapped error to match ErrStructuredOutput")
	}
}
