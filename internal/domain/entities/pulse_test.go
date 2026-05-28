package entities

import (
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/helpers"
)

func TestMemoryKey_String(t *testing.T) {
	k := MemoryKey{
		Channel: "whatsapp",
		User:    "+5511999",
	}
	got := k.String()
	want := "whatsapp:+5511999"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestMemoryKey_Validate_Valid(t *testing.T) {
	k := MemoryKey{
		Channel: "whatsapp",
		User:    "+5511999",
	}
	err := k.Validate()
	if err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestMemoryKey_Validate_EmptyChannel(t *testing.T) {
	k := MemoryKey{
		Channel: "",
		User:    "+5511999",
	}
	err := k.Validate()
	if err == nil {
		t.Errorf("Validate() = nil, want error")
	}
}

func TestMemoryKey_Validate_EmptyUser(t *testing.T) {
	k := MemoryKey{
		Channel: "whatsapp",
		User:    "",
	}
	err := k.Validate()
	if err == nil {
		t.Errorf("Validate() = nil, want error")
	}
}

func TestSubjectKey_String(t *testing.T) {
	k := SubjectKey{
		Entity: "shipment",
		ID:     "123",
	}
	got := k.String()
	want := "shipment:123"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestSubjectKey_Validate_Valid(t *testing.T) {
	k := SubjectKey{
		Entity: "shipment",
		ID:     "123",
	}
	err := k.Validate()
	if err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestSubjectKey_Validate_EmptyEntity(t *testing.T) {
	k := SubjectKey{
		Entity: "",
		ID:     "123",
	}
	err := k.Validate()
	if err == nil {
		t.Errorf("Validate() = nil, want error")
	}
}

func TestSubjectKey_Validate_EmptyID(t *testing.T) {
	k := SubjectKey{
		Entity: "shipment",
		ID:     "",
	}
	err := k.Validate()
	if err == nil {
		t.Errorf("Validate() = nil, want error")
	}
}

func TestNewPulse_InvalidMemoryKey(t *testing.T) {
	k := MemoryKey{
		Channel: "",
		User:    "+5511999",
	}
	builder := NewPulse(k)
	pulse := builder.Build()
	if pulse != nil {
		t.Errorf("Build() = %v, want nil for invalid MemoryKey", pulse)
	}
}

func TestNewPulse_ValidMemoryKey_NonNil(t *testing.T) {
	k := MemoryKey{
		Channel: "whatsapp",
		User:    "+5511999",
	}
	builder := NewPulse(k)
	pulse := builder.Build()
	if pulse == nil {
		t.Errorf("Build() = nil, want non-nil for valid MemoryKey")
	}
}

func TestNewPulse_ValidMemoryKey_MemoryKeySet(t *testing.T) {
	k := MemoryKey{
		Channel: "whatsapp",
		User:    "+5511999",
	}
	builder := NewPulse(k)
	pulse := builder.Build()
	got := pulse.MemoryKey
	want := "whatsapp:+5511999"
	if got != want {
		t.Errorf("MemoryKey = %q, want %q", got, want)
	}
}

func TestNewPulse_ValidMemoryKey_IDNonEmpty(t *testing.T) {
	k := MemoryKey{
		Channel: "whatsapp",
		User:    "+5511999",
	}
	builder := NewPulse(k)
	pulse := builder.Build()
	if pulse.ID == "" {
		t.Errorf("ID = %q, want non-empty", pulse.ID)
	}
}

func TestNewPulse_ValidMemoryKey_TimestampNearNow(t *testing.T) {
	k := MemoryKey{
		Channel: "whatsapp",
		User:    "+5511999",
	}
	before := helpers.NowUTC()
	builder := NewPulse(k)
	pulse := builder.Build()
	after := helpers.NowUTC()

	if pulse.Timestamp.Before(before) || pulse.Timestamp.After(after) {
		t.Errorf("Timestamp = %v, want between %v and %v", pulse.Timestamp, before, after)
	}
}

func TestPulseBuilder_WithSubjectKey_InvalidKey_BuildReturnsNil(t *testing.T) {
	k := MemoryKey{
		Channel: "whatsapp",
		User:    "+5511999",
	}
	subject := SubjectKey{
		Entity: "shipment",
		ID:     "",
	}
	pulse := NewPulse(k).WithSubjectKey(subject).Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil when SubjectKey is invalid")
	}
}

func TestPulseBuilder_WithSubjectKey_ValidKey(t *testing.T) {
	k := MemoryKey{
		Channel: "whatsapp",
		User:    "+5511999",
	}
	subject := SubjectKey{
		Entity: "order",
		ID:     "42",
	}
	builder := NewPulse(k).WithSubjectKey(subject)
	pulse := builder.Build()
	if pulse == nil {
		t.Errorf("Build() = nil, want non-nil for valid SubjectKey")
	}
	got := pulse.SubjectKey
	want := "order:42"
	if got != want {
		t.Errorf("SubjectKey = %q, want %q", got, want)
	}
}

func TestPulse_GetPayloadString_PresentString(t *testing.T) {
	pulse := &Pulse{
		Payload: map[string]any{
			"k": "value",
		},
	}
	got, ok := pulse.GetPayloadString("k")
	if !ok {
		t.Errorf("GetPayloadString(\"k\") ok = false, want true")
	}
	if got != "value" {
		t.Errorf("GetPayloadString(\"k\") = %q, want %q", got, "value")
	}
}

func TestPulse_GetPayloadString_Missing(t *testing.T) {
	pulse := &Pulse{
		Payload: map[string]any{
			"k": "value",
		},
	}
	_, ok := pulse.GetPayloadString("missing")
	if ok {
		t.Errorf("GetPayloadString(\"missing\") ok = true, want false")
	}
}

func TestPulse_GetPayloadString_WrongType(t *testing.T) {
	pulse := &Pulse{
		Payload: map[string]any{
			"n": 42.0,
		},
	}
	_, ok := pulse.GetPayloadString("n")
	if ok {
		t.Errorf("GetPayloadString(\"n\") ok = true for float64, want false")
	}
}

func TestPulse_GetKnowledgeString_Present(t *testing.T) {
	pulse := &Pulse{
		Knowledge: map[string]any{
			"tier": "gold",
		},
	}
	got, ok := pulse.GetKnowledgeString("tier")
	if !ok {
		t.Errorf("GetKnowledgeString(\"tier\") ok = false, want true")
	}
	if got != "gold" {
		t.Errorf("GetKnowledgeString(\"tier\") = %q, want %q", got, "gold")
	}
}

func TestPulse_GetKnowledgeString_Missing(t *testing.T) {
	p := &Pulse{Knowledge: map[string]any{}}
	_, ok := p.GetKnowledgeString("missing")
	if ok {
		t.Error("want ok=false for missing key")
	}
}

func TestPulse_GetKnowledgeString_WrongType(t *testing.T) {
	p := &Pulse{Knowledge: map[string]any{"n": 42}}
	_, ok := p.GetKnowledgeString("n")
	if ok {
		t.Error("want ok=false for non-string value")
	}
}

func TestPulse_GetMetadataInt_FromInt(t *testing.T) {
	pulse := &Pulse{
		Metadata: map[string]any{
			"count": 10,
		},
	}
	got, ok := pulse.GetMetadataInt("count")
	if !ok {
		t.Errorf("GetMetadataInt(\"count\") ok = false, want true")
	}
	if got != 10 {
		t.Errorf("GetMetadataInt(\"count\") = %d, want %d", got, 10)
	}
}

func TestPulse_GetMetadataInt_FromFloat64(t *testing.T) {
	pulse := &Pulse{
		Metadata: map[string]any{
			"count": 10.5,
		},
	}
	got, ok := pulse.GetMetadataInt("count")
	if !ok {
		t.Errorf("GetMetadataInt(\"count\") ok = false, want true")
	}
	if got != 10 {
		t.Errorf("GetMetadataInt(\"count\") = %d, want %d", got, 10)
	}
}

func TestPulse_GetMetadataInt64_FromInt64(t *testing.T) {
	pulse := &Pulse{
		Metadata: map[string]any{
			"big_num": int64(9223372036854775807),
		},
	}
	got, ok := pulse.GetMetadataInt64("big_num")
	if !ok {
		t.Errorf("GetMetadataInt64(\"big_num\") ok = false, want true")
	}
	if got != 9223372036854775807 {
		t.Errorf("GetMetadataInt64(\"big_num\") = %d, want %d", got, 9223372036854775807)
	}
}

func TestPulse_GetMetadataFloat64_Present(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"rate": 3.14}}
	v, ok := p.GetMetadataFloat64("rate")
	if !ok || v != 3.14 {
		t.Errorf("want 3.14 ok=true, got %v %v", v, ok)
	}
}

func TestPulse_GetMetadataFloat64_Missing(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{}}
	_, ok := p.GetMetadataFloat64("missing")
	if ok {
		t.Error("want ok=false for missing key")
	}
}

