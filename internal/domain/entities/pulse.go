package entities

import (
	"fmt"
	"time"

	"github.com/wmulabs/eywa/internal/helpers"
)

type MemoryKey struct {
	Channel string
	User    string
}

func (k MemoryKey) String() string {
	return fmt.Sprintf("%s:%s", k.Channel, k.User)
}

func (k MemoryKey) Validate() error {
	if k.Channel == "" || k.User == "" {
		return fmt.Errorf("invalid session key")
	}
	return nil
}

type SubjectKey struct {
	Entity string
	ID     string
}

func (k SubjectKey) Validate() error {
	if k.Entity == "" || k.ID == "" {
		return fmt.Errorf("invalid topic key")
	}
	return nil
}

func (k SubjectKey) String() string {
	return fmt.Sprintf("%s:%s", k.Entity, k.ID)
}

// Pulse is the core unit of work flowing through the pipeline:
// InboundConverter → Scouts → Pathfinder → Weave (Spirit + LLM) → Actions (Voice delivery).
type Pulse struct {
	ID        string    `json:"id"`
	MemoryKey string    `json:"memory_key"`
	Timestamp time.Time `json:"timestamp"`

	UserMessage string `json:"user_message,omitempty"`
	// For inbound messages: sender's phone. For trigger Pulses (e.g. start_route): target phone, may be set by Scouts.
	ContactPhone string `json:"contact_phone,omitempty"`

	Source    string `json:"source,omitempty"`
	SubType   string `json:"sub_type,omitempty"`
	EventType string `json:"event_type,omitempty"`

	IdempotencyKey string `json:"idempotency_key,omitempty"` // e.g. wamid, MessageSid

	Attachments []*Artifact    `json:"attachments,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`

	// SubjectKey identifies the business entity being discussed (e.g. "shipment:123", "ticket:456").
	// Set by inbound converters when the payload carries enough context to determine the topic.
	// Weave propagates this into Memory.SubjectKey at setup time.
	// Scouts and the LLM update_topic Action can override it later in the pipeline.
	SubjectKey string `json:"subject_key,omitempty"`

	// Metadata is PRIVATE — used for audit/logging only, never sent to the LLM.
	// Examples: wamid, phone_number_id, raw_timestamp.
	Metadata map[string]any `json:"metadata,omitempty"`

	// Knowledge is PUBLIC — accumulated by Scouts, session, and receptors; forwarded to the LLM as context.
	// This appears in the "Knowledge" section of the system prompt.
	Knowledge map[string]any `json:"knowledge,omitempty"`

	// Orchestration fields — set when this Pulse is a sub-task delegated by an orchestrator Spirit.
	ParentPulseID      string `json:"parent_pulse_id,omitempty"`
	OrchestrationDepth int    `json:"orchestration_depth,omitempty"`
}

type ArtifactType string

const (
	ArtifactTypeImage    ArtifactType = "image"
	ArtifactTypeAudio    ArtifactType = "audio"
	ArtifactTypeVideo    ArtifactType = "video"
	ArtifactTypeDocument ArtifactType = "document"
)

type Artifact struct {
	Type     ArtifactType `json:"type"`
	URL      string       `json:"url,omitempty"`
	MimeType string       `json:"mime_type,omitempty"`
	FileName string       `json:"file_name,omitempty"`
	Caption  string       `json:"caption,omitempty"`
	MediaID  string       `json:"media_id,omitempty"`
	// Data holds raw bytes for direct upload; preferred over URL when available.
	Data []byte `json:"data,omitempty"`
}

func NewPulse(sessionKey MemoryKey) *PulseBuilder {
	if err := sessionKey.Validate(); err != nil {
		return &PulseBuilder{
			errs: []error{err},
		}
	}

	return &PulseBuilder{
		event: &Pulse{
			ID:        helpers.GenerateRandomID(),
			MemoryKey: sessionKey.String(),
			Timestamp: helpers.NowUTC(),
			Payload:   make(map[string]any),
			Metadata: map[string]any{
				"session": sessionKey,
			},
			Knowledge: make(map[string]any),
		},
	}
}

type PulseBuilder struct {
	event *Pulse
	errs  []error
}

func (b *PulseBuilder) addErr(err error) {
	if err == nil {
		return
	}
	b.errs = append(b.errs, err)
}

func (b *PulseBuilder) WithID(id string) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.ID = id
	return b
}

func (b *PulseBuilder) WithUserMessage(message string) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.UserMessage = message
	return b
}

