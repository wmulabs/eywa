package ports

import "context"

// HandoffStore persists which Spirit currently owns a conversation after a handoff, so subsequent
// Pulses for the same session route directly to that Spirit. sessionKey is the Pulse MemoryKey.
// An empty active Spirit (the zero value returned by GetActiveSpirit) means no handoff is in effect.
type HandoffStore interface {
	GetActiveSpirit(ctx context.Context, sessionKey string) (string, error)
	SetActiveSpirit(ctx context.Context, sessionKey, spiritName string) error
	ClearActiveSpirit(ctx context.Context, sessionKey string) error
}
