package eywa

import (
	"testing"

	"github.com/wmulabs/eywa/internal/helpers/otelgenai"
)

func TestSetTraceContentCapture(t *testing.T) {
	t.Cleanup(func() { SetTraceContentCapture(false) })

	SetTraceContentCapture(true)
	if !otelgenai.CaptureContentEnabled() {
		t.Error("expected content capture enabled after SetTraceContentCapture(true)")
	}

	SetTraceContentCapture(false)
	if otelgenai.CaptureContentEnabled() {
		t.Error("expected content capture disabled after SetTraceContentCapture(false)")
	}
}