func TestPulse_GetMetadataBool_Present(t *testing.T) {
	pulse := &Pulse{
		Metadata: map[string]any{
			"active": true,
		},
	}
	got, ok := pulse.GetMetadataBool("active")
	if !ok {
		t.Errorf("GetMetadataBool(\"active\") ok = false, want true")
	}
	if got != true {
		t.Errorf("GetMetadataBool(\"active\") = %v, want true", got)
	}
}

func TestPulse_GetMetadataStringSlice_FromStringSlice(t *testing.T) {
	pulse := &Pulse{
		Metadata: map[string]any{
			"tags": []string{"a", "b", "c"},
		},
	}
	got, ok := pulse.GetMetadataStringSlice("tags")
	if !ok {
		t.Errorf("GetMetadataStringSlice(\"tags\") ok = false, want true")
	}
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("GetMetadataStringSlice(\"tags\") len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("GetMetadataStringSlice(\"tags\")[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPulse_GetMetadataStringSlice_FromInterfaceSlice(t *testing.T) {
	pulse := &Pulse{
		Metadata: map[string]any{
			"tags": []any{"x", "y", "z"},
		},
	}
	got, ok := pulse.GetMetadataStringSlice("tags")
	if !ok {
		t.Errorf("GetMetadataStringSlice(\"tags\") ok = false, want true")
	}
	want := []string{"x", "y", "z"}
	if len(got) != len(want) {
		t.Fatalf("GetMetadataStringSlice(\"tags\") len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("GetMetadataStringSlice(\"tags\")[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPulse_SetMemoryKey_ChangesKey(t *testing.T) {
	pulse := &Pulse{
		MemoryKey: "old:key",
		Metadata:  make(map[string]any),
	}
	newKey := MemoryKey{
		Channel: "telegram",
		User:    "999",
	}
	err := pulse.SetMemoryKey(newKey, "")
	if err != nil {
		t.Errorf("SetMemoryKey() = %v, want nil", err)
	}
	if pulse.MemoryKey != "telegram:999" {
		t.Errorf("MemoryKey = %q, want %q", pulse.MemoryKey, "telegram:999")
	}
}

func TestPulse_SetMemoryKey_RecordsPreviousSession(t *testing.T) {
	pulse := &Pulse{
		MemoryKey: "old:key",
		Metadata:  make(map[string]any),
	}
	newKey := MemoryKey{
		Channel: "telegram",
		User:    "999",
	}
	pulse.SetMemoryKey(newKey, "")
	prev, ok := pulse.GetMetadataString("previous_session")
	if !ok {
		t.Errorf("previous_session metadata not found")
	}
	if prev != "old:key" {
		t.Errorf("previous_session = %q, want %q", prev, "old:key")
	}
}

func TestPulse_SetMemoryKey_SameKey_NoPreviousSession(t *testing.T) {
	pulse := &Pulse{
		MemoryKey: "whatsapp:123",
		Metadata:  make(map[string]any),
	}
	newKey := MemoryKey{
		Channel: "whatsapp",
		User:    "123",
	}
	pulse.SetMemoryKey(newKey, "")
	_, ok := pulse.GetMetadataString("previous_session")
	if ok {
		t.Errorf("previous_session should not be set for same key")
	}
}

func TestPulse_SetMemoryKey_InvalidKey(t *testing.T) {
	pulse := &Pulse{
		MemoryKey: "old:key",
		Metadata:  make(map[string]any),
	}
	newKey := MemoryKey{
		Channel: "",
		User:    "999",
	}
	err := pulse.SetMemoryKey(newKey, "")
	if err == nil {
		t.Errorf("SetMemoryKey() = nil, want error for invalid key")
	}
}

func TestPulse_SetTopic_ChangesSubjectKey(t *testing.T) {
	pulse := &Pulse{
		SubjectKey: "old:topic",
		Metadata:   make(map[string]any),
	}
	newTopic := SubjectKey{
		Entity: "order",
		ID:     "100",
	}
	err := pulse.SetTopic(newTopic, "")
	if err != nil {
		t.Errorf("SetTopic() = %v, want nil", err)
	}
	if pulse.SubjectKey != "order:100" {
		t.Errorf("SubjectKey = %q, want %q", pulse.SubjectKey, "order:100")
	}
}

func TestPulse_SetTopic_RecordsPreviousTopic(t *testing.T) {
	pulse := &Pulse{
		SubjectKey: "shipment:500",
		Metadata:   make(map[string]any),
	}
	newTopic := SubjectKey{
		Entity: "order",
		ID:     "100",
	}
	pulse.SetTopic(newTopic, "")
	prev, ok := pulse.GetMetadataString("previous_topic")
	if !ok {
		t.Errorf("previous_topic metadata not found")
	}
	if prev != "shipment:500" {
		t.Errorf("previous_topic = %q, want %q", prev, "shipment:500")
	}
}

func TestPulse_SetTopic_EmptyPrevious_NoRecording(t *testing.T) {
	pulse := &Pulse{
		SubjectKey: "",
		Metadata:   make(map[string]any),
	}
	newTopic := SubjectKey{
		Entity: "order",
		ID:     "100",
	}
	pulse.SetTopic(newTopic, "")
	_, ok := pulse.GetMetadataString("previous_topic")
	if ok {
		t.Errorf("previous_topic should not be set when previous is empty")
	}
}

func TestPulse_SetTopic_InvalidKey(t *testing.T) {
	p := &Pulse{}
	if err := p.SetTopic(SubjectKey{Entity: "", ID: ""}, ""); err == nil {
		t.Error("expected error for invalid SubjectKey")
	}
}

func TestPulse_SetTopic_SameKey_NoPreviousRecording(t *testing.T) {
	p := &Pulse{SubjectKey: "order:1", Metadata: map[string]any{}}
	p.SetTopic(SubjectKey{Entity: "order", ID: "1"}, "")
	if _, ok := p.Metadata["previous_topic"]; ok {
		t.Error("same key must not record previous_topic")
	}
}

func TestPulse_MergeKnowledge_Preserves(t *testing.T) {
	pulse := &Pulse{
		Knowledge: map[string]any{
			"a": "1",
		},
	}
	pulse.MergeKnowledge(map[string]any{
		"b": "2",
	})
	if pulse.Knowledge["a"] != "1" {
		t.Errorf("key 'a' must be preserved, got %v", pulse.Knowledge["a"])
	}
	if pulse.Knowledge["b"] != "2" {
		t.Errorf("key 'b' must be added, got %v", pulse.Knowledge["b"])
	}
}

func TestPulse_SetContactPhone_ChangesPhone(t *testing.T) {
	pulse := &Pulse{
		ContactPhone: "+5511999",
		Metadata:     make(map[string]any),
	}
	pulse.SetContactPhone("+5522888", "")
	if pulse.ContactPhone != "+5522888" {
		t.Errorf("ContactPhone = %q, want %q", pulse.ContactPhone, "+5522888")
	}
}

func TestPulse_SetContactPhone_RecordsPreviousPhone(t *testing.T) {
	pulse := &Pulse{
		ContactPhone: "+5511999",
		Metadata:     make(map[string]any),
	}
	pulse.SetContactPhone("+5522888", "")
	prev, ok := pulse.GetMetadataString("previous_contact_phone")
	if !ok {
		t.Errorf("previous_contact_phone metadata not found")
	}
	if prev != "+5511999" {
		t.Errorf("previous_contact_phone = %q, want %q", prev, "+5511999")
	}
}

func TestPulse_SetContactPhone_SamePhone_NoChange(t *testing.T) {
	p := &Pulse{ContactPhone: "+111", Metadata: map[string]any{}}
	p.SetContactPhone("+111", "")
	if _, ok := p.Metadata["previous_contact_phone"]; ok {
		t.Error("same phone must not record previous_contact_phone")
	}
}

func validMemoryKey() MemoryKey {
	return MemoryKey{Channel: "whatsapp", User: "+5511999"}
}

func invalidMemoryKey() MemoryKey {
	return MemoryKey{Channel: "", User: "+5511999"}
}

func TestPulseBuilder_WithID_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).WithID("custom-id").Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.ID != "custom-id" {
		t.Errorf("ID = %q, want %q", pulse.ID, "custom-id")
	}
}

func TestPulseBuilder_WithID_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).WithID("custom-id").Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_WithUserMessage_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).WithUserMessage("hello").Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.UserMessage != "hello" {
		t.Errorf("UserMessage = %q, want %q", pulse.UserMessage, "hello")
	}
}

