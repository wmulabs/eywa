package chunking

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func TestRecursive_ShortText_SingleChunk(t *testing.T) {
	chunks := Recursive("short text", 100, 0)
	if len(chunks) != 1 || chunks[0] != "short text" {
		t.Errorf("expected single chunk, got %v", chunks)
	}
}

func TestRecursive_Empty(t *testing.T) {
	if got := Recursive("   ", 100, 0); got != nil {
		t.Errorf("expected nil for blank text, got %v", got)
	}
}

func TestRecursive_ZeroSize_SingleChunk(t *testing.T) {
	if got := Recursive("anything here", 0, 0); len(got) != 1 {
		t.Errorf("expected whole text when size<=0, got %v", got)
	}
}

func TestRecursive_AllChunksWithinSize(t *testing.T) {
	text := strings.Repeat("word ", 200) // 1000 runes
	size := 50
	chunks := Recursive(text, size, 10)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if runeLen(c) > size {
			t.Errorf("chunk %d exceeds size %d: %d runes", i, size, runeLen(c))
		}
	}
}

func TestRecursive_PrefersParagraphBoundary(t *testing.T) {
	text := "First paragraph here.\n\nSecond paragraph here.\n\nThird paragraph here."
	chunks := Recursive(text, 30, 0)
	if len(chunks) < 2 {
		t.Fatalf("expected a split on paragraph boundaries, got %v", chunks)
	}
	// no chunk should glue two paragraphs that individually fit
	for _, c := range chunks {
		if strings.Count(c, "paragraph") > 1 && runeLen(c) > 30 {
			t.Errorf("chunk merged paragraphs beyond size: %q", c)
		}
	}
}

func TestRecursive_UTF8Safe(t *testing.T) {
	// Portuguese accents: byte-slicing would corrupt multi-byte runes.
	text := strings.Repeat("ção coração não é fácil. ", 40)
	chunks := Recursive(text, 20, 5)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if !utf8.ValidString(c) {
			t.Errorf("chunk %d is not valid UTF-8: %q", i, c)
		}
		if runeLen(c) > 20 {
			t.Errorf("chunk %d exceeds size in runes: %d", i, runeLen(c))
		}
	}
}

func TestRecursive_Overlap(t *testing.T) {
	// Single long word forces hard split; with overlap, consecutive chunks share a tail/head.
	text := strings.Repeat("a", 100)
	noOverlap := Recursive(text, 10, 0)
	withOverlap := Recursive(text, 10, 4)
	if len(withOverlap) <= len(noOverlap) {
		t.Errorf("expected overlap to produce more chunks: no=%d over=%d", len(noOverlap), len(withOverlap))
	}
}

func TestRecursive_OverlapClampedWhenTooLarge(t *testing.T) {
	// overlap >= size must not loop forever or panic; should still chunk.
	chunks := Recursive(strings.Repeat("x", 50), 10, 20)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks with clamped overlap, got %d", len(chunks))
	}
	for _, c := range chunks {
		if runeLen(c) > 10 {
			t.Errorf("chunk exceeds size: %d", runeLen(c))
		}
	}
}

func TestRecursive_SkipsBlankPartsBetweenSeparators(t *testing.T) {
	// Consecutive separators yield empty parts that must be skipped, not emitted as chunks.
	chunks := Recursive("alpha beta\n\n\n\ngamma delta epsilon", 12, 0)
	for _, c := range chunks {
		if strings.TrimSpace(c) == "" {
			t.Errorf("blank chunk emitted: %v", chunks)
		}
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %v", chunks)
	}
}

func TestRecursive_NegativeOverlap(t *testing.T) {
	chunks := Recursive(strings.Repeat("c", 25), 10, -5)
	if len(chunks) != 3 {
		t.Errorf("expected negative overlap treated as 0 → 3 chunks, got %d: %v", len(chunks), chunks)
	}
}

func TestHardSplit_StepClamp(t *testing.T) {
	// overlap >= size would make step <= 0; hardSplit must clamp to size and still terminate.
	out := hardSplit("xxxxxxx", 3, 5)
	if len(out) == 0 {
		t.Fatal("expected non-empty output")
	}
	for _, c := range out {
		if runeLen(c) > 3 {
			t.Errorf("chunk exceeds size: %q", c)
		}
	}
}

func TestRecursive_HardSplitLongLine(t *testing.T) {
	// No natural separators → hard rune window.
	chunks := Recursive(strings.Repeat("b", 25), 10, 0)
	if len(chunks) != 3 {
		t.Errorf("expected 3 hard-split chunks (10+10+5), got %d: %v", len(chunks), chunks)
	}
}
