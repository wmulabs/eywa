package helpers

import "strings"

// ObfuscatePhone masks the middle digits of a phone number for safe logging (e.g. +552****9999).
func ObfuscatePhone(phone string) string {
	const keep = 4
	if len(phone) <= keep*2 {
		return strings.Repeat("*", len(phone))
	}
	return phone[:keep] + "****" + phone[len(phone)-keep:]
}
