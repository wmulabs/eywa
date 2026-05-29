package redis

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	eywa "github.com/wmulabs/eywa"
)

func newTestClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return client, mr
}

func TestMemoryRepository_SaveAndGet(t *testing.T) {
	client, _ := newTestClient(t)
	repo := NewMemoryRepository(client, "svc", "test", 3600, nil)

	mem := &eywa.Memory{MemoryKey: "user:1", SubjectKey: "billing"}
	if err := repo.SaveMemory(context.Background(), "user:1", mem); err != nil {
		t.Fatalf("SaveMemory error: %v", err)
	}

	got, err := repo.GetMemory(context.Background(), "user:1")
	if err != nil {
		t.Fatalf("GetMemory error: %v", err)
	}
	if got.MemoryKey != "user:1" {
		t.Errorf("expected MemoryKey=user:1, got %s", got.MemoryKey)
	}
	if got.SubjectKey != "billing" {
		t.Errorf("expected SubjectKey=billing, got %s", got.SubjectKey)
	}
}

func TestMemoryRepository_GetMiss_ReturnsError(t *testing.T) {
	client, _ := newTestClient(t)
	repo := NewMemoryRepository(client, "svc", "test", 3600, nil)

	_, err := repo.GetMemory(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error on cache miss")
	}
}

func TestMemoryRepository_PrefixKey_NamespacesCorrectly(t *testing.T) {
	client, mr := newTestClient(t)
	repo := NewMemoryRepository(client, "myapp", "prod", 3600, nil)

	mem := &eywa.Memory{MemoryKey: "u:42"}
	_ = repo.SaveMemory(context.Background(), "u:42", mem)

	keys := mr.Keys()
	found := false
	for _, k := range keys {
		if k == "myapp:prod:memory:u:42" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected namespaced key, got keys: %v", keys)
	}
}

func TestMemoryRepository_TTL_IsSet(t *testing.T) {
	client, mr := newTestClient(t)
	repo := NewMemoryRepository(client, "svc", "test", 60, nil)

	_ = repo.SaveMemory(context.Background(), "u:1", &eywa.Memory{MemoryKey: "u:1"})

	ttl := mr.TTL("svc:test:memory:u:1")
	if ttl <= 0 {
		t.Errorf("expected TTL > 0, got %v", ttl)
	}
}

func TestMemoryRepository_Delete_RemovesKey(t *testing.T) {
	client, mr := newTestClient(t)
	repo := NewMemoryRepository(client, "svc", "test", 3600, nil)

	_ = repo.SaveMemory(context.Background(), "u:del", &eywa.Memory{MemoryKey: "u:del"})
	if err := repo.DeleteMemory(context.Background(), "u:del"); err != nil {
		t.Fatalf("DeleteMemory error: %v", err)
	}

	if mr.Exists("svc:test:memory:u:del") {
		t.Error("expected key deleted")
	}
}

func TestMemoryRepository_Save_UpdatesLastInteraction(t *testing.T) {
	client, _ := newTestClient(t)
	repo := NewMemoryRepository(client, "svc", "test", 3600, nil)

	before := time.Now().UTC()
	mem := &eywa.Memory{MemoryKey: "u:ts"}
	_ = repo.SaveMemory(context.Background(), "u:ts", mem)

	got, _ := repo.GetMemory(context.Background(), "u:ts")
	if got.LastInteraction.Before(before) {
		t.Errorf("expected LastInteraction updated, got %v (before %v)", got.LastInteraction, before)
	}
}

func TestMemoryRepository_Get_InvalidJSON_ReturnsError(t *testing.T) {
	client, mr := newTestClient(t)
	repo := NewMemoryRepository(client, "svc", "test", 3600, nil)

	mr.Set("svc:test:memory:bad", "not-json")

	_, err := repo.GetMemory(context.Background(), "bad")
	if err == nil {
		t.Error("expected error on invalid JSON")
	}
}

func TestMemoryRepository_Save_Overwrites(t *testing.T) {
	client, _ := newTestClient(t)
	repo := NewMemoryRepository(client, "svc", "test", 3600, nil)

	_ = repo.SaveMemory(context.Background(), "u:1", &eywa.Memory{MemoryKey: "u:1", SubjectKey: "first"})
	_ = repo.SaveMemory(context.Background(), "u:1", &eywa.Memory{MemoryKey: "u:1", SubjectKey: "second"})

	got, _ := repo.GetMemory(context.Background(), "u:1")
	if got.SubjectKey != "second" {
		t.Errorf("expected SubjectKey=second after overwrite, got %s", got.SubjectKey)
	}
}

func TestMemoryRepository_Save_PreservesThreads(t *testing.T) {
	client, _ := newTestClient(t)
	repo := NewMemoryRepository(client, "svc", "test", 3600, nil)

	mem := &eywa.Memory{
		MemoryKey: "u:1",
		Threads: []eywa.Thread{
			{Role: "user", Content: "hello", IsUserFacing: true},
			{Role: "assistant", Content: "hi", IsUserFacing: true},
		},
	}
	_ = repo.SaveMemory(context.Background(), "u:1", mem)

	got, _ := repo.GetMemory(context.Background(), "u:1")
	if len(got.Threads) != 2 {
		t.Errorf("expected 2 threads, got %d", len(got.Threads))
	}
}

func TestMemoryRepository_RoundTrip_TopicFacts(t *testing.T) {
	client, _ := newTestClient(t)
	repo := NewMemoryRepository(client, "svc", "test", 3600, nil)

	mem := &eywa.Memory{
		MemoryKey:  "u:1",
		TopicFacts: map[string]any{"plan": "premium", "lang": "pt"},
	}
	_ = repo.SaveMemory(context.Background(), "u:1", mem)

	got, _ := repo.GetMemory(context.Background(), "u:1")

	// JSON round-trip: values come back as any
	raw, _ := json.Marshal(got.TopicFacts)
	var facts map[string]any
	_ = json.Unmarshal(raw, &facts)
	if facts["plan"] != "premium" {
		t.Errorf("expected plan=premium, got %v", facts["plan"])
	}
}

func TestMemoryRepository_DeleteMemory_RedisError_ReturnsError(t *testing.T) {
	client, mr := newTestClient(t)
	repo := NewMemoryRepository(client, "svc", "test", 3600, nil)

	// Set a key first, then break Redis
	_ = repo.SaveMemory(context.Background(), "u:del", &eywa.Memory{MemoryKey: "u:del"})
	mr.Close()

	err := repo.DeleteMemory(context.Background(), "u:del")
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
}

func TestMemoryRepository_GetMemory_RedisError_ReturnsError(t *testing.T) {
	client, mr := newTestClient(t)
	repo := NewMemoryRepository(client, "svc", "test", 3600, nil)

	// Set a key first, then break Redis
	_ = repo.SaveMemory(context.Background(), "u:err", &eywa.Memory{MemoryKey: "u:err"})
	mr.Close()

	_, err := repo.GetMemory(context.Background(), "u:err")
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
}

func TestMemoryRepository_SaveMemory_RedisError_ReturnsError(t *testing.T) {
	client, mr := newTestClient(t)
	repo := NewMemoryRepository(client, "svc", "test", 3600, nil)

	// Break Redis before saving
	mr.Close()

	err := repo.SaveMemory(context.Background(), "u:save-err", &eywa.Memory{MemoryKey: "u:save-err"})
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
}
