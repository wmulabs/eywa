package fiber

import (
	"context"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

// stubPubSub is a simple in-memory PubSub.
type stubPubSub struct {
	published []struct{ channel, message string }
	pubErr    error
}

func (s *stubPubSub) Publish(_ context.Context, channel, message string) error {
	s.published = append(s.published, struct{ channel, message string }{channel, message})
	return s.pubErr
}
func (s *stubPubSub) Subscribe(_ context.Context, _ string, _ func(string)) error {
	return nil
}

var _ eywa.PubSub = (*stubPubSub)(nil)

func TestEventBus_NewEventBus_NilPubSub_ReturnsNil(t *testing.T) {
	b := NewEventBus(nil)
	if b != nil {
		t.Error("expected nil bus for nil pubSub")
	}
}

func TestEventBus_NewEventBus_WithPubSub_ReturnsNonNil(t *testing.T) {
	b := NewEventBus(&stubPubSub{})
	if b == nil {
		t.Error("expected non-nil bus")
	}
}

func TestEventBus_NilReceiver_PublishRiteDecided_NoOp(t *testing.T) {
	var b *EventBus
	// Should not panic
	b.PublishRiteDecided(context.Background(), "rite-1", eywa.RiteApproved)
}

func TestEventBus_NilReceiver_PublishVigilAcquired_NoOp(t *testing.T) {
	var b *EventBus
	b.PublishVigilAcquired(context.Background(), "mem1", "op-1")
}

func TestEventBus_NilReceiver_PublishVigilReleased_NoOp(t *testing.T) {
	var b *EventBus
	b.PublishVigilReleased(context.Background(), "mem1")
}

func TestEventBus_NilReceiver_PublishEchoAdded_NoOp(t *testing.T) {
	var b *EventBus
	b.PublishEchoAdded(context.Background(), "mem1", eywa.Echo{})
}

func TestEventBus_PublishRiteDecided_PublishesMessage(t *testing.T) {
	ps := &stubPubSub{}
	b := NewEventBus(ps)
	b.PublishRiteDecided(context.Background(), "rite-1", eywa.RiteApproved)
	if len(ps.published) != 1 {
		t.Errorf("expected 1 publish, got %d", len(ps.published))
	}
	if ps.published[0].channel != channelRites {
		t.Errorf("expected channel %s, got %s", channelRites, ps.published[0].channel)
	}
}

func TestEventBus_PublishVigilAcquired_PublishesTwoMessages(t *testing.T) {
	ps := &stubPubSub{}
	b := NewEventBus(ps)
	b.PublishVigilAcquired(context.Background(), "mem1", "op-1")
	if len(ps.published) != 2 {
		t.Errorf("expected 2 publishes (vigil + echo channel), got %d", len(ps.published))
	}
}

func TestEventBus_PublishVigilReleased_PublishesTwoMessages(t *testing.T) {
	ps := &stubPubSub{}
	b := NewEventBus(ps)
	b.PublishVigilReleased(context.Background(), "mem1")
	if len(ps.published) != 2 {
		t.Errorf("expected 2 publishes, got %d", len(ps.published))
	}
}

func TestEventBus_PublishEchoAdded_PublishesMessage(t *testing.T) {
	ps := &stubPubSub{}
	b := NewEventBus(ps)
	b.PublishEchoAdded(context.Background(), "mem1", eywa.Echo{ID: "e1"})
	if len(ps.published) != 1 {
		t.Errorf("expected 1 publish, got %d", len(ps.published))
	}
}

func TestEventBus_publish_PubSubError_Ignored(t *testing.T) {
	ps := &stubPubSub{pubErr: errInternal}
	b := NewEventBus(ps)
	// Should not panic despite pubSub error
	b.PublishRiteDecided(context.Background(), "rite-1", eywa.RiteApproved)
}

func TestEventBus_publish_MarshalError_Ignored(t *testing.T) {
	ps := &stubPubSub{}
	b := NewEventBus(ps)
	// func type cannot be marshaled to JSON — triggers the err != nil return branch
	b.publish(context.Background(), "chan", map[string]any{"fn": func() {}})
	if len(ps.published) != 0 {
		t.Error("want no publish on marshal error")
	}
}
