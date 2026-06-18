package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers"
	"github.com/wmulabs/eywa/internal/helpers/chunking"
	"github.com/wmulabs/eywa/internal/implementation/pathfinders"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	MaxReasoningIterations = 5
)

// AppInfo contains build metadata used by health and monitoring endpoints.
type AppInfo struct {
	Name    string
	Version string
}

// Weave is the core runtime engine — wires event processing, Spirit selection, reasoning, and response delivery.
type Weave struct {
	scoutRegistry       ports.ScoutRegistry
	pathfinderRegistry  ports.PathfinderRegistry
	logicRouterRegistry ports.LogicRouterRegistry
	voiceRegistry       ports.VoiceRegistry
	loreHarvester       ports.LoreHarvester
	loreRepository      ports.LoreRepository
	loreStore           ports.LoreStore
	loreEmbedder        ports.LoreEmbedder

	imprintHarvester     ports.ImprintHarvester
	imprintRepository    ports.ImprintRepository
	imprintExtractionCfg ImprintExtractionConfig

	ledgerRepo        ports.LedgerRepository
	costAlertHook     ports.CostAlertHook
	modelRoutingRules []entities.ModelRoutingRule

	spiritRepo         ports.SpiritRepository
	interactionLogRepo ports.ChronicleRepository

	memoryManager    MemoryManager
	messageManager   MessageManager
	oracleFactory    ports.OracleFactory
	reasoningService ReasoningExecutor
	actionExecutor   ActionExecutor
	validator        EventValidatorIface

	archivist           ports.Archivist
	archivistThreshold  int
	archivistKeepRecent int // 0 = derived as threshold/2

	distributedLock      ports.Bond
	idempotencyStore     ports.IdempotencyStore
	rateLimiter          ports.Limiter
	messageInbox         ports.Inbox
	asyncDispatcher      ports.Keeper
	scheduledTaskManager ports.RitualManager
	mediaStore           ports.Vault
	mediaProcessor       ports.Lens

	typingIndicator ports.TypingIndicator

	receptorMu        sync.RWMutex
	inboundConverters map[string]ports.Receptor
	configCache       *ConfigCache
	httpToolRepo      ports.HTTPToolRepository
	vigilRepo         ports.VigilRepository

	pipeline *Pipeline // built once in Build(), reused per Pulse

	config  *WeaveConfig
	appInfo AppInfo

	// closeCancel stops background goroutines (Subscribe, etc.) on Close().
	closeCancel context.CancelFunc
	closeOnce   sync.Once

	logger *zap.SugaredLogger
	tracer trace.Tracer
}

// Close stops all background goroutines started by Build() (e.g. the config-cache Subscribe
// loop) and releases resources. Calling Close multiple times is safe.
func (e *Weave) Close() error {
	e.closeOnce.Do(func() {
		if e.closeCancel != nil {
			e.closeCancel()
		}
	})
	return nil
}

// newWeaveWithConfig is private — use NewWeaveBuilder() instead.
func newWeaveWithConfig(
	scoutRegistry ports.ScoutRegistry,
	pathfinderRegistry ports.PathfinderRegistry,
	voiceRegistry ports.VoiceRegistry,
	spiritRepo ports.SpiritRepository,
	sessionRepo ports.MemoryRepository,
	messageRepo ports.EchoRepository,
	interactionLogRepo ports.ChronicleRepository,
	oracleFactory ports.OracleFactory,
	actionRegistry ports.ActionRegistry,
	distributedLock ports.Bond,
	rateLimiter ports.Limiter,
	messageInbox ports.Inbox,
	asyncDispatcher ports.Keeper,
	config *WeaveConfig,
	logger *zap.SugaredLogger,
	tracer trace.Tracer,
) (*Weave, error) {
	if config == nil {
		config = DefaultWeaveConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid Weave configuration: %w", err)
	}

	actionExecutor := NewActionExecutor(actionRegistry, config.ParallelActionExecution, logger, tracer).
		WithRetry(ActionRetryConfig{
			MaxAttempts: config.ActionRetryMaxAttempts,
			BaseDelay:   config.ActionRetryBaseDelay,
		})
	memoryManager := NewMemoryManager(sessionRepo, messageRepo, config.MaxMemoryMessages, logger, tracer).
		WithMemoryReconstruction(config.EnableMemoryReconstruction, config.MemoryReconstructionLimit)
	messageManager := NewMessageManager(messageRepo, logger, tracer)
	reasoningService := NewReasoningService(oracleFactory, actionExecutor, memoryManager, config.MaxReasoningIterations, config.MaxIterationsMessage, config.MaxActionsPerCycle, logger, tracer)
	reasoningService.SetToolResultLimits(config.ToolResultLimits)
	reasoningService.SetProgressPolicy(config.ProgressPolicy)
	reasoningService.SetCompressionPolicy(config.CompressionPolicy)
	reasoningService.SetReflectionPolicy(config.ReflectionPolicy)
	reasoningService.SetGroundingPolicy(config.GroundingPolicy)
	reasoningService.SetPlanPolicy(config.PlanPolicy)
	reasoningService.SetHandoffPolicy(config.HandoffPolicy)
	validator := NewEventValidator(config, logger, tracer)

	engine := &Weave{
		scoutRegistry:        scoutRegistry,
		pathfinderRegistry:   pathfinderRegistry,
		voiceRegistry:        voiceRegistry,
		spiritRepo:           spiritRepo,
		interactionLogRepo:   interactionLogRepo,
		memoryManager:        memoryManager,
		messageManager:       messageManager,
		oracleFactory:        oracleFactory,
		reasoningService:     reasoningService,
		actionExecutor:       actionExecutor,
		validator:            validator,
		distributedLock:      distributedLock,
		rateLimiter:          rateLimiter,
		messageInbox:         messageInbox,
		asyncDispatcher:      asyncDispatcher,
		scheduledTaskManager: nil, // wired by WeaveBuilder after construction
		mediaStore:           nil, // wired by WeaveBuilder after construction
		mediaProcessor:       nil, // wired by WeaveBuilder after construction
		config:               config,
		inboundConverters:    make(map[string]ports.Receptor),
		configCache:          NewConfigCache(nil, nil, logger),
		logger:               logger,
		tracer:               tracer,
	}

	engine.pipeline = engine.buildProcessingPipeline()

	return engine, nil
}

