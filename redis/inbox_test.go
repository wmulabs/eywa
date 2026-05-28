package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newInboxClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr
}

func TestInbox_Push_PopAll_ReturnsMessages(t *testing.T) {
	client, _ := newInboxClient(t)
	inbox := NewInbox(client, 10*time.Second)

	_ = inbox.Push(context.Background(), "user:1", "msg1")
	_ = inbox.Push(context.Background(), "user:1", "msg2")
	_ = inbox.Push(context.Background(), "user:1", "msg3")

	msgs, err := inbox.PopAll(context.Background(), "user:1")
	if err != nil {
		t.Fatalf("PopAll error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0] != "msg1" || msgs[1] != "msg2" || msgs[2] != "msg3" {
		t.Errorf("unexpected message order: %v", msgs)
	}
}

func TestInbox_PopAll_EmptyInbox_ReturnsEmpty(t *testing.T) {
	client, _ := newInboxClient(t)
	inbox := NewInbox(client, 10*time.Second)

	msgs, err := inbox.PopAll(context.Background(), "user:nobody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected empty, got %v", msgs)
	}
}

func TestInbox_PopAll_DeletesAfterRead(t *testing.T) {
	client, mr := newInboxClient(t)
	inbox := NewInbox(client, 10*time.Second)

	_ = inbox.Push(context.Background(), "user:1", "msg")
	_, _ = inbox.PopAll(context.Background(), "user:1")

	key := "eywa:inbox:user:1"
	if mr.Exists(key) {
		t.Error("expected inbox key deleted after PopAll")
	}
}

func TestInbox_Push_SetsTTL(t *testing.T) {
	client, mr := newInboxClient(t)
	inbox := NewInbox(client, 5*time.Second)

	_ = inbox.Push(context.Background(), "user:ttl", "msg")

	ttl := mr.TTL("eywa:inbox:user:ttl")
	if ttl <= 0 {
		t.Errorf("expected TTL > 0, got %v", ttl)
	}
}

func TestInbox_PopAll_SecondCall_ReturnsEmpty(t *testing.T) {
	client, _ := newInboxClient(t)
	inbox := NewInbox(client, 10*time.Second)

	_ = inbox.Push(context.Background(), "user:1", "hello")
	_, _ = inbox.PopAll(context.Background(), "user:1")

	msgs, err := inbox.PopAll(context.Background(), "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected empty on second pop, got %v", msgs)
	}
}

func TestInbox_KeyNamespace(t *testing.T) {
	client, mr := newInboxClient(t)
	inbox := NewInbox(client, 10*time.Second)

	_ = inbox.Push(context.Background(), "tenant:user:42", "x")

	keys := mr.Keys()
	found := false
	for _, k := range keys {
		if k == "eywa:inbox:tenant:user:42" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected namespaced key, got: %v", keys)
	}
}

func TestInbox_PopAll_PipelineError_ReturnsError(t *testing.T) {
	client, mr := newInboxClient(t)
	inbox := NewInbox(client, 10*time.Second)

	// Push something so there's data
	_ = inbox.Push(context.Background(), "user:fail", "msg")

	// Close miniredis to force pipeline failure
	mr.Close()

	_, err := inbox.PopAll(context.Background(), "user:fail")
	if err == nil {
		t.Error("expected error when Redis is unavailable")
	}
}
