package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/implementation/media"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type ReasoningRequest struct {
	Event        *entities.Pulse
	Spirit       *entities.Spirit
	Session      *entities.Memory
	SystemPrompt string

	// ConversationContext is the clean conversation history (User + Assistant final messages).
	// Persisted in Threads and excludes Action calls/results.
	ConversationContext []ports.OracleMessage
}

type ReasoningResult struct {
	Iterations      []entities.IterationLog
	FinalResponse   string
	FinalError      string
	ExecutionErrors []string

	// WorkingContext contains ALL messages including Action calls/results.
	// Used only for IterationLogs audit, NOT persisted to Threads.
	WorkingContext []ports.OracleMessage

	IterationsUsed int
	TokensUsed     ports.OracleUsage

	// TokensByModel attributes token usage per model when model tiering is enabled; the aggregate
	// remains in TokensUsed. Empty when tiering is off.
	TokensByModel map[string]ports.OracleUsage

	// FinalSession is the session at the end of the reasoning loop.
	// May differ from the input session when an Action triggered a topic switch mid-loop.
	// PersistenceStep must use this value — not the original state.Session.
	FinalSession *entities.Memory

	// ResponseDelivered is true when a VoiceDelivery-category Action succeeded.
	// When set, the interaction is complete and Pub/Sub must ACK even if a later pipeline step fails.
	ResponseDelivered bool

	// Citations holds the validated Lore chunk IDs the final answer referenced, when grounding is enabled.
	Citations []string

	// Plan is the final state of the agent's plan/scratchpad, when the plan policy is enabled.
	Plan []entities.PlanItem
}

func (r *ReasoningResult) accumulateTokens(usage ports.OracleUsage) {
	r.TokensUsed.PromptTokens += usage.PromptTokens
	r.TokensUsed.CompletionTokens += usage.CompletionTokens
	r.TokensUsed.TotalTokens += usage.TotalTokens
}

// accumulateModelTokens adds usage to both the aggregate and the per-model breakdown.
func (r *ReasoningResult) accumulateModelTokens(model string, usage ports.OracleUsage) {
	r.accumulateTokens(usage)
	if r.TokensByModel == nil {
		r.TokensByModel = make(map[string]ports.OracleUsage)
	}
	cur := r.TokensByModel[model]
	cur.PromptTokens += usage.PromptTokens
	cur.CompletionTokens += usage.CompletionTokens
	cur.TotalTokens += usage.TotalTokens
	r.TokensByModel[model] = cur
}

// Closing hints are appended ephemerally to the system prompt — never stored in conversation history.
// closingInstruction: used after a critical business error; the LLM has the domain error in context.
// infraClosingInstruction: used after a critical infra error on a conversational Spirit; details are withheld.
const closingInstruction = "\n\n--- CLOSING INSTRUCTION ---\n" +
	"A terminal error has occurred in this conversation turn. You must now\n" +
	"communicate the result to the user and end the conversation.\n" +
	"Do not attempt any further actions."

const infraClosingInstruction = "\n\n--- CLOSING INSTRUCTION ---\n" +
	"A technical error prevented this operation from completing. Apologize to the\n" +
	"user and inform them that the service is temporarily unavailable.\n" +
	"Do not attempt any further actions."

// ReasoningService drives the multi-turn LLM loop: it calls the model,
// executes any Action calls it requests, and repeats until a stop condition is
// met or the iteration cap is reached.
//
// When a MemoryManager is provided, the service detects topic switches between
// iterations and reloads Threads for the new topic before the next LLM
// call, ensuring the LLM has full history from the very next iteration.
type ReasoningService struct {
	oracleFactory        ports.OracleFactory
	actionExecutor       ActionExecutor
	memoryManager        MemoryManager  // optional; enables topic-switch memory reload
	summonService        *SummonService // optional; enables summon_spirit for orchestrators
	maxIterations        int
	maxIterationsMessage string
	maxActionsPerCycle   int                    // 0 = unlimited
	toolResultLimits     ports.ToolResultLimits // zero MaxChars = shaping disabled
	progressPolicy       ProgressPolicy         // disabled by default
	compressionPolicy    CompressionPolicy      // disabled by default
	reflectionPolicy     ReflectionPolicy       // disabled by default
	groundingPolicy      GroundingPolicy        // disabled by default
	planPolicy           PlanPolicy             // disabled by default
	logger               *zap.SugaredLogger
	tracer               trace.Tracer
}