func (b *PulseBuilder) WithContactPhone(phone string) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.ContactPhone = phone
	return b
}

func (b *PulseBuilder) WithSource(source string) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.Source = source
	return b
}

func (b *PulseBuilder) WithSubType(subType string) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.SubType = subType
	return b
}

func (b *PulseBuilder) WithEventType(eventType string) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.EventType = eventType
	return b
}

func (b *PulseBuilder) WithIdempotencyKey(key string) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.IdempotencyKey = key
	return b
}

func (b *PulseBuilder) WithSubjectKey(k SubjectKey) *PulseBuilder {
	if b.event == nil {
		return b
	}
	if err := k.Validate(); err != nil {
		b.addErr(err)
		return b
	}

	b.event.SubjectKey = k.String()
	b.event.AddMetadata("topic", k.String())

	return b
}

func (b *PulseBuilder) WithTimestamp(timestamp time.Time) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.Timestamp = timestamp
	return b
}

func (b *PulseBuilder) WithPayload(payload map[string]any) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.Payload = payload
	return b
}

func (b *PulseBuilder) WithMetadata(metadata map[string]any) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.Metadata = metadata
	return b
}

func (b *PulseBuilder) WithKnowledge(data map[string]any) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.Knowledge = data
	return b
}

func (b *PulseBuilder) MergeKnowledge(data map[string]any) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.MergeKnowledge(data)
	return b
}

func (b *PulseBuilder) WithAttachments(attachments []*Artifact) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.Attachments = attachments
	return b
}

func (b *PulseBuilder) AddPayload(key string, value any) *PulseBuilder {
	if b.event == nil {
		return b
	}
	if b.event.Payload == nil {
		b.event.Payload = make(map[string]any)
	}
	b.event.Payload[key] = value
	return b
}

func (b *PulseBuilder) MergePayload(payload map[string]any) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.MergePayload(payload)
	return b
}

func (b *PulseBuilder) AddMetadata(key string, value any) *PulseBuilder {
	if b.event == nil {
		return b
	}
	if b.event.Metadata == nil {
		b.event.Metadata = make(map[string]any)
	}
	b.event.Metadata[key] = value
	return b
}

func (b *PulseBuilder) MergeMetadata(metadata map[string]any) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.MergeMetadata(metadata)
	return b
}

func (b *PulseBuilder) AddKnowledge(key string, value any) *PulseBuilder {
	if b.event == nil {
		return b
	}
	if b.event.Knowledge == nil {
		b.event.Knowledge = make(map[string]any)
	}
	b.event.Knowledge[key] = value
	return b
}

func (b *PulseBuilder) AddAttachment(attachment *Artifact) *PulseBuilder {
	if b.event == nil {
		return b
	}
	if b.event.Attachments == nil {
		b.event.Attachments = []*Artifact{}
	}
	b.event.Attachments = append(b.event.Attachments, attachment)
	return b
}

func (b *PulseBuilder) WithParent(parentPulseID string, depth int) *PulseBuilder {
	if b.event == nil {
		return b
	}
	b.event.ParentPulseID = parentPulseID
	b.event.OrchestrationDepth = depth
	return b
}

// Build returns nil when NewPulse was called with an invalid MemoryKey.
// Callers should check for nil before using the returned Pulse.
func (b *PulseBuilder) Build() *Pulse {
	if len(b.errs) > 0 {
		return nil
	}
	return b.event
}

