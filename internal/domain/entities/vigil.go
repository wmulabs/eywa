package entities

import "time"

// Vigil holds the operator seat state for a conversation.
// Stored ephemerally in Redis — no MongoDB collection.
type Vigil struct {
	MemoryKey  string
	OperatorID string
	SeatSince  time.Time
	ExpiresAt  time.Time
}
