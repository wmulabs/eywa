package fiber

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

func buildStreamHandlerApp(weave *eywa.Weave) *fiberlib.App {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	h := NewStreamEventHandler(weave)
	app.Post("/api/v1/events/:event_key/stream", h.StreamEvent)
	return app
}

func streamWeave(t *testing.T, receptor *testReceptor) *eywa.Weave {
	t.Helper()
	linkRepo := newStubLinkRepo(
		eywa.NewLink("test_event").WithInboundConverter(receptor.name).Build(),
	)
	weave, err := eywa.NewWeaveBuilder(context.Background()).
		WithSpiritRepository(&testSpiritRepo{}).
		WithMemoryRepository(&testMemoryRepo{}).
		WithEchoRepository(&testEchoRepo{}).
		WithChronicleRepository(&testChronicleRepo{}).
		WithBond(&testBond{}).
		WithOracleFactory(&testOracleFactory{}).
		WithActionRegistry(eywa.NewActionRegistry()).
		WithLinkRepository(linkRepo).
		Build()
	if err != nil {
		t.Fatalf("failed to build stream weave: %v", err)
	}
	weave.RegisterReceptor(receptor.name, receptor)
	return weave
}

func TestStreamEventHandler_MissingEventKey_Returns400(t *testing.T) {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	h := NewStreamEventHandler(minimalTestWeave(t))
	app.Post("/api/v1/events/", h.StreamEvent)

	req := httptest.NewRequest("POST", "/api/v1/events/", strings.NewReader(`{"x":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestStreamEventHandler_ConvertError_Returns400(t *testing.T) {
	app := buildStreamHandlerApp(minimalTestWeave(t))

	req := httptest.NewRequest("POST", "/api/v1/events/unknown_event/stream", strings.NewReader(`{"test":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestStreamEventHandler_InvalidPayload_Returns400(t *testing.T) {
	app := buildStreamHandlerApp(minimalTestWeave(t))

	req := httptest.NewRequest("POST", "/api/v1/events/test_event/stream", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestStreamEventHandler_NoEvents_Returns422(t *testing.T) {
	receptor := &testReceptor{name: "stream-receptor", events: nil}
	app := buildStreamHandlerApp(streamWeave(t, receptor))

	req := httptest.NewRequest("POST", "/api/v1/events/test_event/stream", strings.NewReader(`{"test":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 422 {
		t.Errorf("want 422, got %d", resp.StatusCode)
	}
}

// A Pulse converts successfully but the pipeline fails to load a Spirit (none configured), so the
// turn surfaces an error frame — exercising the full SSE writer path end to end.
func TestStreamEventHandler_StreamsErrorFrame(t *testing.T) {
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "test", User: "user1"}).Build()
	receptor := &testReceptor{name: "stream-receptor", events: []*eywa.Pulse{pulse}}
	app := buildStreamHandlerApp(streamWeave(t, receptor))

	req := httptest.NewRequest("POST", "/api/v1/events/test_event/stream", strings.NewReader(`{"test":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("expected SSE content type, got %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "data: ") {
		t.Errorf("expected at least one SSE data frame, got %q", body)
	}
	if !strings.Contains(string(body), `"type":"error"`) {
		t.Errorf("expected an error frame when no Spirit is configured, got %q", body)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("connection closed") }

func TestWriteStreamFrame_FlushError_ReturnsFalse(t *testing.T) {
	w := bufio.NewWriter(failingWriter{})
	if writeStreamFrame(w, streamFrame{Type: "delta", Delta: "x"}) {
		t.Error("expected false when the underlying writer fails to flush")
	}
}

func TestWriteStreamFrame_WriteError_ReturnsFalse(t *testing.T) {
	// A 1-byte buffer forces a flush mid-write, surfacing the write error from Fprintf.
	w := bufio.NewWriterSize(failingWriter{}, 1)
	if writeStreamFrame(w, streamFrame{Type: "delta", Delta: "hello"}) {
		t.Error("expected false when the underlying write fails")
	}
}

func TestToStreamFrame(t *testing.T) {
	cases := []struct {
		name string
		in   eywa.AgentStreamEvent
		want streamFrame
	}{
		{"delta", eywa.AgentStreamEvent{Type: eywa.AgentStreamDelta, Delta: "hi"}, streamFrame{Type: "delta", Delta: "hi"}},
		{"tool", eywa.AgentStreamEvent{Type: eywa.AgentStreamToolStatus, ToolName: "lookup"}, streamFrame{Type: "tool_status", Tool: "lookup"}},
		{"done", eywa.AgentStreamEvent{Type: eywa.AgentStreamDone, Response: "answer"}, streamFrame{Type: "done", Response: "answer"}},
		{"error", eywa.AgentStreamEvent{Type: eywa.AgentStreamError, Err: context.Canceled}, streamFrame{Type: "error", Error: context.Canceled.Error()}},
		{"error_nil", eywa.AgentStreamEvent{Type: eywa.AgentStreamError}, streamFrame{Type: "error"}},
		{"unknown", eywa.AgentStreamEvent{Type: eywa.AgentStreamEventType("weird")}, streamFrame{Type: "weird"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := toStreamFrame(tc.in); got != tc.want {
				t.Errorf("toStreamFrame(%+v) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}
