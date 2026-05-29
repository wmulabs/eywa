package helpers

import (
	"strings"
	"testing"
)

func TestObfuscatePhone(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		// long number (>8 chars) — keep first 4 + **** + last 4
		{"+5511987654321", "+551****4321"},
		{"+525512345678", "+525****5678"},
		{"123456789", "1234****6789"}, // 9 chars — just over boundary
		// boundary: exactly 8 chars → len <= keep*2 → all stars
		{"12345678", "********"},
		// short: < 8 chars → all stars
		{"123", "***"},
		{"1234567", strings.Repeat("*", 7)}, // 7 chars
		// empty
		{"", ""},
	}

	for _, c := range cases {
		got := ObfuscatePhone(c.input)
		if got != c.want {
			t.Errorf("ObfuscatePhone(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
