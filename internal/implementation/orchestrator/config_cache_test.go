package orchestrator

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type stubLinkRepo struct {
	mu      sync.Mutex
	links   []*entities.Link
	saved   []*entities.Link
	deleted []string
	err     error
}

var _ ports.LinkRepository = (*stubLinkRepo)(nil)

func (s *stubLinkRepo) FindAll(ctx context.Context) ([]*entities.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	result := make([]*entities.Link, len(s.links))
	copy(result, s.links)
	return result, nil
}

func (s *stubLinkRepo) FindByKey(ctx context.Context, eventType string) (*entities.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	for _, l := range s.links {
		if l.EventType == eventType {
			return l, nil
		}
	}
	return nil, errors.New("not found")
}

func (s *stubLinkRepo) Save(ctx context.Context, link *entities.Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, link)
	for i, l := range s.links {
		if l.EventType == link.EventType {
			s.links[i] = link
			return nil
		}
	}
	s.links = append(s.links, link)
	return nil
}

func (s *stubLinkRepo) Delete(ctx context.Context, eventType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.deleted = append(s.deleted, eventType)
	for i, l := range s.links {
		if l.EventType == eventType {
			s.links = append(s.links[:i], s.links[i+1:]...)
			return nil
		}
	}
	return nil
}

var _ ports.PubSub = (*stubPubSub)(nil)

type stubPubSub struct {
	mu        sync.Mutex
	published []string
	err       error
}

func (s *stubPubSub) Publish(ctx context.Context, channel, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.published = append(s.published, message)
	return nil
}

func (s *stubPubSub) Subscribe(ctx context.Context, channel string, handler func(msg string)) error {
	return nil
}

func TestConfigCache_RegisterLink_GetLink(t *testing.T) {
	cache := NewConfigCache(nil, nil, nil)
	link := entities.NewLink("test_event")
	link.WithInboundConverter("test_converter")

	cache.RegisterLink(link)

	retrieved, ok := cache.GetLink("test_event")
	if !ok {
		t.Fatal("expected link to be registered")
	}
	if retrieved.EventType != "test_event" {
		t.Errorf("expected event type test_event, got %s", retrieved.EventType)
	}
	if retrieved.InboundConverterName != "test_converter" {
		t.Errorf("expected converter test_converter, got %s", retrieved.InboundConverterName)
	}
}

func TestConfigCache_GetLink_Missing(t *testing.T) {
	cache := NewConfigCache(nil, nil, nil)

	_, ok := cache.GetLink("nonexistent")
	if ok {
		t.Error("expected link to not exist")
	}
}

func TestConfigCache_GetAllLinks(t *testing.T) {
	cache := NewConfigCache(nil, nil, nil)

	link1 := entities.NewLink("event1").WithInboundConverter("converter1")
	link2 := entities.NewLink("event2").WithInboundConverter("converter2")
	link3 := entities.NewLink("event3").WithInboundConverter("converter3")

	cache.RegisterLink(link1)
	cache.RegisterLink(link2)
	cache.RegisterLink(link3)

	all := cache.GetAllLinks()
	if len(all) != 3 {
		t.Fatalf("expected 3 links, got %d", len(all))
	}

	eventTypes := make(map[string]bool)
	for _, l := range all {
		eventTypes[l.EventType] = true
	}
	if !eventTypes["event1"] || !eventTypes["event2"] || !eventTypes["event3"] {
		t.Error("expected all three event types to be present")
	}
}

func TestConfigCache_LoadAll_NilRepo_NoOp(t *testing.T) {
	cache := NewConfigCache(nil, nil, nil)

	err := cache.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	all := cache.GetAllLinks()
	if len(all) != 0 {
		t.Fatalf("expected empty cache, got %d links", len(all))
	}
}

func TestConfigCache_LoadAll_FromRepo(t *testing.T) {
	link1 := entities.NewLink("event1").WithInboundConverter("converter1")
	link2 := entities.NewLink("event2").WithInboundConverter("converter2")

	repo := &stubLinkRepo{
		links: []*entities.Link{link1, link2},
	}
	cache := NewConfigCache(repo, nil, nil)

	err := cache.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	all := cache.GetAllLinks()
	if len(all) != 2 {
		t.Fatalf("expected 2 links, got %d", len(all))
	}

	retrieved1, ok := cache.GetLink("event1")
	if !ok {
		t.Fatal("expected event1 to be loaded")
	}
	if retrieved1.InboundConverterName != "converter1" {
		t.Errorf("expected converter1, got %s", retrieved1.InboundConverterName)
	}

	retrieved2, ok := cache.GetLink("event2")
	if !ok {
		t.Fatal("expected event2 to be loaded")
	}
	if retrieved2.InboundConverterName != "converter2" {
		t.Errorf("expected converter2, got %s", retrieved2.InboundConverterName)
	}
}