func TestPulseBuilder_WithUserMessage_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).WithUserMessage("hello").Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_WithContactPhone_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).WithContactPhone("+5511999").Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.ContactPhone != "+5511999" {
		t.Errorf("ContactPhone = %q, want %q", pulse.ContactPhone, "+5511999")
	}
}

func TestPulseBuilder_WithContactPhone_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).WithContactPhone("+5511999").Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_WithSource_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).WithSource("api").Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.Source != "api" {
		t.Errorf("Source = %q, want %q", pulse.Source, "api")
	}
}

func TestPulseBuilder_WithSource_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).WithSource("api").Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_WithSubType_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).WithSubType("text").Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.SubType != "text" {
		t.Errorf("SubType = %q, want %q", pulse.SubType, "text")
	}
}

func TestPulseBuilder_WithSubType_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).WithSubType("text").Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_WithEventType_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).WithEventType("message").Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.EventType != "message" {
		t.Errorf("EventType = %q, want %q", pulse.EventType, "message")
	}
}

func TestPulseBuilder_WithEventType_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).WithEventType("message").Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_WithIdempotencyKey_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).WithIdempotencyKey("idem-123").Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.IdempotencyKey != "idem-123" {
		t.Errorf("IdempotencyKey = %q, want %q", pulse.IdempotencyKey, "idem-123")
	}
}

