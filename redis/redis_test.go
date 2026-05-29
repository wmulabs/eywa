package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestNewRedisConnection_ConnectsAndReturnsRepo(t *testing.T) {
	mr := miniredis.RunT(t)
	repo, err := NewRedisConnection(context.Background(), "redis://"+mr.Addr(), "myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo == nil {
		t.Fatal("expected non-nil RedisRepository")
	}
	if repo.basePath != "myapp" {
		t.Errorf("expected basePath=myapp, got %s", repo.basePath)
	}
}

func TestRedisRepository_GetRedisClient_ReturnsSameClient(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := &RedisRepository{redisClient: client, basePath: "x"}
	if repo.GetRedisClient() != client {
		t.Error("expected GetRedisClient to return the same *redis.Client")
	}
}

func TestRedisRepository_GetClient_ReturnsSameClient(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := &RedisRepository{redisClient: client, basePath: "x"}
	if repo.GetClient() != client {
		t.Error("expected GetClient to return the same *redis.Client")
	}
}

func TestRedisRepository_CachePath_JoinsSegments(t *testing.T) {
	repo := &RedisRepository{basePath: "svc"}
	got := repo.CachePath([]string{"memory", "u:1"})
	if got != "svc:memory:u:1" {
		t.Errorf("expected svc:memory:u:1, got %s", got)
	}
}

func TestRedisRepository_CachePath_EmptySegments(t *testing.T) {
	repo := &RedisRepository{basePath: "svc"}
	got := repo.CachePath([]string{})
	if got != "svc" {
		t.Errorf("expected svc, got %s", got)
	}
}

func TestRedisRepository_DisconnectRedisDB_NilClient_NoError(t *testing.T) {
	repo := &RedisRepository{redisClient: nil, basePath: "x"}
	// Should not panic — logs error and returns early.
	repo.DisconnectRedisDB(context.Background())
}

func TestRedisRepository_DisconnectRedisDB_ClosesClient(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := &RedisRepository{redisClient: client, basePath: "x"}
	repo.DisconnectRedisDB(context.Background())
	// After close, Ping should fail.
	if err := client.Ping(context.Background()).Err(); err == nil {
		t.Error("expected error after disconnect")
	}
}
