package fiber

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	"go.opentelemetry.io/otel/trace/noop"
)

// StreamEventHandler streams an agent turn over Server-Sent Events.
type StreamEventHandler struct {
	weave *eywa.Weave
}

func NewStreamEventHandler(weave *eywa.Weave) *StreamEventHandler {
	return &StreamEventHandler{weave: weave}
}

// streamFrame is the JSON payload of one SSE `data:` frame. Type is delta|tool_status|done|error.
type streamFrame struct {
	Type     string `json:"type"`
	Delta    string `json:"delta,omitempty"`
	Tool     string `json:"tool,omitempty"`
	Response string `json:"response,omitempty"`
	Error    string `json:"error,omitempty"`
}

// StreamEvent handles POST /api/v1/events/:event_key/stream. It converts the payload to Pulses and
// streams each turn's events as SSE frames: token deltas, tool-execution status, and a terminal
// done (with the assembled answer) or error. Payload/conversion failures are returned as JSON before
// the stream starts; failures mid-stream surface as an error frame.
func (h *StreamEventHandler) StreamEvent(c *fiber.Ctx) error {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Event/StreamEvent")
	defer span.End()

	log := newLogger()

	eventKey := c.Params("event_key")
	if eventKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "event_key path parameter is required",
		})
	}

	payload, err := resthttp.ParseWebhookPayload(c)
	if err != nil {
		log.Warnw("failed to parse webhook payload", "event_key", eventKey, "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": err.Error()})
	}

	events, err := h.weave.ConvertEventByType(ctx, eventKey, payload)
	if err != nil {
		log.Errorw("event conversion failed", "event_key", eventKey, "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "failed to convert event: " + err.Error(),
		})
	}
	if len(events) == 0 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"error":   "no events produced from payload",
		})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no") // disable nginx buffering

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		// Fresh cancellable context: the request ctx is recycled once the handler returns. Cancelling
		// on a failed write (client gone) propagates down and stops the in-flight turn.
		streamCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for _, evt := range events {
			ch, streamErr := h.weave.ProcessEventByKeyStream(streamCtx, eventKey, evt)
			if streamErr != nil {
				writeStreamFrame(w, streamFrame{Type: "error", Error: streamErr.Error()})
				return
			}
			for frame := range ch {
				if !writeStreamFrame(w, toStreamFrame(frame)) {
					return
				}
			}
		}
	}))

	return nil
}

func toStreamFrame(ev eywa.AgentStreamEvent) streamFrame {
	switch ev.Type {
	case eywa.AgentStreamDelta:
		return streamFrame{Type: "delta", Delta: ev.Delta}
	case eywa.AgentStreamToolStatus:
		return streamFrame{Type: "tool_status", Tool: ev.ToolName}
	case eywa.AgentStreamDone:
		return streamFrame{Type: "done", Response: ev.Response}
	case eywa.AgentStreamError:
		msg := ""
		if ev.Err != nil {
			msg = ev.Err.Error()
		}
		return streamFrame{Type: "error", Error: msg}
	default:
		return streamFrame{Type: string(ev.Type)}
	}
}

func writeStreamFrame(w *bufio.Writer, f streamFrame) bool {
	data, err := json.Marshal(f)
	if err != nil {
		return false
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return false
	}
	return w.Flush() == nil
}