// NewReasoningService constructs a ReasoningService.
// memoryManager is optional — pass nil to disable topic-switch reload.
// maxIterationsMessage is the FinalResponse when the loop exits via the iteration cap.
// maxActionsPerCycle caps total Action calls across all iterations; 0 disables the cap.
func NewReasoningService(
	oracleFactory ports.OracleFactory,
	actionExecutor ActionExecutor,
	memoryManager MemoryManager,
	maxIterations int,
	maxIterationsMessage string,
	maxActionsPerCycle int,
	logger *zap.SugaredLogger,
	tracer trace.Tracer,
) *ReasoningService {
	return &ReasoningService{
		oracleFactory:        oracleFactory,
		actionExecutor:       actionExecutor,
		memoryManager:        memoryManager,
		maxIterations:        maxIterations,
		maxIterationsMessage: maxIterationsMessage,
		maxActionsPerCycle:   maxActionsPerCycle,
		logger:               logger,
		tracer:               tracer,
	}
}

func (r *ReasoningService) SetSummonService(s *SummonService) {
	r.summonService = s
}

// SetToolResultLimits configures the default bound applied to each Action result before it enters
// the reasoning context. A zero MaxChars (the default) disables shaping.
func (r *ReasoningService) SetToolResultLimits(limits ports.ToolResultLimits) {
	r.toolResultLimits = limits
}

// SetProgressPolicy enables stall detection and forced final synthesis. Disabled by default.
func (r *ReasoningService) SetProgressPolicy(policy ProgressPolicy) {
	r.progressPolicy = policy
}

// SetCompressionPolicy enables in-loop working-context compression. Disabled by default.
func (r *ReasoningService) SetCompressionPolicy(policy CompressionPolicy) {
	r.compressionPolicy = policy
}

// SetReflectionPolicy enables a self-critique pass before delivering a draft answer. Disabled by default.
func (r *ReasoningService) SetReflectionPolicy(policy ReflectionPolicy) {
	r.reflectionPolicy = policy
}

// SetGroundingPolicy enables source-citation enforcement for RAG answers. Disabled by default.
func (r *ReasoningService) SetGroundingPolicy(policy GroundingPolicy) {
	r.groundingPolicy = policy
}

// SetPlanPolicy enables the turn-scoped plan/scratchpad maintained via update_plan. Disabled by default.
func (r *ReasoningService) SetPlanPolicy(policy PlanPolicy) {
	r.planPolicy = policy
}

// synthesizeFinal makes one tools-stripped LLM call to produce a closing answer when the loop
// stalls or hits the iteration cap. Tokens are accounted into result; on error it falls back to
// the configured max-iterations message.
// synthesizeFinal makes one tools-stripped call on the primary model to produce a closing answer
// when the loop stalls or hits the iteration cap. The primary is always the strong tier, so no
// escalation flag is needed. On error it falls back to the configured max-iterations message.
func (r *ReasoningService) synthesizeFinal(ctx context.Context, provider ports.Oracle, req *ReasoningRequest, workingContext []ports.OracleMessage, result *ReasoningResult) string {
	resp, err := r.callLLM(ctx, provider, req, workingContext, nil, stallSynthesisInstruction, "")
	if err != nil {
		r.logger.Warnw("forced synthesis call failed, using fallback message", "error", err)
		return r.maxIterationsMessage
	}
	r.accrueTokens(req, result, "", resp.TokensUsed)
	return resp.Content
}

// accrueTokens records usage on the aggregate, plus a per-model bucket when tiering is active.
func (r *ReasoningService) accrueTokens(req *ReasoningRequest, result *ReasoningResult, model string, usage ports.OracleUsage) {
	if !r.tieringActive(req) {
		result.accumulateTokens(usage)
		return
	}
	if model == "" {
		model = req.Spirit.ModelConfig.Model
	}
	result.accumulateModelTokens(model, usage)
}

// limitsFor resolves the result limits for an Action: a per-Action ToolResultShaper override when
// implemented, otherwise the service default.
func (r *ReasoningService) limitsFor(actionName string) ports.ToolResultLimits {
	if action, err := r.actionExecutor.GetAction(actionName); err == nil {
		if shaper, ok := action.(ports.ToolResultShaper); ok {
			return shaper.ResultLimit()
		}
	}
	return r.toolResultLimits
}

