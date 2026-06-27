package helpers

import (
	"reflect"
	"strings"
	"testing"
)

func TestRedactPII(t *testing.T) {
	const mask = "[REDACTED]"
	cases := []struct {
		name      string
		text      string
		kinds     []PIIKind
		want      string
		wantFound []PIIKind
	}{
		{
			name:      "email",
			text:      "reach me at john.doe+tag@example.co.uk please",
			want:      "reach me at [REDACTED] please",
			wantFound: []PIIKind{PIIEmail},
		},
		{
			name:      "credit card spaced, valid luhn",
			text:      "card 4111 1111 1111 1111 expires soon",
			want:      "card [REDACTED] expires soon",
			wantFound: []PIIKind{PIICreditCard},
		},
		{
			name:      "credit card hyphenated, valid luhn",
			text:      "5500-0000-0000-0004",
			want:      "[REDACTED]",
			wantFound: []PIIKind{PIICreditCard},
		},
		{
			name:      "invalid luhn is not a credit card",
			text:      "order 4111 1111 1111 1112 shipped",
			want:      "order 4111 1111 1111 1112 shipped",
			wantFound: nil,
		},
		{
			name:      "phone international",
			text:      "call +55 11 98765-4321 now",
			want:      "call [REDACTED] now",
			wantFound: []PIIKind{PIIPhone},
		},
		{
			name:      "phone us formatted",
			text:      "tel (555) 123-4567",
			want:      "tel [REDACTED]",
			wantFound: []PIIKind{PIIPhone},
		},
		{
			name:      "bare digit run is not a phone",
			text:      "order number 12345678 confirmed",
			want:      "order number 12345678 confirmed",
			wantFound: nil,
		},
		{
			name:      "kind filter redacts only requested",
			text:      "a@b.com and +55 11 98765-4321",
			kinds:     []PIIKind{PIIEmail},
			want:      "[REDACTED] and +55 11 98765-4321",
			wantFound: []PIIKind{PIIEmail},
		},
		{
			name:      "multiple kinds, stable order, deduped",
			text:      "x@y.com, z@w.com, +55 11 98765-4321",
			want:      "[REDACTED], [REDACTED], [REDACTED]",
			wantFound: []PIIKind{PIIEmail, PIIPhone},
		},
		{
			name:      "no pii unchanged",
			text:      "just a normal sentence",
			want:      "just a normal sentence",
			wantFound: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, found := RedactPII(c.text, c.kinds, mask)
			if got != c.want {
				t.Errorf("redacted = %q, want %q", got, c.want)
			}
			if !reflect.DeepEqual(found, c.wantFound) {
				t.Errorf("found = %v, want %v", found, c.wantFound)
			}
		})
	}
}

func TestRedactPII_CustomMask(t *testing.T) {
	got, _ := RedactPII("mail a@b.com", []PIIKind{PIIEmail}, "***")
	if got != "mail ***" {
		t.Errorf("got %q, want %q", got, "mail ***")
	}
}

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