func TestPulseBuilder_WithIdempotencyKey_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).WithIdempotencyKey("idem-123").Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_WithTimestamp_SetsField(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	pulse := NewPulse(validMemoryKey()).WithTimestamp(ts).Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if !pulse.Timestamp.Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", pulse.Timestamp, ts)
	}
}

func TestPulseBuilder_WithTimestamp_NilGuard(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	pulse := NewPulse(invalidMemoryKey()).WithTimestamp(ts).Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_WithPayload_SetsField(t *testing.T) {
	payload := map[string]any{"foo": "bar"}
	pulse := NewPulse(validMemoryKey()).WithPayload(payload).Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.Payload["foo"] != "bar" {
		t.Errorf("Payload[\"foo\"] = %v, want %q", pulse.Payload["foo"], "bar")
	}
}

func TestPulseBuilder_WithPayload_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).WithPayload(map[string]any{"foo": "bar"}).Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_WithMetadata_SetsField(t *testing.T) {
	meta := map[string]any{"source": "test"}
	pulse := NewPulse(validMemoryKey()).WithMetadata(meta).Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.Metadata["source"] != "test" {
		t.Errorf("Metadata[\"source\"] = %v, want %q", pulse.Metadata["source"], "test")
	}
}

func TestPulseBuilder_WithMetadata_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).WithMetadata(map[string]any{"source": "test"}).Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_WithKnowledge_SetsField(t *testing.T) {
	know := map[string]any{"tier": "gold"}
	pulse := NewPulse(validMemoryKey()).WithKnowledge(know).Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.Knowledge["tier"] != "gold" {
		t.Errorf("Knowledge[\"tier\"] = %v, want %q", pulse.Knowledge["tier"], "gold")
	}
}

func TestPulseBuilder_WithKnowledge_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).WithKnowledge(map[string]any{"tier": "gold"}).Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_MergeKnowledge_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).
		WithKnowledge(map[string]any{"existing": "yes"}).
		MergeKnowledge(map[string]any{"new": "also"}).
		Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.Knowledge["existing"] != "yes" {
		t.Errorf("Knowledge[\"existing\"] = %v, want %q", pulse.Knowledge["existing"], "yes")
	}
	if pulse.Knowledge["new"] != "also" {
		t.Errorf("Knowledge[\"new\"] = %v, want %q", pulse.Knowledge["new"], "also")
	}
}

func TestPulseBuilder_MergeKnowledge_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).MergeKnowledge(map[string]any{"new": "also"}).Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_WithAttachments_SetsField(t *testing.T) {
	att := &Artifact{MediaID: "att-1"}
	pulse := NewPulse(validMemoryKey()).WithAttachments([]*Artifact{att}).Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if len(pulse.Attachments) != 1 || pulse.Attachments[0].MediaID != "att-1" {
		t.Errorf("Attachments = %v, want one artifact with ID att-1", pulse.Attachments)
	}
}

func TestPulseBuilder_WithAttachments_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).WithAttachments([]*Artifact{{MediaID: "att-1"}}).Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_AddPayload_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).AddPayload("key", "val").Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.Payload["key"] != "val" {
		t.Errorf("Payload[\"key\"] = %v, want %q", pulse.Payload["key"], "val")
	}
}

func TestPulseBuilder_AddPayload_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).AddPayload("key", "val").Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_MergePayload_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).
		AddPayload("existing", "yes").
		MergePayload(map[string]any{"new": "also"}).
		Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.Payload["existing"] != "yes" {
		t.Errorf("Payload[\"existing\"] = %v, want %q", pulse.Payload["existing"], "yes")
	}
	if pulse.Payload["new"] != "also" {
		t.Errorf("Payload[\"new\"] = %v, want %q", pulse.Payload["new"], "also")
	}
}

func TestPulseBuilder_MergePayload_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).MergePayload(map[string]any{"new": "also"}).Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_AddMetadata_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).AddMetadata("mkey", "mval").Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.Metadata["mkey"] != "mval" {
		t.Errorf("Metadata[\"mkey\"] = %v, want %q", pulse.Metadata["mkey"], "mval")
	}
}

func TestPulseBuilder_AddMetadata_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).AddMetadata("mkey", "mval").Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_MergeMetadata_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).
		AddMetadata("existing", "yes").
		MergeMetadata(map[string]any{"new": "also"}).
		Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.Metadata["existing"] != "yes" {
		t.Errorf("Metadata[\"existing\"] = %v, want %q", pulse.Metadata["existing"], "yes")
	}
	if pulse.Metadata["new"] != "also" {
		t.Errorf("Metadata[\"new\"] = %v, want %q", pulse.Metadata["new"], "also")
	}
}

func TestPulseBuilder_MergeMetadata_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).MergeMetadata(map[string]any{"new": "also"}).Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_AddKnowledge_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).AddKnowledge("kkey", "kval").Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.Knowledge["kkey"] != "kval" {
		t.Errorf("Knowledge[\"kkey\"] = %v, want %q", pulse.Knowledge["kkey"], "kval")
	}
}