func (r *ReasoningService) Execute(ctx context.Context, req *ReasoningRequest) (*ReasoningResult, error) {
	ctx, span := r.tracer.Start(ctx, "ReasoningService/Execute")
	defer span.End()

	provider, err := r.oracleFactory.GetProvider(req.Spirit.ModelConfig.Provider)
	if err != nil {
		return nil, fmt.Errorf("get LLM provider: %w", err)
	}

	actions := r.availableActions(req)
	workingContext := r.initializeWorkingContext(req)

	conversationOffset := len(workingContext)
	if req.Event.UserMessage != "" {
		conversationOffset = len(workingContext) - 1
	}

	activeTopic := sessionTopicKey(req.Session)
	enforceVoiceDelivery := req.Spirit.EnforceVoiceDelivery

	// bannedActions are scoped to this turn — critical failures of the same Action must not block unrelated Actions next turn.
	bannedActions := make(map[string]bool)
	infraTerminal := false
	totalActionCalls := 0

	// Iteration boundaries (working-context length after each iteration) drive context compression
	// at safe points; topicSwitchedThisTurn disables compression for the turn to avoid carrying an
	// old-topic ledger across a topic change.
	iterBoundaries := []int{}
	topicSwitchedThisTurn := false
	reflectionRounds := 0
	groundingRevised := false
	planNudged := false

	// When grounding is enabled and Lore was retrieved, instruct the model to cite sources.
	// Ephemeral for the turn — req is turn-scoped.
	if r.groundingPolicy.Enabled && retrievedLoreContext(req) != "" {
		req.SystemPrompt += groundingAddendum
	}

	// plan is nil unless the plan policy is enabled; the model maintains it via update_plan.
	var plan *planState
	if r.planPolicy.Enabled {
		plan = newPlanState(r.planPolicy.MaxItems)
		if r.planPolicy.Required {
			req.SystemPrompt += planRequiredInstruction
		}
	}

	stallWindow := 0
	if r.progressPolicy.Enabled {
		stallWindow = r.progressPolicy.StallWindow
	}
	stall := newStallTracker(stallWindow)

	// With model tiering, tool-using iterations run on the cheaper draft model; the terminal answer
	// is re-synthesized on the strong model below. draftModel is "" (primary) when tiering is off.
	draftModel := ""
	if r.tieringActive(req) {
		draftModel = r.tierDraftModel(req)
	}

	result := &ReasoningResult{
		Iterations:      []entities.IterationLog{},
		ExecutionErrors: []string{},
		WorkingContext:  workingContext,
		FinalSession:    req.Session,
	}

	appendIter := func(log *entities.IterationLog, start time.Time) {
		log.DurationMs = time.Since(start).Milliseconds()
		result.Iterations = append(result.Iterations, *log)
	}

	for iteration := range r.maxIterations {
		result.IterationsUsed = iteration + 1

		if err := ctx.Err(); err != nil {
			result.FinalError = fmt.Sprintf("context cancelled at iteration %d", result.IterationsUsed)
			result.FinalSession = req.Session
			return result, fmt.Errorf("context cancelled at iteration %d: %w", result.IterationsUsed, err)
		}

		r.logger.Infow("reasoning iteration",
			"iteration", result.IterationsUsed,
			"memory_key", req.Event.MemoryKey,
		)

		iterStart := time.Now()
		iterLog := entities.IterationLog{
			Iteration: result.IterationsUsed,
			Errors:    []string{},
		}

		activeActions, closingHint := resolveIterationActions(actions, bannedActions, infraTerminal)
		if plan != nil {
			// The current plan is injected ephemerally each iteration — always up to date, never persisted.
			closingHint = plan.render() + closingHint
		}
		llmResp, err := r.callLLM(ctx, provider, req, workingContext, activeActions, closingHint, draftModel)
		if err != nil {
			iterLog.Errors = append(iterLog.Errors, fmt.Sprintf("LLM call failed: %v", err))
			appendIter(&iterLog, iterStart)
			result.FinalError = fmt.Sprintf("LLM call failed at iteration %d", result.IterationsUsed)
			result.FinalSession = req.Session
			return result, fmt.Errorf("LLM call at iteration %d: %w", result.IterationsUsed, err)
		}

		iterLog.OracleResponse = llmResp.Content
		iterLog.PromptTokens = llmResp.TokensUsed.PromptTokens
		iterLog.CompletionTokens = llmResp.TokensUsed.CompletionTokens
		iterLog.TotalTokens = llmResp.TokensUsed.TotalTokens
		r.accrueTokens(req, result, draftModel, llmResp.TokensUsed)

		workingContext = append(workingContext, ports.OracleMessage{
			Role:      ports.RoleAssistant,
			Content:   llmResp.Content,
			ToolCalls: llmResp.ToolCalls,
		})
		result.WorkingContext = workingContext

		if len(llmResp.ToolCalls) == 0 {
			if r.isTerminalResponse(llmResp) {
				// The draft model concluded; re-synthesize the user-facing answer on the primary
				// (strong) model — tools stripped — so the quality gates below evaluate what is
				// actually delivered. An empty model targets the primary.
				if r.tieringActive(req) && !result.ResponseDelivered {
					if strongResp, serr := r.callLLM(ctx, provider, req, workingContext, nil, "", ""); serr == nil {
						r.accrueTokens(req, result, "", strongResp.TokensUsed)
						llmResp.Content = strongResp.Content
						workingContext[len(workingContext)-1].Content = strongResp.Content
						result.WorkingContext = workingContext
					} else {
						r.logger.Warnw("primary-model synthesis failed, keeping draft answer", "error", serr)
					}
				}
				if plan != nil && !planNudged && !result.ResponseDelivered {
					if inc := plan.incomplete(); len(inc) > 0 {
						planNudged = true
						workingContext = append(workingContext, planNudgeMessage(inc))
						result.WorkingContext = workingContext
						r.logger.Infow("plan has incomplete items, nudging before delivery", "open", inc)
						appendIter(&iterLog, iterStart)
						continue
					}
				}
				if r.reflectionPolicy.Enabled && reflectionRounds < r.reflectionPolicy.MaxRounds && !result.ResponseDelivered {
					pass, issues, usage := r.reflect(ctx, provider, req, workingContext)
					result.accumulateTokens(usage)
					if !pass {
						reflectionRounds++
						iterLog.ReflectionIssues = issues
						workingContext = append(workingContext, reflectionRevisionMessage(issues))
						result.WorkingContext = workingContext
						r.logger.Infow("reflection requested a revision",
							"issues", issues,
							"round", reflectionRounds,
						)
						appendIter(&iterLog, iterStart)
						continue
					}
				}
				if r.groundingPolicy.Enabled && !result.ResponseDelivered {
					revise, blocked := r.enforceGrounding(req, llmResp.Content, result)
					if revise && !groundingRevised {
						groundingRevised = true
						workingContext = append(workingContext, citationRevisionMessage())
						result.WorkingContext = workingContext
						appendIter(&iterLog, iterStart)
						continue
					}
					if blocked {
						result.FinalSession = req.Session
						appendIter(&iterLog, iterStart)
						return result, nil
					}
				}

				result.FinalResponse = llmResp.Content
				result.FinalSession = req.Session
				if plan != nil {
					result.Plan = plan.items
				}
				appendIter(&iterLog, iterStart)
				r.logger.Infow("reasoning loop ended — terminal response",
					"stop_reason", llmResp.StopReason,
					"iteration", result.IterationsUsed,
				)
				return result, nil
			}
			appendIter(&iterLog, iterStart)
			continue
		}

		r.logger.Infow("executing Action calls",
			"action_count", len(llmResp.ToolCalls),
			"iteration", result.IterationsUsed,
		)

		totalActionCalls += len(llmResp.ToolCalls)
		if r.maxActionsPerCycle > 0 && totalActionCalls > r.maxActionsPerCycle {
			appendIter(&iterLog, iterStart)
			result.FinalError = fmt.Sprintf("action budget of %d exceeded", r.maxActionsPerCycle)
			result.FinalSession = req.Session
			return result, ErrToolBudgetExceeded(r.maxActionsPerCycle)
		}

		newBannedActions, isInfraTerminal, err := r.processActionCalls(ctx, provider, req, result, llmResp.ToolCalls, &iterLog, &workingContext, enforceVoiceDelivery, plan)
		for _, name := range newBannedActions {
			if !bannedActions[name] {
				bannedActions[name] = true
				r.logger.Warnw("Action banned for remainder of turn", "action", name)
			}
		}
		iterLog.BannedActions = newBannedActions
		if isInfraTerminal {
			infraTerminal = true
		}
		if err != nil {
			appendIter(&iterLog, iterStart)
			result.FinalError = fmt.Sprintf("Critical Action failed at iteration %d", result.IterationsUsed)
			result.FinalSession = req.Session
			result.WorkingContext = workingContext
			return result, err
		}

		prevTopic := activeTopic
		workingContext, activeTopic, conversationOffset = r.handleTopicSwitch(
			ctx, req, workingContext, conversationOffset, activeTopic,
		)
		if activeTopic != prevTopic {
			topicSwitchedThisTurn = true
		}

		result.WorkingContext = workingContext

		if stall.observe(llmResp.ToolCalls) {
			r.logger.Warnw("reasoning loop stalled, forcing final synthesis",
				"memory_key", req.Event.MemoryKey,
				"iteration", result.IterationsUsed,
			)
			result.FinalResponse = r.synthesizeFinal(ctx, provider, req, workingContext, result)
			result.FinalError = fmt.Sprintf("reasoning stalled at iteration %d; forced final synthesis", result.IterationsUsed)
			result.FinalSession = req.Session
			appendIter(&iterLog, iterStart)
			return result, nil
		}

		if r.compressionPolicy.Enabled && !topicSwitchedThisTurn {
			iterBoundaries = append(iterBoundaries, len(workingContext))
			workingContext, iterBoundaries = r.maybeCompress(ctx, provider, req, workingContext, conversationOffset, iterBoundaries, result)
			result.WorkingContext = workingContext
		}

		appendIter(&iterLog, iterStart)
	}

	r.logger.Errorw("max iterations reached without completion",
		"memory_key", req.Event.MemoryKey,
		"iterations", result.IterationsUsed,
	)
	result.FinalSession = req.Session

	// With stall detection enabled, exhausting the cap yields a forced synthesis instead of a
	// canned message, so the user still gets a real answer built from the gathered context.
	if r.progressPolicy.Enabled {
		result.FinalResponse = r.synthesizeFinal(ctx, provider, req, workingContext, result)
		result.FinalError = fmt.Sprintf("max iterations (%d) reached; forced final synthesis", r.maxIterations)
		return result, nil
	}

	result.FinalResponse = r.maxIterationsMessage
	result.FinalError = fmt.Sprintf("Max iterations (%d) reached without completion", r.maxIterations)

	return result, fmt.Errorf("max reasoning iterations (%d) reached without completion", r.maxIterations)
}

