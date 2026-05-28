package entities

import "time"

// Echo is a single message in the conversation history, persisted to the Echo (message store).
// It maps directly to an LLM chat thread entry and carries the role, content, and optional
// media URLs for multimodal conversations.
type Echo struct {
	ID           string                 `bson:"_id,omitempty"          json:"id,omitempty"`
	MemoryKey    string                 `bson:"memory_key"             json:"memory_key"`
	SubjectKey   string                 `bson:"subject_key,omitempty"  json:"subject_key,omitempty"`
	Role         string                 `bson:"role"                   json:"role"`
	Content      string                 `bson:"content"                json:"content"`
	Timestamp    time.Time              `bson:"timestamp"              json:"timestamp"`
	IsUserFacing bool                   `bson:"is_user_facing"         json:"is_user_facing"`
	ToolCallID   string                 `bson:"tool_call_id,omitempty" json:"tool_call_id,omitempty"`
	ImageURLs    []string               `bson:"image_urls,omitempty"   json:"image_urls,omitempty"`
	AudioURLs    []string               `bson:"audio_urls,omitempty"   json:"audio_urls,omitempty"`
	Metadata     map[string]any `bson:"metadata,omitempty"     json:"metadata,omitempty"`
}

func (m *Echo) ToThread() Thread {
	return Thread{
		Role:         m.Role,
		Content:      m.Content,
		Timestamp:    m.Timestamp,
		IsUserFacing: m.IsUserFacing,
		ToolCallID:   m.ToolCallID,
		ImageURLs:    m.ImageURLs,
		AudioURLs:    m.AudioURLs,
		Metadata:     m.Metadata,
	}
}