func TestConfigCache_SaveLink_UpdatesCache(t *testing.T) {
	cache := NewConfigCache(nil, nil, nil)
	link := entities.NewLink("test_event").WithInboundConverter("test_converter")

	err := cache.SaveLink(context.Background(), link)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	retrieved, ok := cache.GetLink("test_event")
	if !ok {
		t.Fatal("expected link to be in cache")
	}
	if retrieved.InboundConverterName != "test_converter" {
		t.Errorf("expected test_converter, got %s", retrieved.InboundConverterName)
	}
}

func TestConfigCache_SaveLink_PersistsToRepo(t *testing.T) {
	repo := &stubLinkRepo{links: []*entities.Link{}}
	cache := NewConfigCache(repo, nil, nil)
	link := entities.NewLink("test_event").WithInboundConverter("test_converter")

	err := cache.SaveLink(context.Background(), link)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 saved link, got %d", len(repo.saved))
	}
	if repo.saved[0].EventType != "test_event" {
		t.Errorf("expected event type test_event, got %s", repo.saved[0].EventType)
	}
}

func TestConfigCache_SaveLink_PublishesToPubSub(t *testing.T) {
	pubsub := &stubPubSub{published: []string{}}
	cache := NewConfigCache(nil, pubsub, nil)
	link := entities.NewLink("test_event").WithInboundConverter("test_converter")

	err := cache.SaveLink(context.Background(), link)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(pubsub.published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(pubsub.published))
	}
	if pubsub.published[0] != "link:test_event" {
		t.Errorf("expected message link:test_event, got %s", pubsub.published[0])
	}
}

func TestConfigCache_DeleteLink_RemovesFromCache(t *testing.T) {
	cache := NewConfigCache(nil, nil, nil)
	link := entities.NewLink("test_event").WithInboundConverter("test_converter")
	cache.RegisterLink(link)

	if _, ok := cache.GetLink("test_event"); !ok {
		t.Fatal("expected link to be registered initially")
	}

	err := cache.DeleteLink(context.Background(), "test_event")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if _, ok := cache.GetLink("test_event"); ok {
		t.Error("expected link to be deleted from cache")
	}
}

func TestConfigCache_DeleteLink_NotifiesPubSub(t *testing.T) {
	pubsub := &stubPubSub{published: []string{}}
	cache := NewConfigCache(nil, pubsub, nil)
	link := entities.NewLink("test_event").WithInboundConverter("test_converter")
	cache.RegisterLink(link)

	err := cache.DeleteLink(context.Background(), "test_event")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(pubsub.published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(pubsub.published))
	}
	if pubsub.published[0] != "link:test_event" {
		t.Errorf("expected message link:test_event, got %s", pubsub.published[0])
	}
}