// handleTopicSwitch detects whether an Action changed the session's subject_key mid-loop.
// When detected, it rebuilds Threads for the new topic and splices it into
// the working context: [new topic history] + [current cycle messages].
// Returns the updated working context, the new active topic, and the updated offset.
func (r *ReasoningService) handleTopicSwitch(
	ctx context.Context,
	req *ReasoningRequest,
	workingContext []ports.OracleMessage,
	conversationOffset int,
	previousTopic string,
) ([]ports.OracleMessage, string, int) {
	currentTopic := sessionTopicKey(req.Session)
	if currentTopic == previousTopic || r.memoryManager == nil {
		return workingContext, currentTopic, conversationOffset
	}

	r.logger.Infow("topic switch detected, reloading memory for next iteration",
		"from", previousTopic,
		"to", currentTopic,
		"memory_key", req.Session.MemoryKey,
	)

	// Capture TopicFacts written by Actions this cycle before the rebuild overwrites them.
	// The Action cleared stale facts from the previous topic and may have written new ones
	// for the incoming topic — those must survive the session reload.
	pendingFacts := req.Session.TopicFacts

	newSession, err := r.memoryManager.RebuildForTopic(ctx, req.Session.MemoryKey, currentTopic)
	if err != nil {
		r.logger.Warnw("failed to rebuild session after topic switch, continuing without reload",
			"error", err,
			"memory_key", req.Session.MemoryKey,
		)
		return workingContext, currentTopic, conversationOffset
	}

	if len(pendingFacts) > 0 {
		if newSession.TopicFacts == nil {
			newSession.TopicFacts = make(map[string]any)
		}
		for k, v := range pendingFacts {
			newSession.TopicFacts[k] = v
		}
	}

	req.Session = newSession

	newHistory := chatMessagesToLLM(newSession.Threads)
	newWorkingContext := append(newHistory, workingContext[conversationOffset:]...)

	r.logger.Infow("working context updated after topic switch",
		"new_history_messages", len(newHistory),
		"cycle_messages", len(workingContext[conversationOffset:]),
		"memory_key", req.Session.MemoryKey,
	)

	return newWorkingContext, currentTopic, len(newHistory)
}

