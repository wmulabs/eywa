package tracer

import (
	"testing"
)

func TestGetTracer_ReturnsNonNil(t *testing.T) {
	tr := GetTracer()
	if tr == nil {
		t.Fatal("expected non-nil tracer")
	}
}

func TestGetTracer_Idempotent(t *testing.T) {
	t1 := GetTracer()
	t2 := GetTracer()
	if t1 != t2 {
		t.Error("expected same tracer instance on repeated calls")
	}
}
