package fiber

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
	eywa "github.com/wmulabs/eywa"
)

type sseHandler struct {
	pubSub    eywa.PubSub
	riteRepo  eywa.RiteRepository  // optional; nil skips Rite snapshot
	vigilRepo eywa.VigilRepository // optional; nil skips Vigil snapshot
}

func newSSEHandler(pubSub eywa.PubSub, riteRepo eywa.RiteRepository, vigilRepo eywa.VigilRepository) *sseHandler {
	return &sseHandler{pubSub: pubSub, riteRepo: riteRepo, vigilRepo: vigilRepo}
}

// streamRites streams Rite lifecycle events (rite_created, rite_decided).
// On connect, sends a rite_snapshot event for each pending Rite so reconnecting
// clients see the current approval queue without a manual page refresh.
func (h *sseHandler) streamRites(c *fiberlib.Ctx) error {
	return h.stream(c, channelRites, h.riteSnapshot)
}

// streamVigil streams Vigil seat events (vigil_acquired, vigil_released) across all sessions.
// On connect, sends a vigil_snapshot event for each currently active Vigil seat.
func (h *sseHandler) streamVigil(c *fiberlib.Ctx) error {
	return h.stream(c, channelVigil, h.vigilSnapshot)
}

// streamEchoes streams per-session events (message_added, vigil_acquired, vigil_released).
// No snapshot is sent — historical echoes are available via GET /api/v1/echoes/:memoryKey.
func (h *sseHandler) streamEchoes(c *fiberlib.Ctx) error {
	memoryKey := c.Params("memoryKey")
	return h.stream(c, channelEchoPrefix+memoryKey, nil)
}

// stream is the shared SSE writer. It calls snapshot (if non-nil) before subscribing,
// then forwards Pub/Sub messages to the client until disconnect or context cancellation.
func (h *sseHandler) stream(c *fiberlib.Ctx, channel string, snapshot func(context.Context, *bufio.Writer)) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no") // disable nginx buffering

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if snapshot != nil {
			// Bound the snapshot phase to avoid blocking the subscription start
			// indefinitely if the repository is slow.
			snapCtx, snapCancel := context.WithTimeout(ctx, 10*time.Second)
			snapshot(snapCtx, w)
			snapCancel()
		}

		msgs := make(chan string, 32)

		// Subscribe in a goroutine; unblocks when ctx is cancelled (client disconnect).
		go func() {
			_ = h.pubSub.Subscribe(ctx, channel, func(msg string) {
				select {
				case msgs <- msg:
				default: // drop if buffer full — client too slow
				}
			})
		}()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case msg := <-msgs:
				if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			case <-ticker.C:
				// Heartbeat keeps connection alive through proxies.
				if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	}))

	return nil
}

func (h *sseHandler) riteSnapshot(ctx context.Context, w *bufio.Writer) {
	if h.riteRepo == nil {
		return
	}
	// Cap at 100 — approval queues are not expected to grow beyond this in practice.
	rites, _, err := h.riteRepo.List(ctx, eywa.RiteListOptions{Status: eywa.RitePending, Limit: 100})
	if err != nil {
		return
	}
	for _, r := range rites {
		data, err := json.Marshal(map[string]any{"event": "rite_snapshot", "rite": r})
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", data) //nolint:errcheck
		w.Flush()                            //nolint:errcheck
	}
}

func (h *sseHandler) vigilSnapshot(ctx context.Context, w *bufio.Writer) {
	if h.vigilRepo == nil {
		return
	}
	vigils, err := h.vigilRepo.ListAll(ctx)
	if err != nil {
		return
	}
	for _, v := range vigils {
		data, err := json.Marshal(map[string]any{"event": "vigil_snapshot", "vigil": v})
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", data) //nolint:errcheck
		w.Flush()                            //nolint:errcheck
	}
}
