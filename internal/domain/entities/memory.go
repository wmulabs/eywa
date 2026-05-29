package entities

import "time"

// Thread is a single message in a conversation session held in Memory.
type Thread struct {
	Role      string    `json:"role" bson:"role"`
	Content   string    `json:"content" bson:"content"`
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`
	// IsUserFacing controls whether the message is persisted to MongoDB and replayed on Memory reconstruction.
	IsUserFacing bool           `json:"is_user_facing" bson:"is_user_facing"`
	ToolCallID   string         `json:"tool_call_id,omitempty" bson:"tool_call_id,omitempty"`
	ImageURLs    []string       `json:"image_urls,omitempty" bson:"image_urls,omitempty"`
	AudioURLs    []string       `json:"audio_urls,omitempty" bson:"audio_urls,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty" bson:"metadata,omitempty"`
}

// Memory is ephemeral working state in Redis, keyed by (MemoryKey, SubjectKey).
// The composite key creates isolated memory spaces per (user, topic) pair,
// reconstructed from MongoDB on cache miss.
type Memory struct {
	MemoryKey       string         `json:"memory_key"`
	SubjectKey      string         `json:"subject_key"`
	Threads         []Thread       `json:"threads"`
	Summary         string         `json:"summary"`
	TopicFacts      map[string]any `json:"topic_facts"`
	LastInteraction time.Time      `json:"last_interaction"`
}