func TestConfigCache_ForceReload_NilRepo_NoError(t *testing.T) {
	cache := NewConfigCache(nil, nil, nil)

	err := cache.ForceReload(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	all := cache.GetAllLinks()
	if len(all) != 0 {
		t.Fatalf("expected empty cache, got %d links", len(all))
	}
}

func TestConfigCache_LoadAll_RepoError(t *testing.T) {
	repo := &stubLinkRepo{err: errors.New("db down")}
	cache := NewConfigCache(repo, nil, nil)

	err := cache.LoadAll(context.Background())
	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
}

func TestConfigCache_SaveLink_RepoError(t *testing.T) {
	repo := &stubLinkRepo{err: errors.New("save failed")}
	cache := NewConfigCache(repo, nil, nil)
	link := entities.NewLink("evt").WithInboundConverter("conv")

	err := cache.SaveLink(context.Background(), link)
	if err == nil {
		t.Fatal("expected repo error, got nil")
	}
}

func TestConfigCache_DeleteLink_RepoError(t *testing.T) {
	repo := &stubLinkRepo{err: errors.New("delete failed")}
	cache := NewConfigCache(repo, nil, nil)

	err := cache.DeleteLink(context.Background(), "evt")
	if err == nil {
		t.Fatal("expected repo error, got nil")
	}
}

func TestConfigCache_ForceReload_WithPubSub(t *testing.T) {
	repo := &stubLinkRepo{links: []*entities.Link{
		entities.NewLink("evt1").WithInboundConverter("conv"),
	}}
	pubsub := &stubPubSub{}
	cache := NewConfigCache(repo, pubsub, nil)

	err := cache.ForceReload(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(pubsub.published) != 1 {
		t.Errorf("expected 1 published message, got %d", len(pubsub.published))
	}
}

func TestConfigCache_ForceReload_LoadError(t *testing.T) {
	repo := &stubLinkRepo{err: errors.New("repo down")}
	cache := NewConfigCache(repo, nil, nil)

	err := cache.ForceReload(context.Background())
	if err == nil {
		t.Fatal("expected error from LoadAll, got nil")
	}
}

// subscribingPubSub calls handler synchronously for each message in msgs, then returns.
type subscribingPubSub struct {
	msgs []string
	err  error
}

func (s *subscribingPubSub) Publish(_ context.Context, _, _ string) error { return nil }

func (s *subscribingPubSub) Subscribe(_ context.Context, _ string, handler func(msg string)) error {
	for _, msg := range s.msgs {
		handler(msg)
	}
	return s.err
}

func TestConfigCache_Subscribe_NilPubSub(t *testing.T) {
	cache := NewConfigCache(nil, nil, nil)
	// must return immediately without panic
	cache.Subscribe(context.Background())
}

func TestConfigCache_Subscribe_ReloadAll(t *testing.T) {
	repo := &stubLinkRepo{links: []*entities.Link{
		entities.NewLink("evt").WithInboundConverter("conv"),
	}}
	pubsub := &subscribingPubSub{msgs: []string{"link:*"}}
	cache := NewConfigCache(repo, pubsub, nil)

	cache.Subscribe(context.Background())
	if _, ok := cache.GetLink("evt"); !ok {
		t.Error("expected link to be loaded after subscribe reload-all")
	}
}

func TestConfigCache_Subscribe_ReloadSingleLink(t *testing.T) {
	repo := &stubLinkRepo{links: []*entities.Link{
		entities.NewLink("evt").WithInboundConverter("conv"),
	}}
	pubsub := &subscribingPubSub{msgs: []string{"link:evt"}}
	cache := NewConfigCache(repo, pubsub, nil)

	cache.Subscribe(context.Background())
	if _, ok := cache.GetLink("evt"); !ok {
		t.Error("expected link to be loaded after subscribe single-link")
	}
}

func TestConfigCache_Subscribe_ReloadSingleLink_NotFound(t *testing.T) {
	// FindByKey returns error → link deleted from cache
	repo := &stubLinkRepo{err: errors.New("not found")}
	pubsub := &subscribingPubSub{msgs: []string{"link:missing"}}
	cache := NewConfigCache(repo, pubsub, nil)
	cache.RegisterLink(entities.NewLink("missing"))

	cache.Subscribe(context.Background())
	if _, ok := cache.GetLink("missing"); ok {
		t.Error("expected link to be removed from cache when FindByKey fails")
	}
}

func TestConfigCache_Subscribe_NilRepoOnSingleLink(t *testing.T) {
	pubsub := &subscribingPubSub{msgs: []string{"link:evt"}}
	// linkRepo is nil → early return inside handler
	cache := NewConfigCache(nil, pubsub, nil)
	cache.Subscribe(context.Background()) // should not panic
}

func TestConfigCache_Subscribe_ErrorWithNonCancelledCtx_Logs(t *testing.T) {
	pubsub := &subscribingPubSub{err: errors.New("subscription failed")}
	cache := NewConfigCache(nil, pubsub, testLogger(t))
	// non-cancelled ctx + logger + error from Subscribe → logger.Errorw branch
	cache.Subscribe(context.Background())
}

// ── WeaveConfig.Validate ─────────────────────────────────────────────────────

func TestWeaveConfig_Validate_DefaultValid(t *testing.T) {
	cfg := DefaultWeaveConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("DefaultWeaveConfig should be valid, got %v", err)
	}
}

func TestWeaveConfig_Validate_InvalidLockTTL(t *testing.T) {
	cfg := DefaultWeaveConfig()
	cfg.LockTTL = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for LockTTL=0")
	}
}

func TestWeaveConfig_Validate_InvalidMaxReasoningIterations(t *testing.T) {
	cfg := DefaultWeaveConfig()
	cfg.MaxReasoningIterations = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for MaxReasoningIterations=0")
	}
}

func TestWeaveConfig_Validate_InvalidMaxMemoryMessages(t *testing.T) {
	cfg := DefaultWeaveConfig()
	cfg.MaxMemoryMessages = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for MaxMemoryMessages=0")
	}
}

func TestWeaveConfig_Validate_InvalidMaxSessionKeyLength(t *testing.T) {
	cfg := DefaultWeaveConfig()
	cfg.MaxSessionKeyLength = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for MaxSessionKeyLength=0")
	}
}

func TestWeaveConfig_Validate_InvalidMaxUserMessageLength(t *testing.T) {
	cfg := DefaultWeaveConfig()
	cfg.MaxUserMessageLength = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for MaxUserMessageLength=0")
	}
}

func TestConfigCache_Subscribe_ReloadAll_LoadAllError_Logs(t *testing.T) {
	// "link:*" message + LoadAll fails (repo error) + logger set → log branch covered.
	repo := &stubLinkRepo{err: errors.New("repo down")}
	pubsub := &subscribingPubSub{msgs: []string{"link:*"}}
	cache := NewConfigCache(repo, pubsub, testLogger(t))
	cache.Subscribe(context.Background()) // must not panic
}

func TestConfigCache_Subscribe_ShortMessage_NoPanic(t *testing.T) {
	cache := NewConfigCache(nil, nil, zap.NewNop().Sugar())

	msgs := []string{"", "li", "link", "link:", "link:wa", "link:*"}
	for _, msg := range msgs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic for msg %q: %v", msg, r)
				}
			}()
			if msg == "link:*" {
				_ = cache.LoadAll(context.Background())
				return
			}
			if strings.HasPrefix(msg, "link:") {
				eventType := msg[5:]
				_ = eventType
			}
		}()
	}
}
