package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	eywa "github.com/wmulabs/eywa"
)

func newVigilClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr
}

func TestVigilRepository_Acquire_Success(t *testing.T) {
	client, _ := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	if err := repo.Acquire(context.Background(), "user:1", "operator42", time.Minute); err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
}

func TestVigilRepository_Acquire_AlreadyHeld_ReturnsErrSessionHeld(t *testing.T) {
	client, _ := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	_ = repo.Acquire(context.Background(), "user:1", "operator1", time.Minute)
	err := repo.Acquire(context.Background(), "user:1", "operator2", time.Minute)

	if err != eywa.ErrSessionHeld {
		t.Errorf("expected ErrSessionHeld, got %v", err)
	}
}

func TestVigilRepository_Get_AfterAcquire_ReturnsVigil(t *testing.T) {
	client, _ := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	_ = repo.Acquire(context.Background(), "user:1", "operator42", time.Minute)

	v, err := repo.Get(context.Background(), "user:1")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if v == nil {
		t.Fatal("expected vigil, got nil")
	}
	if v.OperatorID != "operator42" {
		t.Errorf("expected OperatorID=operator42, got %s", v.OperatorID)
	}
	if v.MemoryKey != "user:1" {
		t.Errorf("expected MemoryKey=user:1, got %s", v.MemoryKey)
	}
}

func TestVigilRepository_Get_NoVigil_ReturnsNil(t *testing.T) {
	client, _ := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	v, err := repo.Get(context.Background(), "user:nobody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != nil {
		t.Errorf("expected nil vigil, got %v", v)
	}
}

func TestVigilRepository_Release_RemovesKey(t *testing.T) {
	client, mr := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	_ = repo.Acquire(context.Background(), "user:1", "operator1", time.Minute)
	_ = repo.Release(context.Background(), "user:1")

	if mr.Exists("svc:test:vigil:user:1") {
		t.Error("expected vigil key removed after Release")
	}
}

func TestVigilRepository_Release_ThenAcquire_Succeeds(t *testing.T) {
	client, _ := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	_ = repo.Acquire(context.Background(), "user:1", "operator1", time.Minute)
	_ = repo.Release(context.Background(), "user:1")

	if err := repo.Acquire(context.Background(), "user:1", "operator2", time.Minute); err != nil {
		t.Errorf("expected re-acquire after release, got: %v", err)
	}
}

func TestVigilRepository_Refresh_UpdatesTTL(t *testing.T) {
	client, mr := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	_ = repo.Acquire(context.Background(), "user:1", "operator1", time.Minute)
	mr.SetTTL("svc:test:vigil:user:1", 5*time.Second)

	_ = repo.Refresh(context.Background(), "user:1", time.Hour)

	ttl := mr.TTL("svc:test:vigil:user:1")
	if ttl < time.Minute {
		t.Errorf("expected TTL refreshed to ~1h, got %v", ttl)
	}
}

func TestVigilRepository_ListAll_ReturnsActiveVigils(t *testing.T) {
	client, _ := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	_ = repo.Acquire(context.Background(), "user:1", "operator1", time.Minute)
	_ = repo.Acquire(context.Background(), "user:2", "operator2", time.Minute)

	vigils, err := repo.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll error: %v", err)
	}
	if len(vigils) != 2 {
		t.Errorf("expected 2 vigils, got %d", len(vigils))
	}
}

func TestVigilRepository_ListAll_Empty_ReturnsEmpty(t *testing.T) {
	client, _ := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	vigils, err := repo.ListAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vigils) != 0 {
		t.Errorf("expected empty, got %d vigils", len(vigils))
	}
}

func TestVigilRepository_KeyNamespace(t *testing.T) {
	client, mr := newVigilClient(t)
	repo := NewVigilRepository(client, "myapp", "prod")

	_ = repo.Acquire(context.Background(), "u:99", "operator1", time.Minute)

	if !mr.Exists("myapp:prod:vigil:u:99") {
		t.Errorf("expected namespaced key myapp:prod:vigil:u:99, got keys: %v", mr.Keys())
	}
}

func TestVigilRepository_Get_MalformedValue_ReturnsError(t *testing.T) {
	client, mr := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	mr.Set("svc:test:vigil:bad", "no-colon-separator")

	_, err := repo.Get(context.Background(), "bad")
	if err == nil {
		t.Error("expected error on malformed vigil value")
	}
}

func TestVigilRepository_Get_InvalidTimestamp_ReturnsError(t *testing.T) {
	client, mr := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	mr.Set("svc:test:vigil:badts", "operator:not-a-number")

	_, err := repo.Get(context.Background(), "badts")
	if err == nil {
		t.Error("expected error on invalid timestamp in vigil value")
	}
}

func TestVigilRepository_Get_RedisError_ReturnsError(t *testing.T) {
	client, mr := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	_ = repo.Acquire(context.Background(), "user:err", "op1", time.Minute)
	mr.Close()

	_, err := repo.Get(context.Background(), "user:err")
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
}

func TestVigilRepository_Acquire_RedisError_ReturnsError(t *testing.T) {
	client, mr := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	mr.Close()

	err := repo.Acquire(context.Background(), "user:fail", "op1", time.Minute)
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
}

func TestVigilRepository_Release_RedisError_ReturnsError(t *testing.T) {
	client, mr := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	_ = repo.Acquire(context.Background(), "user:rel", "op1", time.Minute)
	mr.Close()

	err := repo.Release(context.Background(), "user:rel")
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
}

func TestVigilRepository_Refresh_RedisError_ReturnsError(t *testing.T) {
	client, mr := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	_ = repo.Acquire(context.Background(), "user:ref", "op1", time.Minute)
	mr.Close()

	err := repo.Refresh(context.Background(), "user:ref", time.Hour)
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
}

func TestVigilRepository_ListAll_CorruptKeyIgnored(t *testing.T) {
	client, mr := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	_ = repo.Acquire(context.Background(), "user:good", "op1", time.Minute)
	mr.Set("svc:test:vigil:corrupt", "no-colon")

	vigils, err := repo.ListAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// corrupt key skipped, only the good one returned
	if len(vigils) != 1 {
		t.Errorf("expected 1 vigil (corrupt skipped), got %d", len(vigils))
	}
}

func TestVigilRepository_ListAll_RedisError_ReturnsError(t *testing.T) {
	client, mr := newVigilClient(t)
	repo := NewVigilRepository(client, "svc", "test")

	mr.Close()

	_, err := repo.ListAll(context.Background())
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
}
