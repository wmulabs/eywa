package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newPubSubClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr
}

func TestRedisPubSub_Publish_NoError(t *testing.T) {
	client, _ := newPubSubClient(t)
	ps := NewRedisPubSub(client)

	if err := ps.Publish(context.Background(), "events", "hello"); err != nil {
		t.Fatalf("Publish error: %v", err)
	}
}

func TestRedisPubSub_Subscribe_ReceivesMessage(t *testing.T) {
	client, mr := newPubSubClient(t)
	ps := NewRedisPubSub(client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	received := make(chan string, 1)
	go func() {
		_ = ps.Subscribe(ctx, "test-channel", func(msg string) {
			received <- msg
		})
	}()

	// Give subscriber time to register
	time.Sleep(50 * time.Millisecond)
	mr.Publish("test-channel", "payload-42")

	select {
	case msg := <-received:
		if msg != "payload-42" {
			t.Errorf("expected payload-42, got %s", msg)
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for message")
	}
}

func TestRedisPubSub_Subscribe_ContextCancel_Returns(t *testing.T) {
	client, _ := newPubSubClient(t)
	ps := NewRedisPubSub(client)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- ps.Subscribe(ctx, "ch", func(_ string) {})
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Error("Subscribe did not return after context cancel")
	}
}