// initializeWorkingContext builds the initial working context from conversation history.
// Webhooks and automated events may have no UserMessage — the system prompt carries the intent.
func (r *ReasoningService) initializeWorkingContext(req *ReasoningRequest) []ports.OracleMessage {
	ctxLen := len(req.ConversationContext)
	workingContext := make([]ports.OracleMessage, ctxLen)
	copy(workingContext, req.ConversationContext)

	userMsg := req.Event.UserMessage
	if userMsg == "" {
		return workingContext
	}

	if ctxLen > 0 {
		last := workingContext[ctxLen-1]
		if last.Role == ports.RoleUser && last.Content == userMsg {
			return workingContext
		}
	}

	return append(workingContext, ports.OracleMessage{
		Role:    ports.RoleUser,
		Content: userMsg,
	})
}

// callLLM assembles the LLM request and delegates to the provider.
// closingHint is appended to the system prompt when non-empty — never stored in conversation history.
// callLLM issues one generation. model selects the target model; an empty model uses the Spirit's
// primary. When model differs from the primary, the provider is resolved for that model so draft and
// strong tiers may even live on different providers.
func (r *ReasoningService) callLLM(
	ctx context.Context,
	provider ports.Oracle,
	req *ReasoningRequest,
	messages []ports.OracleMessage,
	actions []ports.OracleTool,
	closingHint string,
	model string,
) (*ports.OracleResponse, error) {
	if model == "" {
		model = req.Spirit.ModelConfig.Model
	}
	useProvider := provider
	if model != req.Spirit.ModelConfig.Model {
		if p, err := r.oracleFactory.GetProviderForModel(model); err == nil && p != nil {
			useProvider = p
		}
	}

	systemPrompt := req.SystemPrompt
	if closingHint != "" {
		systemPrompt += closingHint
	}
	resp, err := useProvider.GenerateResponse(ctx, &ports.OracleRequest{
		Model:        model,
		SystemPrompt: systemPrompt,
		Messages:     messages,
		Tools:        actions,
		Temperature:  req.Spirit.ModelConfig.Temperature,
		MaxTokens:    req.Spirit.ModelConfig.MaxTokens,
		UseTools:     len(actions) > 0,
		Attachments:  media.ConvertToLLMAttachments(req.Event.Attachments, useProvider, model),
	})
	if err != nil {
		return nil, fmt.Errorf("oracle generate response: %w", err)
	}
	return resp, nil
}