func TestPulseBuilder_AddKnowledge_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).AddKnowledge("kkey", "kval").Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_AddAttachment_SetsField(t *testing.T) {
	att := &Artifact{MediaID: "att-1"}
	pulse := NewPulse(validMemoryKey()).AddAttachment(att).Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if len(pulse.Attachments) != 1 || pulse.Attachments[0].MediaID != "att-1" {
		t.Errorf("Attachments = %v, want one artifact with ID att-1", pulse.Attachments)
	}
}

func TestPulseBuilder_AddAttachment_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).AddAttachment(&Artifact{MediaID: "att-1"}).Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulseBuilder_WithParent_SetsField(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).WithParent("parent-42", 2).Build()
	if pulse == nil {
		t.Fatal("Build() = nil, want non-nil")
	}
	if pulse.ParentPulseID != "parent-42" {
		t.Errorf("ParentPulseID = %q, want %q", pulse.ParentPulseID, "parent-42")
	}
	if pulse.OrchestrationDepth != 2 {
		t.Errorf("OrchestrationDepth = %d, want %d", pulse.OrchestrationDepth, 2)
	}
}

func TestPulseBuilder_WithParent_NilGuard(t *testing.T) {
	pulse := NewPulse(invalidMemoryKey()).WithParent("parent-42", 2).Build()
	if pulse != nil {
		t.Errorf("Build() = non-nil, want nil for invalid MemoryKey")
	}
}

func TestPulse_GetMetadata_Present(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"k": "v"}}
	val, ok := p.GetMetadata("k")
	if !ok {
		t.Errorf("GetMetadata(\"k\") ok = false, want true")
	}
	if val != "v" {
		t.Errorf("GetMetadata(\"k\") = %v, want %q", val, "v")
	}
}

func TestPulse_GetMetadata_NilMap(t *testing.T) {
	p := &Pulse{}
	_, ok := p.GetMetadata("k")
	if ok {
		t.Errorf("GetMetadata(\"k\") ok = true, want false for nil Metadata")
	}
}

func TestPulse_GetMetadataMap_Present(t *testing.T) {
	inner := map[string]any{"nested": "yes"}
	p := &Pulse{Metadata: map[string]any{"m": inner}}
	got, ok := p.GetMetadataMap("m")
	if !ok {
		t.Errorf("GetMetadataMap(\"m\") ok = false, want true")
	}
	if got["nested"] != "yes" {
		t.Errorf("GetMetadataMap(\"m\")[\"nested\"] = %v, want %q", got["nested"], "yes")
	}
}

func TestPulse_GetMetadataMap_WrongType(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"m": "not-a-map"}}
	_, ok := p.GetMetadataMap("m")
	if ok {
		t.Errorf("GetMetadataMap(\"m\") ok = true, want false for string value")
	}
}

func TestPulse_GetMetadataMap_NilMap(t *testing.T) {
	p := &Pulse{}
	_, ok := p.GetMetadataMap("m")
	if ok {
		t.Errorf("GetMetadataMap(\"m\") ok = true, want false for nil Metadata")
	}
}

func TestPulse_GetMetadataBool_True(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"flag": true}}
	got, ok := p.GetMetadataBool("flag")
	if !ok {
		t.Errorf("GetMetadataBool(\"flag\") ok = false, want true")
	}
	if !got {
		t.Errorf("GetMetadataBool(\"flag\") = false, want true")
	}
}

func TestPulse_GetMetadataBool_False(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"flag": false}}
	got, ok := p.GetMetadataBool("flag")
	if !ok {
		t.Errorf("GetMetadataBool(\"flag\") ok = false, want true")
	}
	if got {
		t.Errorf("GetMetadataBool(\"flag\") = true, want false")
	}
}

func TestPulse_GetMetadataBool_WrongType(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"flag": "yes"}}
	_, ok := p.GetMetadataBool("flag")
	if ok {
		t.Errorf("GetMetadataBool(\"flag\") ok = true, want false for string value")
	}
}

func TestPulse_MergePayload_MergesIntoExisting(t *testing.T) {
	p := &Pulse{Payload: map[string]any{"a": "1"}}
	p.MergePayload(map[string]any{"b": "2"})
	if p.Payload["a"] != "1" {
		t.Errorf("Payload[\"a\"] = %v, want %q", p.Payload["a"], "1")
	}
	if p.Payload["b"] != "2" {
		t.Errorf("Payload[\"b\"] = %v, want %q", p.Payload["b"], "2")
	}
}

// ── addErr nil-branch ────────────────────────────────────────────────────────

func TestPulseBuilder_addErr_Nil_NoOp(t *testing.T) {
	b := NewPulse(validMemoryKey())
	b.addErr(nil) // should not append anything
	pulse := b.Build()
	if pulse == nil {
		t.Error("Build() = nil after addErr(nil), want non-nil")
	}
}

// ── GetMetadataInt branches ──────────────────────────────────────────────────

func TestPulse_GetMetadataInt_Int64Value(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"n": int64(42)}}
	got, ok := p.GetMetadataInt("n")
	if !ok {
		t.Error("GetMetadataInt ok = false, want true")
	}
	if got != 42 {
		t.Errorf("GetMetadataInt = %d, want 42", got)
	}
}

func TestPulse_GetMetadataInt_NilMap(t *testing.T) {
	p := &Pulse{}
	_, ok := p.GetMetadataInt("n")
	if ok {
		t.Error("GetMetadataInt ok = true, want false for nil Metadata")
	}
}

func TestPulse_GetMetadataInt_MissingKey(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{}}
	_, ok := p.GetMetadataInt("n")
	if ok {
		t.Error("GetMetadataInt ok = true, want false for missing key")
	}
}

func TestPulse_GetMetadataInt_WrongType(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"n": "not-a-number"}}
	_, ok := p.GetMetadataInt("n")
	if ok {
		t.Error("GetMetadataInt ok = true, want false for string value")
	}
}

// ── GetMetadataInt64 branches ────────────────────────────────────────────────

