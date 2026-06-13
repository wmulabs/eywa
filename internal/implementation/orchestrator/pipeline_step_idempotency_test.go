package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type stubIdempotencyStore struct {
	seen     bool
	err      error
	gotKey   string
	gotTTL   time.Duration
	callsLen int
}

var _ ports.IdempotencyStore = (*stubIdempotencyStore)(nil)

func (s *stubIdempotencyStore) Seen(_ context.Context, key string, ttl time.Duration) (bool, error) {
	s.gotKey = key
	s.gotTTL = ttl
	s.callsLen++
	return s.seen, s.err
}

func idempotencyState(idempotencyKey string) *ProcessingState {
	return &ProcessingState{
		Event: &entities.Pulse{
			EventType:      "user_message",
			MemoryKey:      "whatsapp:+5511999999999",
			IdempotencyKey: idempotencyKey,
		},
		EventConfig: &entities.Link{},
	}
}

func TestIdempotencyStep_FirstOccurrence_PassesThrough(t *testing.T) {
	store := &stubIdempotencyStore{seen: false}
	step := NewIdempotencyStep(store, time.Hour, time.Second, testLogger(t))
	state := idempotencyState("wamid.ABC")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.gotKey != "wamid.ABC" {
		t.Errorf("expected store keyed on idempotency key, got %q", store.gotKey)
	}
	if store.gotTTL != time.Hour {
		t.Errorf("expected configured TTL passed to store, got %v", store.gotTTL)
	}
}

func TestIdempotencyStep_Duplicate_ReturnsErrDuplicateEvent(t *testing.T) {
	store := &stubIdempotencyStore{seen: true}
	step := NewIdempotencyStep(store, time.Hour, time.Second, testLogger(t))
	state := idempotencyState("wamid.ABC")

	err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatal("expected error for duplicate event")
	}
	if !IsDuplicateEvent(err) {
		t.Errorf("expected DUPLICATE_EVENT error, got %v", err)
	}
	if IsRetriable(err) {
		t.Error("duplicate event must be non-retriable")
	}
}

func TestIdempotencyStep_NoIdempotencyKey_SkipsStore(t *testing.T) {
	store := &stubIdempotencyStore{seen: true}
	step := NewIdempotencyStep(store, time.Hour, time.Second, testLogger(t))
	state := idempotencyState("")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.callsLen != 0 {
		t.Error("store must not be queried when IdempotencyKey is empty")
	}
}

func TestIdempotencyStep_StoreError_FailsOpen(t *testing.T) {
	store := &stubIdempotencyStore{err: errors.New("redis down")}
	step := NewIdempotencyStep(store, time.Hour, time.Second, testLogger(t))
	state := idempotencyState("wamid.ABC")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("expected fail-open (nil error) on store failure, got %v", err)
	}
}