// processActionCalls executes all Action calls and appends results to the working context.
// Returns:
//   - banned: Action names to ban for the rest of the turn (critical business errors)
//   - infraTerminal: true when a critical infra error requires stripping all Actions
//   - err: non-nil only on hard stop (critical infra error, EnforceVoiceDelivery = false)
func (r *ReasoningService) processActionCalls(
	ctx context.Context,
	provider ports.Oracle,
	req *ReasoningRequest,
	result *ReasoningResult,
	actionCalls []ports.OracleToolCall,
	iterLog *entities.IterationLog,
	workingContext *[]ports.OracleMessage,
	enforceVoiceDelivery bool,
	plan *planState,
) (banned []string, infraTerminal bool, err error) {
	results := r.executeActionsWithPlan(ctx, req, actionCalls, plan)

	if len(results) != len(actionCalls) {
		return nil, false, fmt.Errorf("executor returned %d results for %d action calls",
			len(results), len(actionCalls))
	}

	for i, call := range actionCalls {
		actionResult := results[i]

		if actionResult.Error == nil {
			r.handleActionSuccess(ctx, provider, req, call, actionResult, result, workingContext)
		} else {
			iterLog.Errors = append(iterLog.Errors, fmt.Sprintf("%s: %v", call.Name, actionResult.Error))
			isBanned, isInfraTerminal, handleErr := r.handleActionError(call, actionResult, workingContext, enforceVoiceDelivery)
			if handleErr != nil {
				iterLog.ActionCalls = append(iterLog.ActionCalls, buildActionCallLog(call, actionResult))
				return banned, false, handleErr
			}
			if isBanned {
				banned = append(banned, call.Name)
			}
			if isInfraTerminal {
				infraTerminal = true
			}
		}

		iterLog.ActionCalls = append(iterLog.ActionCalls, buildActionCallLog(call, actionResult))
	}

	return banned, infraTerminal, nil
}

// executeAllActions routes summon_spirit calls through SummonService and delegates the rest
// to ActionExecutor. When ParallelSummon is enabled on the orchestrator, all summon calls
// in the same iteration run concurrently.
func (r *ReasoningService) executeAllActions(
	ctx context.Context,
	req *ReasoningRequest,
	calls []ports.OracleToolCall,
) []ActionResult {
	calls = applyIsCriticalOverrides(calls, req.Spirit.AllowedActions)
	if r.summonService == nil || !req.Spirit.IsOrchestrator() {
		return r.actionExecutor.ExecuteAll(ctx, calls)
	}

	// Partition: separate summon calls from regular action calls.
	type indexedCall struct {
		index int
		call  ports.OracleToolCall
	}
	var summonCalls, regularCalls []indexedCall
	for i, c := range calls {
		if c.Name == summonSpiritActionName {
			summonCalls = append(summonCalls, indexedCall{i, c})
		} else {
			regularCalls = append(regularCalls, indexedCall{i, c})
		}
	}

	results := make([]ActionResult, len(calls))

	// Execute regular actions.
	if len(regularCalls) > 0 {
		extracted := make([]ports.OracleToolCall, len(regularCalls))
		for i, ic := range regularCalls {
			extracted[i] = ic.call
		}
		regularResults := r.actionExecutor.ExecuteAll(ctx, extracted)
		for i, ic := range regularCalls {
			results[ic.index] = regularResults[i]
		}
	}

	// Execute summon calls.
	if len(summonCalls) > 0 {
		if req.Spirit.OrchestratorConfig.ParallelSummon && len(summonCalls) > 1 {
			summonResults := make([]ActionResult, len(summonCalls))
			var wg sync.WaitGroup
			for i, ic := range summonCalls {
				wg.Add(1)
				go func(idx int, call ports.OracleToolCall) {
					defer wg.Done()
					summonResults[idx] = r.executeSummonCall(ctx, call, req)
				}(i, ic.call)
			}
			wg.Wait()
			for i, ic := range summonCalls {
				results[ic.index] = summonResults[i]
			}
		} else {
			for _, ic := range summonCalls {
				results[ic.index] = r.executeSummonCall(ctx, ic.call, req)
			}
		}
	}

	return results
}