func TestPulse_GetMetadataInt64_IntValue(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"n": int(7)}}
	got, ok := p.GetMetadataInt64("n")
	if !ok {
		t.Error("GetMetadataInt64 ok = false, want true")
	}
	if got != 7 {
		t.Errorf("GetMetadataInt64 = %d, want 7", got)
	}
}

func TestPulse_GetMetadataInt64_Float64Value(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"n": float64(3.9)}}
	got, ok := p.GetMetadataInt64("n")
	if !ok {
		t.Error("GetMetadataInt64 ok = false, want true")
	}
	if got != 3 {
		t.Errorf("GetMetadataInt64 = %d, want 3 (truncated)", got)
	}
}

func TestPulse_GetMetadataInt64_NilMap(t *testing.T) {
	p := &Pulse{}
	_, ok := p.GetMetadataInt64("n")
	if ok {
		t.Error("GetMetadataInt64 ok = true, want false for nil Metadata")
	}
}

func TestPulse_GetMetadataInt64_MissingKey(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{}}
	_, ok := p.GetMetadataInt64("n")
	if ok {
		t.Error("GetMetadataInt64 ok = true, want false for missing key")
	}
}

func TestPulse_GetMetadataInt64_WrongType(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"n": "bad"}}
	_, ok := p.GetMetadataInt64("n")
	if ok {
		t.Error("GetMetadataInt64 ok = true, want false for string value")
	}
}

// ── GetMetadataFloat64 ───────────────────────────────────────────────────────

// ── GetMetadataString ────────────────────────────────────────────────────────

func TestPulse_GetMetadataString_NilMap(t *testing.T) {
	p := &Pulse{}
	_, ok := p.GetMetadataString("k")
	if ok {
		t.Error("GetMetadataString ok = true, want false for nil Metadata")
	}
}

func TestPulse_GetMetadataString_WrongType(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"k": 42}}
	_, ok := p.GetMetadataString("k")
	if ok {
		t.Error("GetMetadataString ok = true, want false for int value")
	}
}

// ── GetPayloadString / GetKnowledgeString nil-map ────────────────────────────

func TestPulse_GetPayloadString_NilMap(t *testing.T) {
	p := &Pulse{}
	_, ok := p.GetPayloadString("k")
	if ok {
		t.Error("GetPayloadString ok = true, want false for nil Payload")
	}
}

func TestPulse_GetPayloadString_MissingKey(t *testing.T) {
	p := &Pulse{Payload: map[string]any{}}
	_, ok := p.GetPayloadString("k")
	if ok {
		t.Error("GetPayloadString ok = true, want false for missing key")
	}
}

func TestPulse_GetKnowledgeString_NilMap(t *testing.T) {
	p := &Pulse{}
	_, ok := p.GetKnowledgeString("k")
	if ok {
		t.Error("GetKnowledgeString ok = true, want false for nil Knowledge")
	}
}

// ── Pulse direct methods (all previously 0%) ────────────────────────────────

func TestPulse_AddKnowledge_InitNilMap(t *testing.T) {
	p := &Pulse{} // Knowledge is nil
	p.AddKnowledge("k", "v")
	if p.Knowledge == nil {
		t.Fatal("Knowledge still nil after AddKnowledge")
	}
	if p.Knowledge["k"] != "v" {
		t.Errorf("Knowledge[\"k\"] = %v, want %q", p.Knowledge["k"], "v")
	}
}

func TestPulse_AddKnowledge_ExistingMap(t *testing.T) {
	p := &Pulse{Knowledge: map[string]any{"a": "1"}}
	p.AddKnowledge("b", "2")
	if p.Knowledge["b"] != "2" {
		t.Errorf("Knowledge[\"b\"] = %v, want %q", p.Knowledge["b"], "2")
	}
	if p.Knowledge["a"] != "1" {
		t.Errorf("Knowledge[\"a\"] = %v, want %q", p.Knowledge["a"], "1")
	}
}

func TestPulse_SetKnowledge(t *testing.T) {
	p := &Pulse{Knowledge: map[string]any{"old": "data"}}
	p.SetKnowledge(map[string]any{"new": "data"})
	if _, ok := p.Knowledge["old"]; ok {
		t.Error("old key still present after SetKnowledge")
	}
	if p.Knowledge["new"] != "data" {
		t.Errorf("Knowledge[\"new\"] = %v, want %q", p.Knowledge["new"], "data")
	}
}

func TestPulse_AddPayload_InitNilMap(t *testing.T) {
	p := &Pulse{} // Payload is nil
	p.AddPayload("key", "val")
	if p.Payload == nil {
		t.Fatal("Payload still nil after AddPayload")
	}
	if p.Payload["key"] != "val" {
		t.Errorf("Payload[\"key\"] = %v, want %q", p.Payload["key"], "val")
	}
}

func TestPulse_AddPayload_ExistingMap(t *testing.T) {
	p := &Pulse{Payload: map[string]any{"x": "1"}}
	p.AddPayload("y", "2")
	if p.Payload["y"] != "2" {
		t.Errorf("Payload[\"y\"] = %v, want %q", p.Payload["y"], "2")
	}
}

func TestPulse_SetPayload(t *testing.T) {
	p := &Pulse{Payload: map[string]any{"old": "data"}}
	p.SetPayload(map[string]any{"new": "data"})
	if _, ok := p.Payload["old"]; ok {
		t.Error("old key still present after SetPayload")
	}
	if p.Payload["new"] != "data" {
		t.Errorf("Payload[\"new\"] = %v, want %q", p.Payload["new"], "data")
	}
}

func TestPulse_SetMetadata(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"old": "data"}}
	p.SetMetadata(map[string]any{"new": "meta"})
	if _, ok := p.Metadata["old"]; ok {
		t.Error("old key still present after SetMetadata")
	}
	if p.Metadata["new"] != "meta" {
		t.Errorf("Metadata[\"new\"] = %v, want %q", p.Metadata["new"], "meta")
	}
}

