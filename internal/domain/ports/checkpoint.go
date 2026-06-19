package ports

import "context"

// CheckpointStore persists in-progress reasoning-turn state so a turn interrupted mid-flight
// (deploy, restart, crash, request timeout, scale-to-zero) can resume from its last completed
// iteration instead of restarting from zero.
//
// data is opaque: the orchestrator owns the snapshot schema and (de)serialization. An adapter only
// stores and returns the bytes verbatim, keyed by turnID.
//
// Contract:
//   - Load returns (nil, false, nil) when no checkpoint exists for turnID — not an error.
//   - Load returns a non-nil error only on infrastructure failure.
//   - Save overwrites any existing checkpoint for the same turnID (one record per turn).
//   - Delete is idempotent: deleting an absent turnID is not an error.
type CheckpointStore interface {
	Save(ctx context.Context, turnID string, data []byte) error
	Load(ctx context.Context, turnID string) (data []byte, found bool, err error)
	Delete(ctx context.Context, turnID string) error
}