func (r *ReasoningService) executeSummonCall(ctx context.Context, call ports.OracleToolCall, req *ReasoningRequest) ActionResult {
	spiritName, _ := call.Arguments["spirit_name"].(string)
	task, _ := call.Arguments["task"].(string)

	if spiritName == "" || task == "" {
		return ActionResult{
			Error:      errors.NewBusinessError("summon_spirit requires spirit_name and task"),
			IsCritical: false,
		}
	}

	response, err := r.summonService.Summon(ctx, spiritName, task, req.Event, req.Spirit)
	if err != nil {
		return ActionResult{Error: err, IsCritical: true}
	}
	return ActionResult{Result: response}
}

func (r *ReasoningService) handleActionSuccess(
	ctx context.Context,
	provider ports.Oracle,
	req *ReasoningRequest,
	call ports.OracleToolCall,
	actionResult ActionResult,
	result *ReasoningResult,
	workingContext *[]ports.OracleMessage,
) {
	// The full result is preserved in the audit log (buildActionCallLog); only the message that
	// re-enters the reasoning context is shaped, to protect the model's window on large results.
	*workingContext = append(*workingContext, ports.OracleMessage{
		Role:       ports.RoleTool,
		Content:    r.shapeResultForContext(ctx, provider, req, call.Name, actionResult.Result, result),
		ToolCallID: call.ID,
		ToolName:   call.Name,
	})

	if actionResult.IsVoiceDelivery {
		result.ResponseDelivered = true
		r.logger.Infow("Voice delivery Action succeeded — interaction complete", "action", call.Name)
	} else {
		r.logger.Infow("Action executed successfully", "action", call.Name)
	}
}

func (r *ReasoningService) handleActionError(
	call ports.OracleToolCall,
	result ActionResult,
	workingContext *[]ports.OracleMessage,
	enforceVoiceDelivery bool,
) (banned bool, infraTerminal bool, err error) {
	if !result.IsCritical {
		// Non-critical: informational only — the LLM decides whether to try an alternative path.
		r.logger.Warnw("non-critical Action error", "action", call.Name, "error", result.Error)
		*workingContext = append(*workingContext, actionErrorMessage(call.ID, call.Name, result))
		return false, false, nil
	}

	if errors.IsBusinessError(result.Error) {
		r.logger.Warnw("critical business Action failed", "action", call.Name, "error", result.Error)
		*workingContext = append(*workingContext, actionErrorMessage(call.ID, call.Name, result))
		return true, false, nil
	}

	r.logger.Errorw("critical infrastructure Action failed", "action", call.Name, "error", result.Error)
	if enforceVoiceDelivery {
		// Conversational Spirit: strip all Actions so the LLM can only produce a text apology.
		*workingContext = append(*workingContext, actionErrorMessage(call.ID, call.Name, result))
		return false, true, nil
	}
	// Non-conversational Spirit: hard stop, no user notification.
	return false, false, fmt.Errorf("critical action %q: %w", call.Name, result.Error)
}

// isTerminalResponse reports whether the loop should stop.
// Only called when the LLM produced no Action calls.
func (r *ReasoningService) isTerminalResponse(resp *ports.OracleResponse) bool {
	switch resp.StopReason {
	case ports.StopReasonComplete:
		return true
	case ports.StopReasonToolCalls:
		return false
	case ports.StopReasonLength:
		r.logger.Warnw("LLM reached token limit", "stop_reason", resp.StopReason)
		return true
	case ports.StopReasonContentFilter:
		r.logger.Warnw("LLM response blocked by content filter")
		return true
	default:
		r.logger.Warnw("unknown stop reason, terminating loop", "stop_reason", resp.StopReason)
		return true
	}
}

// availableActions resolves Action descriptors for the Spirit's allowed Actions.
// For orchestrator Spirits, summon_spirit is injected automatically when SummonService is wired.
// Actions missing from the registry are skipped with a warning.
func (r *ReasoningService) availableActions(req *ReasoningRequest) []ports.OracleTool {
	actions := make([]ports.OracleTool, 0, len(req.Spirit.AllowedActions))
	for _, aa := range req.Spirit.AllowedActions {
		action, err := r.actionExecutor.GetAction(aa.Name)
		if err != nil {
			r.logger.Warnw("Action not found in registry, skipping", "action", aa.Name, "error", err)
			continue
		}
		description := action.GetDescription()
		if aa.DescriptionOverride != "" {
			description = aa.DescriptionOverride
		}
		actions = append(actions, ports.OracleTool{
			Name:        action.GetName(),
			Description: description,
			Parameters:  action.GetParameters(),
		})
	}

	if r.summonService != nil && req.Spirit.IsOrchestrator() && len(req.Spirit.OrchestratorConfig.SubSpirits) > 0 {
		actions = append(actions, summonSpiritTool(req.Spirit.OrchestratorConfig.SubSpirits))
	}

	if r.planPolicy.Enabled {
		actions = append(actions, updatePlanTool())
	}

	return actions
}

