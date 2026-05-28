package entities

import "time"

// RiteStatus represents the lifecycle state of an approval request.
type RiteStatus string

const (
	// RitePending is the initial state — the Rite is waiting for an Operator decision.
	RitePending RiteStatus = "pending"
	// RiteApproved means an Operator approved the request; the paused Pulse can resume.
	RiteApproved RiteStatus = "approved"
	// RiteRejected means an Operator rejected the request; the Pulse should be aborted.
	RiteRejected RiteStatus = "rejected"
	// RiteExpired means the TTL elapsed before an Operator responded; treated as rejection.
	RiteExpired RiteStatus = "expired"
)

// Rite is a pending human-approval record created when a Spirit calls the request_approval Action.
// The originating Pulse is paused until an Operator approves or rejects the Rite,
// or until ExpiresAt elapses (RiteExpired).
type Rite struct {
	ID          string         `bson:"_id"`
	MemoryKey   string         `bson:"memory_key"`
	SubjectKey  string         `bson:"subject_key,omitempty"`
	EventKey    string         `bson:"event_key"`
	Context     map[string]any `bson:"context"`
	Reason      string         `bson:"reason"`
	Status      RiteStatus     `bson:"status"`
	OperatorID  string         `bson:"operator_id,omitempty"`
	RequestedAt time.Time      `bson:"requested_at"`
	DecidedAt   *time.Time     `bson:"decided_at,omitempty"`
	ExpiresAt   *time.Time     `bson:"expires_at,omitempty"`
}
