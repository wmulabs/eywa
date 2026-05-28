package fiber

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
)

// --- mock repos ---

type mockRiteRepo struct {
	rites []*eywa.Rite
}

func (m *mockRiteRepo) Create(_ context.Context, _ *eywa.Rite) error { return nil }
func (m *mockRiteRepo) FindByID(_ context.Context, _ string) (*eywa.Rite, error) {
	return nil, nil
}
func (m *mockRiteRepo) List(_ context.Context, _ eywa.RiteListOptions) ([]*eywa.Rite, int64, error) {
	return m.rites, int64(len(m.rites)), nil
}
func (m *mockRiteRepo) Decide(_ context.Context, _, _ string, _ eywa.RiteStatus) error {
	return nil
}

type mockVigilRepo struct {
	vigils []*eywa.Vigil
}

func (m *mockVigilRepo) Acquire(_ context.Context, _, _ string, _ time.Duration) error {
	return nil
}
func (m *mockVigilRepo) Release(_ context.Context, _ string) error { return nil }
func (m *mockVigilRepo) Get(_ context.Context, _ string) (*eywa.Vigil, error) {
	return nil, nil
}
func (m *mockVigilRepo) Refresh(_ context.Context, _ string, _ time.Duration) error { return nil }
func (m *mockVigilRepo) ListAll(_ context.Context) ([]*eywa.Vigil, error) {
	return m.vigils, nil
}

// --- tests ---

func TestSSEHandler_RiteSnapshot_SendsPendingRites(t *testing.T) {
	rites := []*eywa.Rite{
		{ID: "rite-1", Status: eywa.RitePending, Reason: "need approval"},
	}
	h := &sseHandler{riteRepo: &mockRiteRepo{rites: rites}}

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	h.riteSnapshot(context.Background(), w)
	w.Flush()

	out := buf.String()
	if out == "" {
		t.Fatal("expected rite_snapshot event, got empty output")
	}

	// Parse the first SSE event (format: "data: {...}\n\n").
	// Splitting on \n\n handles multi-event buffers safely.
	firstEvent := strings.SplitN(out, "\n\n", 2)[0]
	jsonPart := strings.TrimPrefix(firstEvent, "data: ")
	var payload map[string]any
	if err := json.Unmarshal([]byte(jsonPart), &payload); err != nil {
		t.Fatalf("unmarshal snapshot payload: %v", err)
	}
	if payload["event"] != "rite_snapshot" {
		t.Errorf("expected event=rite_snapshot, got %v", payload["event"])
	}
}

func TestSSEHandler_RiteSnapshot_NilRepo_WritesNothing(t *testing.T) {
	h := &sseHandler{riteRepo: nil}

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	h.riteSnapshot(context.Background(), w)
	w.Flush()

	if buf.Len() != 0 {
		t.Errorf("expected empty output with nil riteRepo, got %q", buf.String())
	}
}

func TestSSEHandler_VigilSnapshot_SendsActiveVigils(t *testing.T) {
	now := time.Now()
	vigils := []*eywa.Vigil{
		{MemoryKey: "ch:user:1", OperatorID: "op-1", SeatSince: now},
	}
	h := &sseHandler{vigilRepo: &mockVigilRepo{vigils: vigils}}

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	h.vigilSnapshot(context.Background(), w)
	w.Flush()

	out := buf.String()
	if out == "" {
		t.Fatal("expected vigil_snapshot event, got empty output")
	}

	firstEvent := strings.SplitN(out, "\n\n", 2)[0]
	jsonPart := strings.TrimPrefix(firstEvent, "data: ")
	var payload map[string]any
	if err := json.Unmarshal([]byte(jsonPart), &payload); err != nil {
		t.Fatalf("unmarshal snapshot payload: %v", err)
	}
	if payload["event"] != "vigil_snapshot" {
		t.Errorf("expected event=vigil_snapshot, got %v", payload["event"])
	}
}

func TestSSEHandler_VigilSnapshot_NilRepo_WritesNothing(t *testing.T) {
	h := &sseHandler{vigilRepo: nil}

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	h.vigilSnapshot(context.Background(), w)
	w.Flush()

	if buf.Len() != 0 {
		t.Errorf("expected empty output with nil vigilRepo, got %q", buf.String())
	}
}