func TestPulse_AddAttachment_InitNilSlice(t *testing.T) {
	p := &Pulse{} // Attachments is nil
	att := &Artifact{MediaID: "m1"}
	p.AddAttachment(att)
	if len(p.Attachments) != 1 || p.Attachments[0].MediaID != "m1" {
		t.Errorf("Attachments = %v, want one item with MediaID m1", p.Attachments)
	}
}

func TestPulse_AddAttachment_ExistingSlice(t *testing.T) {
	p := &Pulse{Attachments: []*Artifact{{MediaID: "m1"}}}
	p.AddAttachment(&Artifact{MediaID: "m2"})
	if len(p.Attachments) != 2 {
		t.Errorf("len(Attachments) = %d, want 2", len(p.Attachments))
	}
}

func TestPulse_SetAttachments(t *testing.T) {
	p := &Pulse{Attachments: []*Artifact{{MediaID: "old"}}}
	p.SetAttachments([]*Artifact{{MediaID: "new1"}, {MediaID: "new2"}})
	if len(p.Attachments) != 2 || p.Attachments[0].MediaID != "new1" {
		t.Errorf("Attachments = %v, want new1/new2", p.Attachments)
	}
}

// ── MergeKnowledge / MergePayload / MergeMetadata nil-init branches ──────────

func TestPulse_MergeKnowledge_NilInit(t *testing.T) {
	p := &Pulse{} // Knowledge is nil
	p.MergeKnowledge(map[string]any{"k": "v"})
	if p.Knowledge["k"] != "v" {
		t.Errorf("MergeKnowledge on nil map: Knowledge[\"k\"] = %v, want %q", p.Knowledge["k"], "v")
	}
}

func TestPulse_MergePayload_NilInit(t *testing.T) {
	p := &Pulse{} // Payload is nil
	p.MergePayload(map[string]any{"k": "v"})
	if p.Payload["k"] != "v" {
		t.Errorf("MergePayload on nil map: Payload[\"k\"] = %v, want %q", p.Payload["k"], "v")
	}
}

func TestPulse_MergeMetadata_NilInit(t *testing.T) {
	p := &Pulse{} // Metadata is nil
	p.MergeMetadata(map[string]any{"k": "v"})
	if p.Metadata["k"] != "v" {
		t.Errorf("MergeMetadata on nil map: Metadata[\"k\"] = %v, want %q", p.Metadata["k"], "v")
	}
}

func TestPulse_MergeMetadata_MergesIntoExisting(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"a": "1"}}
	p.MergeMetadata(map[string]any{"b": "2"})
	if p.Metadata["a"] != "1" || p.Metadata["b"] != "2" {
		t.Errorf("MergeMetadata result = %v, want a=1 b=2", p.Metadata)
	}
}

// ── AddMetadata Pulse method nil-init branch ─────────────────────────────────

func TestPulse_AddMetadata_NilInit(t *testing.T) {
	p := &Pulse{} // Metadata is nil
	p.AddMetadata("key", "val")
	if p.Metadata == nil {
		t.Fatal("Metadata still nil after AddMetadata")
	}
	if p.Metadata["key"] != "val" {
		t.Errorf("Metadata[\"key\"] = %v, want %q", p.Metadata["key"], "val")
	}
}

// ── SetMemoryKey updatedBy branch ────────────────────────────────────────────

func TestPulse_SetMemoryKey_WithUpdatedBy(t *testing.T) {
	p := &Pulse{
		MemoryKey: "chan:old",
		Metadata:  map[string]any{},
	}
	newKey := MemoryKey{Channel: "chan", User: "new"}
	err := p.SetMemoryKey(newKey, "admin")
	if err != nil {
		t.Fatalf("SetMemoryKey error: %v", err)
	}
	if p.MemoryKey != "chan:new" {
		t.Errorf("MemoryKey = %q, want %q", p.MemoryKey, "chan:new")
	}
	if p.Metadata["session_updated_by"] != "admin" {
		t.Errorf("Metadata[\"session_updated_by\"] = %v, want %q", p.Metadata["session_updated_by"], "admin")
	}
	if p.Metadata["previous_session"] != "chan:old" {
		t.Errorf("Metadata[\"previous_session\"] = %v, want %q", p.Metadata["previous_session"], "chan:old")
	}
}

func TestPulse_SetMemoryKey_SameKey_NoOp(t *testing.T) {
	p := &Pulse{MemoryKey: "chan:user", Metadata: map[string]any{}}
	err := p.SetMemoryKey(MemoryKey{Channel: "chan", User: "user"}, "admin")
	if err != nil {
		t.Fatalf("SetMemoryKey error: %v", err)
	}
	// same key → no change, updatedBy not set
	if _, ok := p.Metadata["session_updated_by"]; ok {
		t.Error("session_updated_by should not be set when key unchanged")
	}
}

// ── SetTopic updatedBy branch ────────────────────────────────────────────────

func TestPulse_SetTopic_WithUpdatedBy(t *testing.T) {
	p := &Pulse{
		SubjectKey: "order:old",
		Metadata:   map[string]any{},
	}
	newKey := SubjectKey{Entity: "order", ID: "new"}
	err := p.SetTopic(newKey, "bot")
	if err != nil {
		t.Fatalf("SetTopic error: %v", err)
	}
	if p.SubjectKey != "order:new" {
		t.Errorf("SubjectKey = %q, want %q", p.SubjectKey, "order:new")
	}
	if p.Metadata["topic_updated_by"] != "bot" {
		t.Errorf("Metadata[\"topic_updated_by\"] = %v, want %q", p.Metadata["topic_updated_by"], "bot")
	}
	if p.Metadata["previous_topic"] != "order:old" {
		t.Errorf("Metadata[\"previous_topic\"] = %v, want %q", p.Metadata["previous_topic"], "order:old")
	}
}