// GetPayloadString retrieves a string value from the Pulse Payload map by key.
// The Payload carries raw inbound data from the Receptor and is never sent to the LLM.
// Returns the value and true if the key exists and holds a string; otherwise returns "", false.
func (e *Pulse) GetPayloadString(key string) (string, bool) {
	if e.Payload == nil {
		return "", false
	}
	val, ok := e.Payload[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetKnowledgeString retrieves a string value from the public Knowledge map by key.
// Knowledge is accumulated by Scouts and sent to the LLM as context in the system prompt.
// Returns the value and true if the key exists and holds a string; otherwise returns "", false.
func (e *Pulse) GetKnowledgeString(key string) (string, bool) {
	if e.Knowledge == nil {
		return "", false
	}
	val, ok := e.Knowledge[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetMetadata retrieves a raw value from the private Metadata map by key.
// Metadata is for audit and logging only — it is never forwarded to the LLM.
// Returns the value and true if the key exists; otherwise returns nil, false.
func (e *Pulse) GetMetadata(key string) (any, bool) {
	if e.Metadata == nil {
		return nil, false
	}
	val, ok := e.Metadata[key]
	return val, ok
}

// GetMetadataString retrieves a string value from the Metadata map by key.
// Returns the value and true if the key exists and holds a string; otherwise returns "", false.
func (e *Pulse) GetMetadataString(key string) (string, bool) {
	if e.Metadata == nil {
		return "", false
	}
	val, ok := e.Metadata[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetMetadataInt retrieves an integer from the Metadata map by key.
// Handles int, int64, and float64 (the type encoding/json produces for JSON numbers).
// Returns the value and true on success; otherwise returns 0, false.
func (e *Pulse) GetMetadataInt(key string) (int, bool) {
	if e.Metadata == nil {
		return 0, false
	}
	val, ok := e.Metadata[key]
	if !ok {
		return 0, false
	}
	// Handle both int and float64 (JSON unmarshaling often produces float64)
	switch v := val.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// GetMetadataInt64 retrieves an int64 from the Metadata map by key.
// Handles int64, int, and float64 conversions.
// Returns the value and true on success; otherwise returns 0, false.
func (e *Pulse) GetMetadataInt64(key string) (int64, bool) {
	if e.Metadata == nil {
		return 0, false
	}
	val, ok := e.Metadata[key]
	if !ok {
		return 0, false
	}
	// Handle both int64 and float64 (JSON unmarshaling often produces float64)
	switch v := val.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}

// GetMetadataFloat64 retrieves a float64 from the Metadata map by key.
// Returns the value and true if the key exists and holds a float64; otherwise returns 0, false.
func (e *Pulse) GetMetadataFloat64(key string) (float64, bool) {
	if e.Metadata == nil {
		return 0, false
	}
	val, ok := e.Metadata[key]
	if !ok {
		return 0, false
	}
	f, ok := val.(float64)
	return f, ok
}

// GetMetadataBool retrieves a bool from the Metadata map by key.
// Returns the value and true if the key exists and holds a bool; otherwise returns false, false.
func (e *Pulse) GetMetadataBool(key string) (bool, bool) {
	if e.Metadata == nil {
		return false, false
	}
	val, ok := e.Metadata[key]
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// GetMetadataStringSlice retrieves a string slice from the Metadata map by key.
// Handles both []string and []any of strings (the latter is common after JSON round-trips).
// For []any values, non-string elements are silently skipped; the returned slice may be shorter than the original.
func (e *Pulse) GetMetadataStringSlice(key string) ([]string, bool) {
	if e.Metadata == nil {
		return nil, false
	}
	val, ok := e.Metadata[key]
	if !ok {
		return nil, false
	}
	// Try direct type assertion first
	if slice, ok := val.([]string); ok {
		return slice, true
	}
	// Handle []any (common from JSON unmarshaling)
	if interfaceSlice, ok := val.([]any); ok {
		stringSlice := make([]string, 0, len(interfaceSlice))
		for _, item := range interfaceSlice {
			if str, ok := item.(string); ok {
				stringSlice = append(stringSlice, str)
			}
		}
		return stringSlice, true
	}
	return nil, false
}

// GetMetadataMap retrieves a nested map from the Metadata map by key.
// Returns the map and true if the key exists and holds a map[string]any; otherwise returns nil, false.
func (e *Pulse) GetMetadataMap(key string) (map[string]any, bool) {
	if e.Metadata == nil {
		return nil, false
	}
	val, ok := e.Metadata[key]
	if !ok {
		return nil, false
	}
	m, ok := val.(map[string]any)
	return m, ok
}

// SetMemoryKey changes the MemoryKey, routing this Pulse to a different conversation session.
// The previous session key is preserved in Metadata["previous_session"] for audit purposes.
// updatedBy is optional; when non-empty it is recorded in Metadata["session_updated_by"].
// Returns an error if k fails validation (empty Channel or User).
func (e *Pulse) SetMemoryKey(k MemoryKey, updatedBy string) error {
	if err := k.Validate(); err != nil {
		return err
	}

	newMemoryKey := k.String()

	if newMemoryKey == e.MemoryKey {
		return nil
	}

	if e.MemoryKey != "" {
		e.AddMetadata("previous_session", e.MemoryKey)
	}

	if updatedBy != "" {
		e.AddMetadata("session_updated_by", updatedBy)
	}

	e.AddMetadata("session", newMemoryKey)

	e.MemoryKey = newMemoryKey
	return nil
}

// SetTopic changes the SubjectKey, redirecting this Pulse to a different business entity.
// The previous subject key is preserved in Metadata["previous_topic"] for audit purposes.
// updatedBy is optional; when non-empty it is recorded in Metadata["topic_updated_by"].
// Returns an error if k fails validation (empty Entity or ID).
func (e *Pulse) SetTopic(k SubjectKey, updatedBy string) error {
	if err := k.Validate(); err != nil {
		return err
	}

	newSubjectKey := k.String()

	if newSubjectKey == e.SubjectKey {
		return nil
	}

	if e.SubjectKey != "" {
		e.AddMetadata("previous_topic", e.SubjectKey)
	}

	if updatedBy != "" {
		e.AddMetadata("topic_updated_by", updatedBy)
	}

	e.AddMetadata("topic", newSubjectKey)

	e.SubjectKey = newSubjectKey
	return nil
}

// AddKnowledge adds or replaces a key-value pair in the public Knowledge map.
// Knowledge is sent to the LLM as context in the system prompt.
func (e *Pulse) AddKnowledge(key string, value any) {
	if e.Knowledge == nil {
		e.Knowledge = make(map[string]any)
	}
	e.Knowledge[key] = value
}

// SetKnowledge replaces the entire Knowledge map. Use MergeKnowledge to preserve existing values.
func (e *Pulse) SetKnowledge(data map[string]any) {
	e.Knowledge = data
}

// MergeKnowledge merges data into the Knowledge map, overwriting existing keys on conflict.
// All keys in data are added; keys not in data are preserved.
func (e *Pulse) MergeKnowledge(data map[string]any) {
	if e.Knowledge == nil {
		e.Knowledge = make(map[string]any)
	}
	for k, v := range data {
		e.Knowledge[k] = v
	}
}

// AddPayload adds or replaces a key-value pair in the Payload map.
// Payload carries raw inbound data from the Receptor and is never sent to the LLM.
func (e *Pulse) AddPayload(key string, value any) {
	if e.Payload == nil {
		e.Payload = make(map[string]any)
	}
	e.Payload[key] = value
}

// SetPayload replaces the entire Payload map. Use MergePayload to preserve existing values.
func (e *Pulse) SetPayload(payload map[string]any) {
	e.Payload = payload
}

// MergePayload merges data into the Payload map, overwriting existing keys on conflict.
// All keys in data are added; keys not in data are preserved.
func (e *Pulse) MergePayload(payload map[string]any) {
	if e.Payload == nil {
		e.Payload = make(map[string]any)
	}
	for k, v := range payload {
		e.Payload[k] = v
	}
}

// AddMetadata adds or replaces a key-value pair in the private Metadata map.
// Metadata is for audit and logging purposes only — it is never forwarded to the LLM.
func (e *Pulse) AddMetadata(key string, value any) {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
}

// SetMetadata replaces the entire Metadata map. Use MergeMetadata to preserve existing values.
func (e *Pulse) SetMetadata(metadata map[string]any) {
	e.Metadata = metadata
}

// MergeMetadata merges data into the Metadata map, overwriting existing keys on conflict.
// All keys in data are added; keys not in data are preserved.
func (e *Pulse) MergeMetadata(metadata map[string]any) {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	for k, v := range metadata {
		e.Metadata[k] = v
	}
}

// AddAttachment appends an Artifact (image, audio, video, or document) to the Pulse.
func (e *Pulse) AddAttachment(attachment *Artifact) {
	if e.Attachments == nil {
		e.Attachments = []*Artifact{}
	}
	e.Attachments = append(e.Attachments, attachment)
}

// SetAttachments replaces the Pulse attachment list.
// Use AddAttachment to append without discarding existing attachments.
func (e *Pulse) SetAttachments(attachments []*Artifact) {
	e.Attachments = attachments
}

// SetContactPhone updates the phone number used to route the response back to the caller.
// When the phone changes, the previous value is recorded in Metadata["previous_contact_phone"].
// updatedBy is optional; when non-empty it is recorded in Metadata["contact_phone_updated_by"].
func (e *Pulse) SetContactPhone(phone string, updatedBy string) {
	if phone == e.ContactPhone {
		return
	}
	if e.ContactPhone != "" {
		e.AddMetadata("previous_contact_phone", e.ContactPhone)
	}
	if updatedBy != "" {
		e.AddMetadata("contact_phone_updated_by", updatedBy)
	}
	e.ContactPhone = phone
}