func (e *Weave) GetAppInfo() AppInfo {
	return e.appInfo
}

func (e *Weave) GetLLMFactory() ports.OracleFactory {
	return e.oracleFactory
}

func (e *Weave) GetPathfinderRegistry() ports.PathfinderRegistry {
	return e.pathfinderRegistry
}

func (e *Weave) GetScoutRegistry() ports.ScoutRegistry {
	return e.scoutRegistry
}

func (e *Weave) GetVoiceRegistry() ports.VoiceRegistry {
	return e.voiceRegistry
}

func (e *Weave) GetLogicRouterRegistry() ports.LogicRouterRegistry {
	return e.logicRouterRegistry
}

// GetActionRegistry returns the ActionRegistry so management handlers can list registered actions.
func (e *Weave) GetActionRegistry() ports.ActionRegistry {
	if accessor, ok := e.actionExecutor.(registryAccessor); ok {
		return accessor.GetRegistry()
	}
	return nil
}

// GetReceptorNames returns the names of all registered inbound converters (Receptors).
func (e *Weave) GetReceptorNames() []string {
	e.receptorMu.RLock()
	defer e.receptorMu.RUnlock()
	names := make([]string, 0, len(e.inboundConverters))
	for name := range e.inboundConverters {
		names = append(names, name)
	}
	return names
}

// GetAsyncDispatcher returns the task scheduler used for async webhook dispatch (if configured).
func (e *Weave) GetAsyncDispatcher() ports.Keeper {
	return e.asyncDispatcher
}

func (e *Weave) GetRitualManager() ports.RitualManager {
	return e.scheduledTaskManager
}

func (e *Weave) RegisterReceptor(name string, converter ports.Receptor) {
	e.receptorMu.Lock()
	defer e.receptorMu.Unlock()
	e.inboundConverters[name] = converter
}

func (e *Weave) IngestLore(ctx context.Context, ingestion entities.LoreIngestion) error {
	if e.loreRepository == nil || e.loreStore == nil || e.loreEmbedder == nil {
		return fmt.Errorf("lore ingestion not configured: call WithLoreRepository, WithLoreStore, and WithLoreEmbedder")
	}

	lore, err := e.loreRepository.GetByID(ctx, ingestion.LoreID)
	if err != nil {
		return fmt.Errorf("lore %q not found: %w", ingestion.LoreID, err)
	}

	text := ingestion.Text
	if text == "" && ingestion.FilePath != "" {
		data, err := os.ReadFile(ingestion.FilePath)
		if err != nil {
			return fmt.Errorf("read lore file: %w", err)
		}
		text = string(data)
	}
	if text == "" {
		return fmt.Errorf("lore ingestion requires Text or FilePath")
	}

	chunkSize := lore.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	overlap := lore.Overlap
	if overlap < 0 {
		overlap = 0
	}

	var rawChunks []string
	if ingestion.NoChunk {
		rawChunks = []string{text} // record mode: one vector for the whole document
	} else {
		rawChunks = chunking.Recursive(text, chunkSize, overlap)
	}

	embeddings, err := e.loreEmbedder.Embed(ctx, rawChunks)
	if err != nil {
		return fmt.Errorf("embed lore chunks: %w", err)
	}

	metadata := chunkMetadata(ingestion)
	now := time.Now()
	chunks := make([]entities.LoreChunk, len(rawChunks))
	for i, content := range rawChunks {
		chunks[i] = entities.LoreChunk{
			ID:        chunkID(ingestion, i, len(rawChunks)),
			LoreID:    ingestion.LoreID,
			Content:   content,
			Embedding: embeddings[i],
			Metadata:  metadata,
			CreatedAt: now,
		}
	}

	if err := e.loreStore.Upsert(ctx, chunks); err != nil {
		return fmt.Errorf("upsert lore chunks: %w", err)
	}
	return nil
}

// chunkID is the stable ID for chunk i. With a DocumentID, a single-chunk record uses it directly and
// multi-chunk records are suffixed; without one, IDs are positional within the Lore (legacy).
func chunkID(ingestion entities.LoreIngestion, i, total int) string {
	if ingestion.DocumentID != "" {
		if total == 1 {
			return ingestion.DocumentID
		}
		return fmt.Sprintf("%s_%d", ingestion.DocumentID, i)
	}
	return fmt.Sprintf("%s_%d", ingestion.LoreID, i)
}

