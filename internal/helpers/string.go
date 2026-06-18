package helpers

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// CombineTextPartsMultiLine joins text parts with newlines between them.
// Returns "" for an empty slice. Returns the single element unchanged when len(parts) == 1.
func CombineTextPartsMultiLine(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	var builder strings.Builder
	builder.WriteString(parts[0])
	for i := 1; i < len(parts); i++ {
		builder.WriteString("\n")
		builder.WriteString(parts[i])
	}
	return builder.String()
}

// GenerateRandomID returns a unique ID in the form "YYYYMMDDHHMMSS-<10-char-random>".
// Used to generate Pulse IDs and similar internal identifiers.
func GenerateRandomID() string {
	return fmt.Sprintf("%s-%s",
		NowUTC().Format("20060102150405"),
		randomString(10),
	)
}

func randomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
		}
		b[i] = charset[idx.Int64()]
	}
	return string(b)
}
