package helpers

import (
	"regexp"
	"strings"
)

// ObfuscatePhone masks the middle digits of a phone number for safe logging (e.g. +552****9999).
func ObfuscatePhone(phone string) string {
	const keep = 4
	if len(phone) <= keep*2 {
		return strings.Repeat("*", len(phone))
	}
	return phone[:keep] + "****" + phone[len(phone)-keep:]
}

// PIIKind identifies a category of personally identifiable information.
type PIIKind string

const (
	PIIEmail      PIIKind = "email"
	PIICreditCard PIIKind = "credit_card"
	PIIPhone      PIIKind = "phone"
)

// allPIIKinds is the canonical reporting order for RedactPII's found result.
var allPIIKinds = []PIIKind{PIIEmail, PIICreditCard, PIIPhone}

var (
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	// cardPattern matches 13-19 digit candidates separated by single spaces or hyphens.
	// A Luhn check (isLuhnValid) gates the actual redaction to keep false positives down.
	cardPattern = regexp.MustCompile(`\b\d(?:[ -]?\d){12,18}\b`)
	// phonePattern is deliberately conservative: it requires an international "+" prefix or
	// explicit separators, so bare digit runs (order numbers, IDs) are not mistaken for phones.
	phonePattern = regexp.MustCompile(`\+\d[\d\s().\-]{6,}\d|\(\d{3}\)[\s.\-]?\d{3}[\s.\-]?\d{4}|\b\d{3}[\s.\-]\d{3}[\s.\-]\d{4}\b`)
)

// RedactPII replaces occurrences of the requested PII kinds in text with mask. When kinds is empty,
// every supported kind is redacted. It returns the redacted text and the kinds actually found, in the
// canonical order email, credit_card, phone (deduplicated). Numeric kinds are redacted most-specific
// first so a credit card is never mis-classified as a phone number.
func RedactPII(text string, kinds []PIIKind, mask string) (string, []PIIKind) {
	enabled := func(k PIIKind) bool {
		if len(kinds) == 0 {
			return true
		}
		for _, kk := range kinds {
			if kk == k {
				return true
			}
		}
		return false
	}

	found := map[PIIKind]bool{}

	if enabled(PIICreditCard) {
		text = cardPattern.ReplaceAllStringFunc(text, func(m string) string {
			if !isLuhnValid(m) {
				return m
			}
			found[PIICreditCard] = true
			return mask
		})
	}
	if enabled(PIIPhone) {
		text = phonePattern.ReplaceAllStringFunc(text, func(m string) string {
			found[PIIPhone] = true
			return mask
		})
	}
	if enabled(PIIEmail) {
		text = emailPattern.ReplaceAllStringFunc(text, func(m string) string {
			found[PIIEmail] = true
			return mask
		})
	}

	var result []PIIKind
	for _, k := range allPIIKinds {
		if found[k] {
			result = append(result, k)
		}
	}
	return text, result
}

func isLuhnValid(s string) bool {
	var sum, digits int
	alt := false
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c < '0' || c > '9' {
			continue
		}
		d := int(c - '0')
		if alt {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
		digits++
	}
	return digits >= 13 && digits <= 19 && sum%10 == 0
}
