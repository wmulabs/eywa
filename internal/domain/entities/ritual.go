package entities

import "time"

// MetadataKeyRitualID is the Pulse.Metadata key carrying the internal scheduled task ID.
// Set by CloudTasksScheduler at dispatch time; consumed by RitualMarkStep and MarkExecutedStep.
const MetadataKeyRitualID = "ritual_id"

type RitualStatus string

const (
	RitualPending   RitualStatus = "pending"
	RitualRunning   RitualStatus = "running"
	RitualExecuted  RitualStatus = "executed"
	RitualCancelled RitualStatus = "cancelled"
	RitualFailed    RitualStatus = "failed"
)

type Ritual struct {
	ID            string                 `bson:"_id,omitempty"            json:"id"`
	MemoryKey     string                 `bson:"memory_key"               json:"memory_key"`
	EventKey      string                 `bson:"event_key"                json:"event_key"`
	KeeperTaskID  string                 `bson:"keeper_task_id"           json:"-"`
	ExecuteAt     time.Time              `bson:"execute_at"               json:"execute_at"`
	Status        RitualStatus           `bson:"status"                   json:"status"`
	CreatedBy     string                 `bson:"created_by"               json:"created_by"`
	EventSnapshot *Pulse                 `bson:"event_snapshot,omitempty" json:"event_snapshot,omitempty"`
	Recurrence    *RecurrenceConfig      `bson:"recurrence,omitempty"     json:"recurrence,omitempty"`
	AttemptCount  int                    `bson:"attempt_count"            json:"attempt_count"`
	CreatedAt     time.Time              `bson:"created_at"               json:"created_at"`
	StartedAt     *time.Time             `bson:"started_at,omitempty"     json:"started_at,omitempty"`
	ExecutedAt    *time.Time             `bson:"executed_at,omitempty"    json:"executed_at,omitempty"`
	CancelledAt   *time.Time             `bson:"cancelled_at,omitempty"   json:"cancelled_at,omitempty"`
	FailedAt      *time.Time             `bson:"failed_at,omitempty"      json:"failed_at,omitempty"`
	FailureReason string                 `bson:"failure_reason,omitempty" json:"failure_reason,omitempty"`
	Metadata      map[string]any `bson:"metadata,omitempty"       json:"metadata,omitempty"`
}

// RecurrenceConfig defines the schedule for a repeating Ritual.
// When nil, the Ritual executes once at ExecuteAt.
type RecurrenceConfig struct {
	// Cron is a standard cron expression (e.g. "0 9 * * 1-5" for weekday mornings).
	Cron string `bson:"cron"              json:"cron"`
	// Timezone is the IANA timezone name for interpreting the cron expression (e.g. "America/Mexico_City").
	// Defaults to UTC when empty.
	Timezone string `bson:"timezone"          json:"timezone"`
	// EndsAt is the optional recurrence end time. When set, no new runs are scheduled after this time.
	EndsAt *time.Time `bson:"ends_at,omitempty" json:"ends_at,omitempty"`
	// MaxRuns is the maximum number of times this Ritual may execute. 0 means unlimited.
	MaxRuns int `bson:"max_runs,omitempty" json:"max_runs,omitempty"`
	// RunCount tracks how many times this Ritual has executed so far.
	RunCount int `bson:"run_count"         json:"run_count"`
}
