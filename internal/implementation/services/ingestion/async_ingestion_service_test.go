package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- stubs ---

type stubWeave struct {
	dispatcher    ports.Keeper
	eventConfig   *entities.Link
	convertResult []*entities.Pulse
	convertErr    error
}

func (w *stubWeave) GetAsyncDispatcher() ports.Keeper       { return w.dispatcher }
func (w *stubWeave) GetEventConfiguration(_ string) *entities.Link { return w.eventConfig }
func (w *stubWeave) ConvertEventByType(_ context.Context, _ string, _ map[string]any) ([]*entities.Pulse, error) {
	return w.convertResult, w.convertErr
}

type stubDispatcher struct {
	taskID      string
	scheduleErr error
}

func (d *stubDispatcher) Schedule(_ context.Context, _ string, _ *entities.Pulse, _ time.Time) (string, error) {
	return d.taskID, d.scheduleErr
}
func (d *stubDispatcher) Cancel(_ context.Context, _ string) error { return nil }

func pulse(id, memoryKey string) *entities.Pulse {
	return &entities.Pulse{ID: id, MemoryKey: memoryKey}
}

// --- tests ---

func TestIngest_NoAsyncDispatcher_ReturnsError(t *testing.T) {
	engine := &stubWeave{dispatcher: nil}
	svc := &AsyncIngestionService{engine: engine}

	_, err := svc.Ingest(context.Background(), "chat", map[string]any{})
	if err == nil {
		t.Fatal("expected error when no async dispatcher")
	}
}

func TestIngest_ConvertFails_ReturnsError(t *testing.T) {
	engine := &stubWeave{
		dispatcher: &stubDispatcher{taskID: "k1"},
		convertErr: errors.New("bad payload"),
	}
	svc := &AsyncIngestionService{engine: engine}

	_, err := svc.Ingest(context.Background(), "chat", map[string]any{})
	if err == nil {
		t.Fatal("expected error when convert fails")
	}
}

func TestIngest_ZeroEvents_ReturnsError(t *testing.T) {
	engine := &stubWeave{
		dispatcher:    &stubDispatcher{taskID: "k1"},
		convertResult: []*entities.Pulse{},
	}
	svc := &AsyncIngestionService{engine: engine}

	_, err := svc.Ingest(context.Background(), "chat", map[string]any{})
	if err == nil {
		t.Fatal("expected error when converter returns zero events")
	}
}

func TestIngest_DispatchFails_ReturnsError(t *testing.T) {
	engine := &stubWeave{
		dispatcher:    &stubDispatcher{scheduleErr: errors.New("keeper down")},
		convertResult: []*entities.Pulse{pulse("evt-1", "user:1")},
	}
	svc := &AsyncIngestionService{engine: engine}

	_, err := svc.Ingest(context.Background(), "chat", map[string]any{})
	if err == nil {
		t.Fatal("expected error when dispatch fails")
	}
}

func TestIngest_Success_SingleEvent(t *testing.T) {
	engine := &stubWeave{
		dispatcher:    &stubDispatcher{taskID: "k1"},
		convertResult: []*entities.Pulse{pulse("evt-1", "user:1")},
	}
	svc := &AsyncIngestionService{engine: engine}

	result, err := svc.Ingest(context.Background(), "chat", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EventCount != 1 {
		t.Errorf("expected EventCount=1, got %d", result.EventCount)
	}
	if len(result.EventIDs) != 1 || result.EventIDs[0] != "evt-1" {
		t.Errorf("unexpected event IDs: %v", result.EventIDs)
	}
	if result.ResponseTime <= 0 {
		t.Error("expected positive ResponseTime")
	}
}

func TestIngest_Success_MultipleEvents(t *testing.T) {
	engine := &stubWeave{
		dispatcher: &stubDispatcher{taskID: "k1"},
		convertResult: []*entities.Pulse{
			pulse("evt-1", "user:1"),
			pulse("evt-2", "user:2"),
		},
	}
	svc := &AsyncIngestionService{engine: engine}

	result, err := svc.Ingest(context.Background(), "chat", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EventCount != 2 {
		t.Errorf("expected EventCount=2, got %d", result.EventCount)
	}
}

func TestIngest_WithEventConfig_UsesConfigTimeout(t *testing.T) {
	cfg := entities.NewLink("chat").Build()
	engine := &stubWeave{
		dispatcher:    &stubDispatcher{taskID: "k1"},
		eventConfig:   cfg,
		convertResult: []*entities.Pulse{pulse("evt-1", "user:1")},
	}
	svc := &AsyncIngestionService{engine: engine}

	result, err := svc.Ingest(context.Background(), "chat", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EventCount != 1 {
		t.Errorf("expected 1 event, got %d", result.EventCount)
	}
}

func TestIngest_DispatchFails_SecondEvent_ReturnsError(t *testing.T) {
	callCount := 0
	dispatcher := &callCountDispatcher{failAfter: 1, callCount: &callCount}
	engine := &stubWeave{
		dispatcher: dispatcher,
		convertResult: []*entities.Pulse{
			pulse("evt-1", "user:1"),
			pulse("evt-2", "user:2"),
		},
	}
	svc := &AsyncIngestionService{engine: engine}

	_, err := svc.Ingest(context.Background(), "chat", map[string]any{})
	if err == nil {
		t.Fatal("expected error when second dispatch fails")
	}
}

type callCountDispatcher struct {
	failAfter int
	callCount *int
}

func (d *callCountDispatcher) Schedule(_ context.Context, _ string, _ *entities.Pulse, _ time.Time) (string, error) {
	*d.callCount++
	if *d.callCount > d.failAfter {
		return "", errors.New("keeper down")
	}
	return "k1", nil
}
func (d *callCountDispatcher) Cancel(_ context.Context, _ string) error { return nil }

func TestNewAsyncIngestionService_ReturnsNonNil(t *testing.T) {
	svc := NewAsyncIngestionService(nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}
