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

func TestSplitTextIntoChunks(t *testing.T) {
	// text shorter than chunkSize → single chunk
	chunks := SplitTextIntoChunks("short", 100, 0)
	if len(chunks) != 1 || chunks[0] != "short" {
		t.Errorf("short text: got %v", chunks)
	}

	// exact split, no overlap
	text := "abcdefghij" // 10 chars
	chunks = SplitTextIntoChunks(text, 5, 0)
	if len(chunks) != 2 || chunks[0] != "abcde" || chunks[1] != "fghij" {
		t.Errorf("no-overlap: got %v", chunks)
	}

	// overlap
	chunks = SplitTextIntoChunks("abcdefgh", 4, 2)
	// step=2: [0:4]="abcd", [2:6]="cdef", [4:8]="efgh"
	if len(chunks) != 3 {
		t.Errorf("overlap: expected 3 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "abcd" || chunks[1] != "cdef" || chunks[2] != "efgh" {
		t.Errorf("overlap content: got %v", chunks)
	}

	// overlap >= chunkSize → clamped to 0 (step = chunkSize)
	chunks = SplitTextIntoChunks("abcdefgh", 4, 4)
	if len(chunks) != 2 {
		t.Errorf("overlap>=chunkSize: expected 2, got %d: %v", len(chunks), chunks)
	}

	// tail smaller than chunkSize
	chunks = SplitTextIntoChunks("abcde", 3, 0)
	// [0:3]="abc", [3:5]="de"
	if len(chunks) != 2 || chunks[1] != "de" {
		t.Errorf("tail chunk: got %v", chunks)
	}
}