// chunkMetadata returns the metadata stored on every chunk, adding the DocumentID under
// LoreDocumentIDKey (without mutating the caller's map) so a record's chunks stay grouped.
func chunkMetadata(ingestion entities.LoreIngestion) map[string]any {
	if ingestion.DocumentID == "" {
		return ingestion.Metadata
	}
	merged := make(map[string]any, len(ingestion.Metadata)+1)
	for k, v := range ingestion.Metadata {
		merged[k] = v
	}
	merged[entities.LoreDocumentIDKey] = ingestion.DocumentID
	return merged
}

func (e *Weave) DeleteLore(ctx context.Context, loreID string) error {
	if e.loreStore == nil {
		return fmt.Errorf("lore store not configured")
	}
	if err := e.loreStore.Delete(ctx, loreID); err != nil {
		return fmt.Errorf("delete lore: %w", err)
	}
	return nil
}

// SearchLore embeds queryText and returns the best-matching chunks of loreID — a direct, out-of-turn
// semantic query over indexed content, usable for retrieval, deduplication, recommendation, matching,
// or any similarity lookup. When the store supports metadata filtering (FilterableLoreStore) the
// opts.Filter is applied; otherwise it is ignored and a plain vector search runs. Results carry Score.
func (e *Weave) SearchLore(ctx context.Context, loreID, queryText string, opts ports.LoreSearchOptions) ([]entities.LoreChunk, error) {
	if e.loreStore == nil || e.loreEmbedder == nil {
		return nil, fmt.Errorf("lore search not configured: call WithLoreStore and WithLoreEmbedder")
	}

	embeddings, err := e.loreEmbedder.Embed(ctx, []string{queryText})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("embedder returned no vector for query")
	}
	queryVec := embeddings[0]

	if opts.TopK <= 0 {
		opts.TopK = 5
	}

	if filterable, ok := e.loreStore.(ports.FilterableLoreStore); ok {
		chunks, err := filterable.SearchFiltered(ctx, loreID, queryVec, opts)
		if err != nil {
			return nil, fmt.Errorf("filtered lore search: %w", err)
		}
		return chunks, nil
	}

	chunks, err := e.loreStore.Search(ctx, loreID, queryVec, opts.TopK, opts.MinScore)
	if err != nil {
		return nil, fmt.Errorf("lore search: %w", err)
	}
	return chunks, nil
}

