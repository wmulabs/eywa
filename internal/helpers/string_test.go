package helpers

import (
	"strings"
	"testing"
)

func TestCombineTextPartsMultiLine(t *testing.T) {
	if v := CombineTextPartsMultiLine(nil); v != "" {
		t.Errorf("nil: got %q want empty", v)
	}
	if v := CombineTextPartsMultiLine([]string{}); v != "" {
		t.Errorf("empty slice: got %q want empty", v)
	}
	if v := CombineTextPartsMultiLine([]string{"only"}); v != "only" {
		t.Errorf("single: got %q want only", v)
	}
	v := CombineTextPartsMultiLine([]string{"a", "b", "c"})
	if v != "a\nb\nc" {
		t.Errorf("multi: got %q want a\\nb\\nc", v)
	}
}

func TestGenerateRandomID(t *testing.T) {
	id := GenerateRandomID()
	if len(id) == 0 {
		t.Error("expected non-empty ID")
	}
	// format: YYYYMMDDHHMMSS-xxxxxxxxxx  (15 + 1 + 10 = 26 chars)
	if !strings.Contains(id, "-") {
		t.Errorf("expected dash separator in ID %q", id)
	}
	parts := strings.SplitN(id, "-", 2)
	if len(parts[0]) != 14 {
		t.Errorf("timestamp part should be 14 chars, got %d: %q", len(parts[0]), parts[0])
	}
	if len(parts[1]) != 10 {
		t.Errorf("random part should be 10 chars, got %d: %q", len(parts[1]), parts[1])
	}
}

func TestGenerateRandomID_Unique(t *testing.T) {
	ids := make(map[string]struct{})
	for i := 0; i < 50; i++ {
		id := GenerateRandomID()
		if _, dup := ids[id]; dup {
			t.Errorf("duplicate ID generated: %q", id)
		}
		ids[id] = struct{}{}
	}
}