// resolveIterationActions returns the active Action set and closing hint for the current iteration.
// The hint is ephemeral — it is never stored in conversation history.
func resolveIterationActions(actions []ports.OracleTool, banned map[string]bool, infraTerminal bool) ([]ports.OracleTool, string) {
	switch {
	case infraTerminal:
		return nil, infraClosingInstruction
	case len(banned) > 0:
		return filterBannedActions(actions, banned), closingInstruction
	default:
		return actions, ""
	}
}

// actionErrorMessage builds the Action result the LLM receives on failure.
// Critical business errors include the domain message and an explicit block notice.
// Critical infra errors are intentionally generic — details must not reach the user.
// Non-critical business errors pass the domain message so the LLM can reason about alternatives.
func actionErrorMessage(callID, name string, result ActionResult) ports.OracleMessage {
	msg := ports.OracleMessage{
		Role:       ports.RoleTool,
		ToolCallID: callID,
		ToolName:   name,
		IsError:    true,
	}

	switch {
	case result.IsCritical && errors.IsBusinessError(result.Error):
		msg.Content = fmt.Sprintf(
			"Business error: %s\nThis action has been blocked and cannot be called again in this conversation turn.",
			errors.MessageFor(result.Error),
		)
	case result.IsCritical:
		msg.Content = "This operation failed due to a technical error."
	case errors.IsBusinessError(result.Error):
		msg.Content = fmt.Sprintf("Business error: %s", errors.MessageFor(result.Error))
	default:
		msg.Content = "This operation is temporarily unavailable. Try an alternative approach if possible."
	}

	return msg
}

// filterBannedActions returns Actions with banned entries removed.
func filterBannedActions(actions []ports.OracleTool, banned map[string]bool) []ports.OracleTool {
	if len(banned) == 0 {
		return actions
	}
	filtered := make([]ports.OracleTool, 0, len(actions))
	for _, a := range actions {
		if !banned[a.Name] {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func buildActionCallLog(call ports.OracleToolCall, result ActionResult) entities.ActionCallLog {
	log := entities.ActionCallLog{
		ActionName: call.Name,
		Arguments:  call.Arguments,
		IsCritical: result.IsCritical,
		DurationMs: result.DurationMs,
	}
	if result.Error != nil {
		log.IsError = true
		log.ErrorMessage = result.Error.Error()
	} else {
		log.Result = result.Result
	}
	return log
}

func chatMessagesToLLM(messages []entities.Thread) []ports.OracleMessage {
	result := make([]ports.OracleMessage, len(messages))
	for i, msg := range messages {
		result[i] = ports.OracleMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			ImageURLs: msg.ImageURLs,
			AudioURLs: msg.AudioURLs,
		}
	}
	return result
}

func sessionTopicKey(session *entities.Memory) string {
	if session == nil {
		return ""
	}
	return session.SubjectKey
}

const summonSpiritActionName = "summon_spirit"

func summonSpiritTool(subSpirits []string) ports.OracleTool {
	return ports.OracleTool{
		Name: summonSpiritActionName,
		Description: "Delegates a task to a specialized sub-Spirit and returns its response. " +
			"Use this to coordinate work across multiple Spirits. " +
			"Available sub-Spirits: " + fmt.Sprintf("%v", subSpirits),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"spirit_name": map[string]any{
					"type":        "string",
					"description": "Name of the sub-Spirit to summon.",
					"enum":        subSpirits,
				},
				"task": map[string]any{
					"type":        "string",
					"description": "The task or question to delegate to the sub-Spirit.",
				},
			},
			"required": []string{"spirit_name", "task"},
		},
	}
}

func applyIsCriticalOverrides(calls []ports.OracleToolCall, allowed []entities.AllowedAction) []ports.OracleToolCall {
	index := make(map[string]*bool, len(allowed))
	for _, aa := range allowed {
		if aa.IsCritical != nil {
			index[aa.Name] = aa.IsCritical
		}
	}
	if len(index) == 0 {
		return calls
	}
	enriched := make([]ports.OracleToolCall, len(calls))
	copy(enriched, calls)
	for i, call := range enriched {
		if override, ok := index[call.Name]; ok {
			enriched[i].IsCriticalOverride = override
		}
	}
	return enriched
}