func (e *Weave) RegisterAction(action ports.Action) error {
	if accessor, ok := e.actionExecutor.(registryAccessor); ok {
		registry := accessor.GetRegistry()
		if registry != nil {
			if err := registry.Register(action); err != nil {
				return fmt.Errorf("register action: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("action executor does not support dynamic registration")
}

func (e *Weave) wireOrchestration() {
	rs, ok := e.reasoningService.(*ReasoningService)
	if !ok {
		return
	}
	summonSvc := NewSummonService(e.spiritRepo, e.scoutRegistry, rs, e.logger)
	rs.SetSummonService(summonSvc)
}

func (e *Weave) RegisterEventConfiguration(config *entities.Link) {
	e.configCache.RegisterLink(config)
}

func (e *Weave) GetEventConfiguration(eventType string) *entities.Link {
	link, _ := e.configCache.GetLink(eventType)
	return link
}

// ConvertEventByType converts a raw webhook payload to Pulses using the Receptor configured for the event type.
// A single webhook may carry Pulses from multiple senders.
func (e *Weave) ConvertEventByType(ctx context.Context, eventType string, rawPayload map[string]any) ([]*entities.Pulse, error) {
	ctx, span := e.tracer.Start(ctx, "Weave/ConvertEventByType")
	defer span.End()

	log := e.logger

	eventConfig, exists := e.configCache.GetLink(eventType)
	if !exists {
		log.Warnw("event configuration not found", "event_type", eventType)
		return nil, fmt.Errorf("event configuration not found for type: %s", eventType)
	}

	converterName := eventConfig.InboundConverterName
	if converterName == "" {
		log.Warnw("no inbound converter specified in event configuration", "event_type", eventType)
		return nil, fmt.Errorf("no inbound converter specified for event type: %s", eventType)
	}

	e.receptorMu.RLock()
	converter, exists := e.inboundConverters[converterName]
	e.receptorMu.RUnlock()
	if !exists {
		log.Errorw("inbound converter not found", "converter_name", converterName, "event_type", eventType)
		return nil, fmt.Errorf("inbound converter not found: %s", converterName)
	}

	events, err := converter.Convert(ctx, eventType, rawPayload)
	if err != nil {
		log.Errorw("failed to convert event", "event_key", eventType, "converter", converterName, "error", err)
		return nil, fmt.Errorf("failed to convert event: %w", err)
	}

	log.Infow("events converted successfully",
		"event_key", eventType,
		"converter", converterName,
		"event_count", len(events),
	)
	return events, nil
}

// ProcessEventByKey processes a Pulse using the configuration registered for eventKey.
// End-user communication happens through Actions (e.g., send_whatsapp_message), not via the return value.
func (e *Weave) ProcessEventByKey(ctx context.Context, eventKey string, event *entities.Pulse) (*entities.Response, error) {
	ctx, span := e.tracer.Start(ctx, "Weave/ProcessEventByKey")
	defer span.End()

	eventConfig, exists := e.configCache.GetLink(eventKey)
	if !exists {
		return entities.NewErrorResponse(event.ID, event.MemoryKey, fmt.Sprintf("event configuration not found for key: %s", eventKey)), fmt.Errorf("event configuration not found for key: %s", eventKey)
	}

	return e.processEventWithConfig(ctx, event, eventKey, eventConfig)
}

const streamFrameBuffer = 32

// ProcessEventByKeyStream processes a single Pulse with the answer streamed as it is generated. Token
// deltas arrive as AgentStreamDelta, tool executions as AgentStreamToolStatus, and the turn ends with
// one AgentStreamDone (carrying the assembled answer) or AgentStreamError. The full pipeline still runs
// — persistence, audit, ledger included — on the assembled result; only live channel delivery is skipped.
func (e *Weave) ProcessEventByKeyStream(ctx context.Context, eventKey string, event *entities.Pulse) (<-chan ports.AgentStreamEvent, error) {
	eventConfig, exists := e.configCache.GetLink(eventKey)
	if !exists {
		return nil, fmt.Errorf("event configuration not found for key: %s", eventKey)
	}

	out := make(chan ports.AgentStreamEvent, streamFrameBuffer)
	go func() {
		defer close(out)

		emit := func(frame ports.AgentStreamEvent) {
			select {
			case out <- frame:
			case <-ctx.Done():
			}
		}

		state := &ProcessingState{
			Event:       event,
			EventKey:    eventKey,
			EventConfig: eventConfig,
			Streaming:   true,
		}
		state.streamSink = func(ev ReasoningEvent) {
			switch ev.Type {
			case ReasoningEventDelta:
				emit(ports.AgentStreamEvent{Type: ports.AgentStreamDelta, Delta: ev.Delta})
			case ReasoningEventToolStatus:
				emit(ports.AgentStreamEvent{Type: ports.AgentStreamToolStatus, ToolName: ev.ToolName})
			case ReasoningEventDone, ReasoningEventError:
				// Terminal events are surfaced after the pipeline assembles the final Response.
			}
		}

		resp, err := e.runProcessingPipeline(ctx, state)
		if err != nil {
			emit(ports.AgentStreamEvent{Type: ports.AgentStreamError, Err: err})
			return
		}

		final := ""
		if resp != nil {
			final = resp.FinalResponse
		}
		emit(ports.AgentStreamEvent{Type: ports.AgentStreamDone, Response: final})
	}()

	return out, nil
}

// ProcessMultipleEventsByKey processes each Pulse independently in its own goroutine.
// Results are returned in the same order as the input slice.
func (e *Weave) ProcessMultipleEventsByKey(ctx context.Context, eventKey string, events []*entities.Pulse) ([]*entities.Response, error) {
	ctx, span := e.tracer.Start(ctx, "Weave/ProcessMultipleEventsByKey")
	defer span.End()

	log := e.logger
	log.Infow("processing multiple events asynchronously",
		"event_key", eventKey,
		"event_count", len(events),
	)

	eventConfig, exists := e.configCache.GetLink(eventKey)
	if !exists {
		return nil, fmt.Errorf("event configuration not found for key: %s", eventKey)
	}

	results := make([]*entities.Response, len(events))

	maxConcurrent := e.config.MaxConcurrentEvents
	if maxConcurrent == 0 {
		maxConcurrent = 100
	}

	var wg sync.WaitGroup
	var sem chan struct{}
	if maxConcurrent > 0 {
		sem = make(chan struct{}, maxConcurrent)
	}

	for i, event := range events {
		wg.Add(1)
		go func(index int, evt *entities.Pulse) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			result, err := e.processEventWithConfig(ctx, evt, eventKey, eventConfig)
			if err != nil {
				log.Errorw("failed to process event in batch",
					"event_id", evt.ID,
					"index", index,
					"error", err,
				)
				results[index] = entities.NewErrorResponse(evt.ID, evt.MemoryKey, err.Error())
			} else {
				results[index] = result
			}
		}(i, event)
	}

	wg.Wait()

	log.Infow("finished processing multiple events",
		"event_key", eventKey,
		"event_count", len(events),
		"results_count", len(results),
	)

	return results, nil
}

func (e *Weave) processEventWithConfig(ctx context.Context, event *entities.Pulse, eventKey string, eventConfig *entities.Link) (*entities.Response, error) {
	ctx, span := e.tracer.Start(ctx, "Weave/processEventWithConfig")
	defer span.End()

	state := &ProcessingState{
		Event:       event,
		EventKey:    eventKey,
		EventConfig: eventConfig,
	}

	return e.runProcessingPipeline(ctx, state)
}

// runProcessingPipeline executes the full pipeline for a prebuilt state and assembles the Response.
// Both the buffered (processEventWithConfig) and streaming (ProcessEventByKeyStream) drivers share it
// so lock release, idempotency handling, audit logging, and result assembly never drift.
func (e *Weave) runProcessingPipeline(ctx context.Context, state *ProcessingState) (*entities.Response, error) {
	event := state.Event
	eventKey := state.EventKey

	log := e.logger
	log.Infow("processing event with config",
		"event_id", event.ID,
		"event_key", eventKey,
		"memory_key", event.MemoryKey,
	)

	pipelineErr := e.pipeline.Execute(ctx, state)

	if state.LockAcquired {
		// Fresh context: the caller ctx may be cancelled when the pipeline finishes.
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		releaseStep := NewLockReleaseStep(e.distributedLock, 5*time.Second, e.logger)
		if err := releaseStep.Execute(releaseCtx, state); err != nil {
			log.Errorw("failed to release lock in cleanup", "error", err)
		}
	}

	if pipelineErr != nil {
		state.ProcessingStatus = statusFromError(pipelineErr)
		_ = e.logInteraction(ctx, state)

		// A duplicate is an idempotent no-op, not a failure: report success with nil error so
		// callers and Cloud Tasks acknowledge the delivery without retrying.
		if IsDuplicateEvent(pipelineErr) {
			log.Infow("duplicate event ignored", "event_id", event.ID, "memory_key", event.MemoryKey)
			return entities.NewDuplicateResponse(event.ID, event.MemoryKey), nil
		}

		log.Errorw("pipeline execution failed", "error", pipelineErr)

		errResult := entities.NewErrorResponse(event.ID, event.MemoryKey,
			fmt.Sprintf("processing failed: %v", pipelineErr))
		// Preserve ResponseDelivered even on error: the user already received a message,
		// so Pub/Sub must ACK regardless of what failed afterwards.
		errResult.ResponseDelivered = state.ResponseDelivered
		return errResult, pipelineErr
	}

	processingTime := time.Since(state.StartTime).Milliseconds()

	actionNames := []string{}
	if state.ReasoningResult != nil {
		for _, iteration := range state.ReasoningResult.Iterations {
			for _, call := range iteration.ActionCalls {
				if call.ActionName != "" {
					actionNames = append(actionNames, call.ActionName)
				}
			}
		}
	}

	result := entities.NewResponse(
		event.ID,
		event.MemoryKey,
		state.Spirit.Name,
		actionNames,
	)
	result.ProcessingTimeMs = processingTime
	result.ResponseDelivered = state.ResponseDelivered
	result.FinalResponse = state.Response

	log.Infow("event processed successfully",
		"event_id", event.ID,
		"memory_key", event.MemoryKey,
		"spirit", state.Spirit.Name,
		"actions_executed", len(actionNames),
		"iterations_used", state.ReasoningResult.IterationsUsed,
		"processing_time_ms", processingTime,
	)

	return result, nil
}

func (e *Weave) buildProcessingPipeline() *Pipeline {
	pipeline := NewPipeline(e.logger, e.tracer)

	// --- Validation & rate limiting
	pipeline.
		AddStep(NewValidationStep(e.validator, e.config.LockAcquireTimeout, e.logger))

	if e.idempotencyStore != nil {
		pipeline.AddStep(NewIdempotencyStep(e.idempotencyStore, e.config.IdempotencyTTL, e.config.LockAcquireTimeout, e.logger))
	}

	if e.rateLimiter != nil {
		pipeline.AddStep(NewRateLimitStep(e.rateLimiter, e.config.LockAcquireTimeout, e.logger))
	}

	// --- Distributed lock acquisition
	pipeline.
		AddStep(NewLockAcquisitionStep(e.distributedLock, e.messageInbox, e.config.LockTTL, e.config.LockAcquireTimeout, e.logger))

	// --- Human takeover & typing indicator
	pipeline.AddStep(NewVigilCheckStep(e.vigilRepo, e.logger))

	if e.typingIndicator != nil {
		pipeline.AddStep(NewTypingStartStep(e.typingIndicator, e.logger))
	}

	// --- Scheduled task marking
	if e.scheduledTaskManager != nil {
		pipeline.AddStep(NewRitualMarkStep(e.scheduledTaskManager, ritualStepTimeout, e.logger))
	}

	// --- Scout enrichment & guard filtering
	pipeline.
		AddStep(NewLinkScoutStep(e.scoutRegistry, e.distributedLock, e.config.LockTTL, e.config.ScoutTimeout, e.logger))

	pipeline.AddStep(NewFilterStep(e.config.LockAcquireTimeout, e.logger))

	// --- Spirit selection & loading
	pipeline.
		AddStep(NewSpiritSelectionStep(e, e.config.SpiritSelectionTimeout, e.logger)).
		AddStep(NewSpiritLoadStep(e.spiritRepo, e.config.SpiritLoadTimeout, e.logger))

	if e.httpToolRepo != nil {
		if accessor, ok := e.actionExecutor.(registryAccessor); ok {
			if registry := accessor.GetRegistry(); registry != nil {
				pipeline.AddStep(NewHTTPToolLoadStep(e.httpToolRepo, registry, e.config.ScoutTimeout, e.logger))
			}
		}
	}

	pipeline.
		AddStep(NewSpiritScoutStep(e.scoutRegistry, e.loreHarvester, e.imprintHarvester, e.config.ScoutTimeout, e.logger)).
		AddStep(NewOrchestratorRoutingStep(e.logicRouterRegistry, e.spiritRepo, e.config.SpiritSelectionTimeout, e.logger))

	// --- Media processing
	if e.mediaStore != nil {
		pipeline.AddStep(NewMediaVaultStep(e.mediaStore, 15*time.Second, e.logger))
	}

	if e.mediaProcessor != nil {
		pipeline.AddStep(NewMediaProcessingStep(e.mediaProcessor, 30*time.Second))
	}

	// --- Session & memory setup
	pipeline.AddStep(NewSessionSetupStep(e.memoryManager, e.config.SessionTimeout, e.logger))

	if e.archivist != nil {
		pipeline.AddStep(NewArchivistStep(e.archivist, e.archivistThreshold, e.archivistKeepRecent, e.config.SessionTimeout, e.logger))
	}

	// --- Message coalescing (drains inbox, adds UserMessage to session memory)
	// Always present — inbox may be nil (coalescing disabled, memory addition still happens).
	pipeline.AddStep(NewMessageCoalescingStep(e.memoryManager, e.messageInbox, e.config.InboxMinWindow, e.config.MessageCoalescingTimeout, e.logger))

	// --- Cost enforcement & model routing
	if e.ledgerRepo != nil {
		pipeline.AddStep(NewCostEnforcementStep(e.ledgerRepo, e.costAlertHook, e.config.ScoutTimeout, e.logger))
	}

	if len(e.modelRoutingRules) > 0 {
		pipeline.AddStep(NewModelRoutingStep(e.modelRoutingRules, e.config.ScoutTimeout, e.logger))
	}

	// --- Reasoning & response delivery
	pipeline.
		AddStep(NewConditionEvaluationStep(e.config.ScoutTimeout, e.logger)).
		AddStep(NewReasoningStep(e.reasoningService, e.config.ReasoningTimeout, e.distributedLock, e.config.LockTTL, e.logger)).
		AddStep(NewNotificationStep(e.voiceRegistry, e.GetLLMFactory(), e.config.ReasoningTimeout, e.logger)).
		AddStep(NewPersistenceStep(e.memoryManager, e.messageManager, e.config.PersistenceTimeout, e.logger)).
		AddStep(NewResponseDeliveryStep(e.voiceRegistry, e.config.PersistenceTimeout, e.logger)).
		AddStep(NewAuditLogStep(e, e.config.PersistenceTimeout, e.logger))

	// --- Post-response async steps
	if e.imprintRepository != nil {
		pipeline.AddStep(NewImprintExtractionStep(
			e.imprintRepository,
			e.GetLLMFactory(),
			e.imprintExtractionCfg,
			e.config.PersistenceTimeout,
			e.logger,
		))
	}

	if e.ledgerRepo != nil {
		pipeline.AddStep(NewLedgerUpdateStep(e.ledgerRepo, e.config.PersistenceTimeout, e.logger))
	}

	if e.scheduledTaskManager != nil {
		pipeline.AddStep(NewMarkExecutedStep(e.scheduledTaskManager, ritualStepTimeout, e.logger))
	}

	// --- Deferred cleanup (always runs, even on error)
	if e.typingIndicator != nil {
		pipeline.AddDeferredStep(NewTypingStopStep(e.typingIndicator, e.logger))
	}

	return pipeline
}

func (e *Weave) selectSpirit(ctx context.Context, event *entities.Pulse, eventConfig *entities.Link) string {
	ctx, span := e.tracer.Start(ctx, "Weave/SelectSpirit")
	defer span.End()

	log := e.logger
	availableSpirits := eventConfig.AllowedSpirits
	defaultSpirit := eventConfig.DefaultSpirit
	pathfinderName := eventConfig.PathfinderName

	if len(availableSpirits) == 1 {
		log.Infow("single Spirit configured, using directly",
			"spirit", availableSpirits[0],
			"memory_key", event.MemoryKey)
		return availableSpirits[0]
	}

	if len(availableSpirits) == 0 {
		if defaultSpirit != "" {
			log.Infow("no Spirits configured, using default",
				"default_spirit", defaultSpirit,
				"memory_key", event.MemoryKey)
			return defaultSpirit
		}
		log.Warnw("no Spirits configured and no default Spirit", "memory_key", event.MemoryKey)
		return "DEFAULT"
	}

	var pathfinder ports.Pathfinder

	// 1. Prefer the Pathfinder named in the event configuration.
	if pathfinderName != "" && e.pathfinderRegistry != nil {
		pathfinder = e.pathfinderRegistry.Get(pathfinderName)
		if pathfinder == nil {
			log.Warnw("specified Pathfinder not found in registry",
				"pathfinder_name", pathfinderName,
				"memory_key", event.MemoryKey)
		}
	}

	// 2. Fall back to the default LLM Pathfinder when registered.
	if pathfinder == nil && e.pathfinderRegistry != nil {
		pathfinder = e.pathfinderRegistry.Get(pathfinders.DefaultPathfinderName)
		if pathfinder != nil {
			log.Infow("using default LLM Pathfinder for Spirit selection",
				"available_spirits", len(availableSpirits),
				"memory_key", event.MemoryKey)
		}
	}

	// 3. No Pathfinder available — use configured default or first Spirit.
	if pathfinder == nil {
		if defaultSpirit != "" {
			log.Infow("no Pathfinder available, using default Spirit",
				"default_spirit", defaultSpirit,
				"memory_key", event.MemoryKey)
			return defaultSpirit
		}
		log.Infow("no Pathfinder available, using first available Spirit",
			"spirit", availableSpirits[0],
			"memory_key", event.MemoryKey)
		return availableSpirits[0]
	}

	log.Infow("using Pathfinder for Spirit selection",
		"pathfinder_name", pathfinder.GetName(),
		"available_spirits", len(availableSpirits),
		"memory_key", event.MemoryKey)

	selectedSpirit := pathfinder.SelectSpirit(ctx, event, availableSpirits)

	if selectedSpirit == "" {
		log.Warnw("Pathfinder returned empty, using fallback",
			"pathfinder_name", pathfinder.GetName(),
			"memory_key", event.MemoryKey)

		if defaultSpirit != "" {
			return defaultSpirit
		}
		if len(availableSpirits) > 0 {
			return availableSpirits[0]
		}
	}

	return selectedSpirit
}

// defaultBusinessErrorInstructions is injected into every Spirit that has Actions configured.
// It gives the LLM explicit guidance on how to handle business errors returned by tools,
// ensuring the user always receives a meaningful, friendly response instead of raw error text.
// Spirits can override this via Spirit.BusinessErrorInstructions.
const defaultBusinessErrorInstructions = `--- Business Error Handling ---
When a tool returns a business error during execution:
- Read the error message to understand the business constraint or data issue that occurred
- Communicate the situation to the user in clear, friendly language — never expose tool names, technical identifiers, or raw error text verbatim
- If the error implies a corrective action the user can take (e.g., wrong identifier, missing information), guide them on what to do next
- Do not retry the same tool with the same arguments after receiving a business error
- If no alternative path exists, acknowledge the situation honestly and offer what assistance you can`

// buildSystemPrompt assembles the final prompt by appending operational blocks after the Spirit's
// base prompt: delivery enforcement, business error guidance, then event context.
func buildSystemPrompt(spirit *entities.Spirit, event *entities.Pulse) string {
	prompt := spirit.SystemPrompt
	prompt += responseDeliverySection(spirit)
	prompt += businessErrorSection(spirit)
	prompt += eventContextSection(event)
	return prompt
}

func responseDeliverySection(spirit *entities.Spirit) string {
	if !spirit.EnforceVoiceDelivery {
		return ""
	}
	instructions := spirit.VoiceDeliveryInstructions
	if instructions == "" {
		instructions = `
CRITICAL INSTRUCTION: You MUST send a response to the user by calling a response delivery tool.
If you do not call send_whatsapp_message, send_whatsapp_template, or equivalent tool, the user will NOT receive your response.
Always conclude conversations by explicitly calling the appropriate send tool with your final message.`
	}
	return "\n\n" + instructions
}

// businessErrorSection is omitted for Action-less Spirits — no Actions means no business errors.
func businessErrorSection(spirit *entities.Spirit) string {
	if len(spirit.AllowedActions) == 0 {
		return ""
	}
	instructions := spirit.BusinessErrorInstructions
	if instructions == "" {
		instructions = defaultBusinessErrorInstructions
	}
	return "\n\n" + instructions
}

func eventContextSection(event *entities.Pulse) string {
	section := "\n\n--- Current Event Information ---"
	section += fmt.Sprintf("\nEvent Type: %s", event.EventType)
	section += fmt.Sprintf("\nSource: %s", event.Source)

	if event.SubType != "" {
		section += fmt.Sprintf("\nSub Type: %s", event.SubType)
	}

	if len(event.Knowledge) > 0 {
		dataJSON, _ := json.MarshalIndent(event.Knowledge, "", "  ")
		section += fmt.Sprintf("\n\n--- Knowledge ---\n%s", string(dataJSON))
	}

	return section
}

func buildMessageHistory(session *entities.Memory) []ports.OracleMessage {
	messages := make([]ports.OracleMessage, len(session.Threads))
	for i, msg := range session.Threads {
		messages[i] = ports.OracleMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			ImageURLs: msg.ImageURLs,
			AudioURLs: msg.AudioURLs,
		}
	}
	return messages
}

func (e *Weave) logInteraction(
	ctx context.Context,
	state *ProcessingState,
) error {
	ctx, span := e.tracer.Start(ctx, "Weave/LogInteraction")
	defer span.End()

	processingTimeMs := time.Since(state.StartTime).Milliseconds()

	var scoutsUsed []string
	if state.EventConfig != nil && len(state.EventConfig.RequireScouts) > 0 {
		scoutsUsed = state.EventConfig.RequireScouts
	}

	var reasoning ports.OracleUsage
	var iterations []entities.IterationLog
	iterationsUsed := 0
	if state.ReasoningResult != nil {
		reasoning = state.ReasoningResult.TokensUsed
		iterations = state.ReasoningResult.Iterations
		iterationsUsed = state.ReasoningResult.IterationsUsed
	}
	media := state.MediaTokensUsed
	tokenUsage := entities.InteractionTokenUsage{
		Reasoning: entities.TokenUsageBreakdown{
			PromptTokens:     reasoning.PromptTokens,
			CompletionTokens: reasoning.CompletionTokens,
			TotalTokens:      reasoning.TotalTokens,
		},
		Media: entities.TokenUsageBreakdown{
			PromptTokens:     media.PromptTokens,
			CompletionTokens: media.CompletionTokens,
			TotalTokens:      media.TotalTokens,
		},
		Total: entities.TokenUsageBreakdown{
			PromptTokens:     reasoning.PromptTokens + media.PromptTokens,
			CompletionTokens: reasoning.CompletionTokens + media.CompletionTokens,
			TotalTokens:      reasoning.TotalTokens + media.TotalTokens,
		},
	}

	status := state.ProcessingStatus
	if status == "" {
		if state.FinalError != "" {
			status = "error"
		} else {
			status = "success"
		}
	}

	var topicKey string
	if state.Session != nil {
		topicKey = state.Session.SubjectKey
	}

	pathfinderUsed := state.PathfinderUsed
	if pathfinderUsed == "" && state.EventConfig != nil {
		pathfinderUsed = state.EventConfig.PathfinderName
	}

	spiritLog := entities.InteractionSpiritLog{}
	if state.Spirit != nil {
		spiritLog = entities.InteractionSpiritLog{
			ID:          state.Spirit.ID,
			Name:        state.Spirit.Name,
			Version:     fmt.Sprintf("%d", state.Spirit.Version),
			Provider:    state.Spirit.ModelConfig.Provider,
			Model:       state.Spirit.ModelConfig.Model,
			Temperature: state.Spirit.ModelConfig.Temperature,
		}
	}

	responseChannel := ""
	if state.EventConfig != nil {
		responseChannel = state.EventConfig.VoiceName
	}

	interactionLog := &entities.Chronicle{
		MemoryKey: state.Event.MemoryKey,
		EventKey:  state.EventKey,
		Timestamp: helpers.NowUTC(),

		Event: entities.InteractionEventLog{
			ID:             state.Event.ID,
			Type:           state.Event.EventType,
			Source:         state.Event.Source,
			SubType:        state.Event.SubType,
			IdempotencyKey: state.Event.IdempotencyKey,
			ContactPhone:   state.Event.ContactPhone,
			SubjectKey:     topicKey,
			UserInput:      state.Event.UserMessage,
			Attachments:    buildAttachmentLogs(state.Event.Attachments),
			Knowledge:      state.Event.Knowledge,
			ScoutsUsed:     scoutsUsed,
			Payload:        state.Event.Payload,
			Metadata:       state.Event.Metadata,
		},

		Spirit: spiritLog,

		Processing: entities.InteractionProcessingLog{
			Status:                 status,
			ProcessingTimeMs:       processingTimeMs,
			PipelineFailedAtStep:   state.PipelineFailedAtStep,
			PathfinderUsed:         pathfinderUsed,
			ArchivistApplied:       state.ArchivistApplied,
			SessionMessagesCount:   state.SessionMessagesCount,
			CoalescedMessagesCount: state.CoalescedMessagesCount,
			Iterations:             iterations,
			IterationsUsed:         iterationsUsed,
			FinalResponse:          state.Response,
			FinalError:             state.FinalError,
			ExecutionErrors:        state.ExecutionErrors,
			ResponseDelivered:      state.ResponseDelivered,
			ResponseChannel:        responseChannel,
		},

		TokenUsage: tokenUsage,
	}

	if err := e.interactionLogRepo.LogInteraction(ctx, interactionLog); err != nil {
		return fmt.Errorf("log interaction: %w", err)
	}
	return nil
}

// statusFromError maps OrchestrationError codes to audit-log status strings.
func statusFromError(err error) string {
	var oe *OrchestrationError
	if !errors.As(err, &oe) {
		return "error"
	}
	switch oe.Code {
	case "RATE_LIMIT_EXCEEDED":
		return "rate_limited"
	case "VALIDATION_FAILED", "PROMPT_INJECTION_DETECTED":
		return "validation_failed"
	case "DUPLICATE_EVENT":
		return "duplicate"
	case "MEMORY_BUSY":
		return "session_busy"
	case "SESSION_HELD":
		return "session_held"
	case "LOCK_ACQUISITION_FAILED":
		return "lock_acquisition_failed"
	case "ENRICHMENT_FAILED":
		return "enrichment_failed"
	case "SPIRIT_NOT_FOUND":
		return "spirit_not_found"
	case "SESSION_OPERATION_FAILED":
		return "session_operation_failed"
	case "REASONING_FAILED":
		return "reasoning_failed"
	case "PERSISTENCE_FAILED":
		return "persistence_failed"
	case "TIMEOUT_EXCEEDED":
		return "timeout_exceeded"
	case "TOOL_BUDGET_EXCEEDED":
		return "tool_budget_exceeded"
	case "FILTER_BLOCKED":
		return "filter_blocked"
	case "FILTER_NOT_ALLOWED":
		return "filter_not_allowed"
	default:
		return "error"
	}
}

func buildAttachmentLogs(attachments []*entities.Artifact) []entities.AttachmentLog {
	if len(attachments) == 0 {
		return nil
	}
	logs := make([]entities.AttachmentLog, 0, len(attachments))
	for _, att := range attachments {
		logs = append(logs, entities.AttachmentLog{
			Type:       att.Type,
			MediaID:    att.MediaID,
			MimeType:   att.MimeType,
			StorageURL: att.URL,
			Downloaded: len(att.Data) > 0,
		})
	}
	return logs
}
