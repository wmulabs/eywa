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

// SplitTextIntoChunks splits text into overlapping chunks for vector ingestion.
// chunkSize is the max rune count per chunk; overlap is the number of runes shared between adjacent chunks.
// When overlap >= chunkSize it is clamped to zero.
func SplitTextIntoChunks(text string, chunkSize, overlap int) []string {
	if len(text) <= chunkSize {
		return []string{text}
	}
	step := chunkSize - overlap
	if step <= 0 {
		step = chunkSize
	}
	var chunks []string
	for i := 0; i < len(text); i += step {
		end := i + chunkSize
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[i:end])
		if end == len(text) {
			break
		}
	}
	return chunks
}
