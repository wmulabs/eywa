package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	eywa "github.com/wmulabs/eywa"
)

var _ eywa.CheckpointStore = (*CheckpointStore)(nil)

func newCheckpointClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr
}

func TestCheckpointStore_SaveLoadRoundTrip(t *testing.T) {
	client, _ := newCheckpointClient(t)
	store := NewCheckpointStore(client)
	payload := []byte(`{"iterations_used":2}`)

	if err := store.Save(context.Background(), "turn-1", payload); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	got, found, err := store.Load(context.Background(), "turn-1")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !found {
		t.Fatal("expected the checkpoint to be found")
	}
	if string(got) != string(payload) {
		t.Errorf("Load = %q, want %q", got, payload)
	}
}

func TestCheckpointStore_Load_NotFound(t *testing.T) {
	client, _ := newCheckpointClient(t)
	store := NewCheckpointStore(client)

	got, found, err := store.Load(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if found || got != nil {
		t.Errorf("expected (nil, false) for an absent checkpoint, got (%q, %v)", got, found)
	}
}

func TestCheckpointStore_Save_Overwrites(t *testing.T) {
	client, _ := newCheckpointClient(t)
	store := NewCheckpointStore(client)

	_ = store.Save(context.Background(), "turn-1", []byte("v1"))
	_ = store.Save(context.Background(), "turn-1", []byte("v2"))

	got, _, _ := store.Load(context.Background(), "turn-1")
	if string(got) != "v2" {
		t.Errorf("Load = %q, want the latest value %q", got, "v2")
	}
}

func TestCheckpointStore_Delete(t *testing.T) {
	client, _ := newCheckpointClient(t)
	store := NewCheckpointStore(client)

	_ = store.Save(context.Background(), "turn-1", []byte("v1"))
	if err := store.Delete(context.Background(), "turn-1"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	_, found, _ := store.Load(context.Background(), "turn-1")
	if found {
		t.Error("expected the checkpoint to be gone after Delete")
	}
}

func TestCheckpointStore_Delete_AbsentIsNoError(t *testing.T) {
	client, _ := newCheckpointClient(t)
	store := NewCheckpointStore(client)

	if err := store.Delete(context.Background(), "missing"); err != nil {
		t.Errorf("deleting an absent checkpoint must not error, got %v", err)
	}
}

func TestCheckpointStore_TTL_Expires(t *testing.T) {
	client, mr := newCheckpointClient(t)
	store := NewCheckpointStoreWithTTL(client, time.Minute)

	_ = store.Save(context.Background(), "turn-1", []byte("v1"))
	mr.FastForward(2 * time.Minute)

	_, found, err := store.Load(context.Background(), "turn-1")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if found {
		t.Error("expected the checkpoint to expire after its TTL")
	}
}

func TestCheckpointStore_WithTTL_NonPositiveFallsBackToDefault(t *testing.T) {
	client, mr := newCheckpointClient(t)
	store := NewCheckpointStoreWithTTL(client, 0)

	_ = store.Save(context.Background(), "turn-1", []byte("v1"))
	if ttl := mr.TTL("eywa:checkpoint:turn-1"); ttl != defaultCheckpointTTL {
		t.Errorf("expected default TTL %v, got %v", defaultCheckpointTTL, ttl)
	}
}

func TestCheckpointStore_ConnectionErrors(t *testing.T) {
	client, mr := newCheckpointClient(t)
	store := NewCheckpointStore(client)
	mr.Close()

	if err := store.Save(context.Background(), "turn-1", []byte("v1")); err == nil {
		t.Error("expected Save error when Redis unreachable")
	}
	if _, _, err := store.Load(context.Background(), "turn-1"); err == nil {
		t.Error("expected Load error when Redis unreachable")
	}
	if err := store.Delete(context.Background(), "turn-1"); err == nil {
		t.Error("expected Delete error when Redis unreachable")
	}
}
