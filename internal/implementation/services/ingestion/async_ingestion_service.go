package ingestion

import (
	"context"
	"fmt"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers"
	"github.com/wmulabs/eywa/internal/implementation/orchestrator"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/dbg"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/tracer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// weaveEngine is the subset of orchestrator.Weave used by AsyncIngestionService.
// Defined locally so the service can be tested without constructing a full engine.
type weaveEngine interface {
	GetAsyncDispatcher() ports.Keeper
	GetEventConfiguration(eventKey string) *entities.Link
	ConvertEventByType(ctx context.Context, eventKey string, rawPayload map[string]any) ([]*entities.Pulse, error)
}

// The caller gets a < 100ms response; Keeper delivers to /internal/execute-event for processing.
type AsyncIngestionService struct {
	engine weaveEngine
}

func NewAsyncIngestionService(engine *orchestrator.Weave) *AsyncIngestionService {
	return &AsyncIngestionService{engine: engine}
}

type IngestionResult struct {
	EventIDs     []string
	EventCount   int
	ResponseTime time.Duration
}

func (s *AsyncIngestionService) Ingest(ctx context.Context, eventKey string, rawPayload map[string]any) (*IngestionResult, error) {
	startTime := helpers.NowUTC()
	ctx, span := tracer.GetTracer().Start(ctx, "Service/Ingestion/IngestAsync")
	defer span.End()

	log := dbg.GetLogger()

	span.SetAttributes(attribute.String("event.key", eventKey))

	dispatcher := s.engine.GetAsyncDispatcher()
	if dispatcher == nil {
		span.SetStatus(codes.Error, "async dispatcher not configured")
		return nil, fmt.Errorf("async dispatcher not configured")
	}

	log.Infow("processing async event", "event_key", eventKey, "payload_size", len(rawPayload))

	ingestionTimeout := 5 * time.Second
	if cfg := s.engine.GetEventConfiguration(eventKey); cfg != nil {
		ingestionTimeout = cfg.GetIngestionTimeout()
	}

	ingestionCtx, cancel := context.WithTimeout(ctx, ingestionTimeout)
	defer cancel()

	events, err := s.engine.ConvertEventByType(ingestionCtx, eventKey, rawPayload)
	if err != nil {
		log.Errorw("failed to convert event payload", "error", err, "event_key", eventKey)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to convert payload")
		return nil, fmt.Errorf("failed to convert payload: %w", err)
	}

	if len(events) == 0 {
		log.Warnw("converter returned zero events", "event_key", eventKey)
		span.SetStatus(codes.Error, "no events produced")
		return nil, fmt.Errorf("no events produced from payload")
	}

	span.SetAttributes(attribute.Int("event.count", len(events)))

	eventIDs := make([]string, 0, len(events))
	for i, event := range events {
		span.SetAttributes(
			attribute.String(fmt.Sprintf("event.%d.id", i), event.ID),
			attribute.String(fmt.Sprintf("event.%d.memory_key", i), event.MemoryKey),
		)

		if _, err := dispatcher.Schedule(ingestionCtx, eventKey, event, helpers.NowUTC()); err != nil {
			log.Errorw("failed to dispatch event",
				"error", err,
				"event_id", event.ID,
				"event_key", eventKey,
				"memory_key", event.MemoryKey,
				"event_index", i,
				"total_events", len(events),
			)
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to dispatch event")
			return nil, fmt.Errorf("failed to dispatch event: %w", err)
		}

		eventIDs = append(eventIDs, event.ID)
		log.Infow("event dispatched",
			"event_id", event.ID,
			"event_key", eventKey,
			"memory_key", event.MemoryKey,
			"event_index", i+1,
			"total_events", len(events),
		)
	}

	responseTime := time.Since(startTime)
	log.Infow("async event ingested",
		"event_key", eventKey,
		"event_count", len(events),
		"event_ids", eventIDs,
		"response_time_ms", responseTime.Milliseconds(),
	)

	span.SetAttributes(attribute.Int64("response_time_ms", responseTime.Milliseconds()))
	span.SetStatus(codes.Ok, "async event ingested")

	return &IngestionResult{
		EventIDs:     eventIDs,
		EventCount:   len(events),
		ResponseTime: responseTime,
	}, nil
}