func TestPulse_SetTopic_SameKey_NoOp(t *testing.T) {
	p := &Pulse{SubjectKey: "order:1", Metadata: map[string]any{}}
	err := p.SetTopic(SubjectKey{Entity: "order", ID: "1"}, "bot")
	if err != nil {
		t.Fatalf("SetTopic error: %v", err)
	}
	if _, ok := p.Metadata["topic_updated_by"]; ok {
		t.Error("topic_updated_by should not be set when key unchanged")
	}
}

// ── SetContactPhone branches ─────────────────────────────────────────────────

func TestPulse_SetContactPhone_WithUpdatedBy(t *testing.T) {
	p := &Pulse{
		ContactPhone: "+551199",
		Metadata:     map[string]any{},
	}
	p.SetContactPhone("+55119900", "admin")
	if p.ContactPhone != "+55119900" {
		t.Errorf("ContactPhone = %q, want %q", p.ContactPhone, "+55119900")
	}
	if p.Metadata["contact_phone_updated_by"] != "admin" {
		t.Errorf("Metadata[\"contact_phone_updated_by\"] = %v, want %q", p.Metadata["contact_phone_updated_by"], "admin")
	}
	if p.Metadata["previous_contact_phone"] != "+551199" {
		t.Errorf("Metadata[\"previous_contact_phone\"] = %v, want %q", p.Metadata["previous_contact_phone"], "+551199")
	}
}

func TestPulse_SetContactPhone_SamePhone_NoOp(t *testing.T) {
	p := &Pulse{ContactPhone: "+551199", Metadata: map[string]any{}}
	p.SetContactPhone("+551199", "admin")
	if _, ok := p.Metadata["contact_phone_updated_by"]; ok {
		t.Error("contact_phone_updated_by should not be set when phone unchanged")
	}
}

// ── Builder: AddPayload / AddMetadata / AddKnowledge nil-map init branches ───
// NewPulse initialises these maps; nil-init branch is hit via WithPayload(nil)/WithMetadata(nil)/WithKnowledge(nil).

func TestPulseBuilder_AddPayload_NilMapInit(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).WithPayload(nil).AddPayload("k", "v").Build()
	if pulse == nil {
		t.Fatal("Build() = nil")
	}
	if pulse.Payload["k"] != "v" {
		t.Errorf("Payload[\"k\"] = %v, want %q", pulse.Payload["k"], "v")
	}
}

func TestPulseBuilder_AddMetadata_NilMapInit(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).WithMetadata(nil).AddMetadata("k", "v").Build()
	if pulse == nil {
		t.Fatal("Build() = nil")
	}
	if pulse.Metadata["k"] != "v" {
		t.Errorf("Metadata[\"k\"] = %v, want %q", pulse.Metadata["k"], "v")
	}
}

func TestPulseBuilder_AddKnowledge_NilMapInit(t *testing.T) {
	pulse := NewPulse(validMemoryKey()).WithKnowledge(nil).AddKnowledge("k", "v").Build()
	if pulse == nil {
		t.Fatal("Build() = nil")
	}
	if pulse.Knowledge["k"] != "v" {
		t.Errorf("Knowledge[\"k\"] = %v, want %q", pulse.Knowledge["k"], "v")
	}
}

// ── Builder WithSubjectKey nil-event guard ───────────────────────────────────

func TestPulseBuilder_WithSubjectKey_NilEvent(t *testing.T) {
	// invalid MemoryKey → b.event == nil; WithSubjectKey should be a no-op
	pulse := NewPulse(invalidMemoryKey()).
		WithSubjectKey(SubjectKey{Entity: "order", ID: "1"}).
		Build()
	if pulse != nil {
		t.Error("Build() should be nil when MemoryKey was invalid")
	}
}

// ── GetMetadataFloat64 nil-map branch ───────────────────────────────────────

func TestPulse_GetMetadataFloat64_NilMap(t *testing.T) {
	p := &Pulse{}
	_, ok := p.GetMetadataFloat64("f")
	if ok {
		t.Error("GetMetadataFloat64 ok = true, want false for nil Metadata")
	}
}

func TestPulse_GetMetadataFloat64_WrongType(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"f": "not-float"}}
	_, ok := p.GetMetadataFloat64("f")
	if ok {
		t.Error("GetMetadataFloat64 ok = true, want false for string value")
	}
}

// ── GetMetadataBool nil-map + missing-key branches ───────────────────────────

func TestPulse_GetMetadataBool_NilMap(t *testing.T) {
	p := &Pulse{}
	_, ok := p.GetMetadataBool("flag")
	if ok {
		t.Error("GetMetadataBool ok = true, want false for nil Metadata")
	}
}

func TestPulse_GetMetadataBool_MissingKey(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{}}
	_, ok := p.GetMetadataBool("flag")
	if ok {
		t.Error("GetMetadataBool ok = true, want false for missing key")
	}
}

// ── GetMetadataStringSlice nil-map + missing-key + wrong-type branches ───────

func TestPulse_GetMetadataStringSlice_NilMap(t *testing.T) {
	p := &Pulse{}
	_, ok := p.GetMetadataStringSlice("tags")
	if ok {
		t.Error("GetMetadataStringSlice ok = true, want false for nil Metadata")
	}
}

func TestPulse_GetMetadataStringSlice_MissingKey(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{}}
	_, ok := p.GetMetadataStringSlice("tags")
	if ok {
		t.Error("GetMetadataStringSlice ok = true, want false for missing key")
	}
}

func TestPulse_GetMetadataStringSlice_WrongType(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{"tags": 42}}
	_, ok := p.GetMetadataStringSlice("tags")
	if ok {
		t.Error("GetMetadataStringSlice ok = true, want false for int value")
	}
}

// ── GetMetadataMap missing-key branch ────────────────────────────────────────

func TestPulse_GetMetadataMap_MissingKey(t *testing.T) {
	p := &Pulse{Metadata: map[string]any{}}
	_, ok := p.GetMetadataMap("m")
	if ok {
		t.Error("GetMetadataMap ok = true, want false for missing key")
	}
}
