package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers/otelgenai"
)

// turnState holds everything scoped to a single reasoning turn. Bundling it keeps the loop body and
// its policy gates operating on one object instead of a dozen threaded locals and closures.
type turnState struct {
	req      *ReasoningRequest
	provider ports.Oracle
	emit     reasoningEmitter
	result   *ReasoningResult

	actions            []ports.OracleTool
	workingContext     []ports.OracleMessage
	activeTopic        string
	conversationOffset int

	enforceVoiceDelivery bool
	bannedSignatures     map[string]bool
	infraTerminal        bool
	totalActionCalls     int
	iterBoundaries       []int
	topicSwitched        bool
	reflectionRounds     int
	groundingRevised     bool
	planNudged           bool
	criticalErrors       int

	// Effective policies for this turn (per-Spirit override or global default) and derived state.
	progress           ProgressPolicy
	reflection         ReflectionPolicy
	grounding          GroundingPolicy
	planPolicy         PlanPolicy
	handoff            HandoffPolicy
	compressionEnabled bool
	plan               *planState
	stall              *stallTracker
	draftModel         string
}

// run drives the reasoning loop. emit is nil for the buffered path (Execute); ExecuteStream passes an
// emitter that receives text deltas and tool-status events as the turn progresses. Loop logic is
// identical either way — only the LLM calls stream and a few status events are emitted when emit is set.
func (r *ReasoningService) run(ctx context.Context, req *ReasoningRequest, emit reasoningEmitter) (*ReasoningResult, error) {
	ctx, span := r.tracer.Start(ctx, "ReasoningService/Execute")
	defer span.End()

	ts, err := r.newTurnState(req, emit)
	if err != nil {
		return nil, err
	}

	// Runs before span.End (LIFO) so the turn span carries final usage/status regardless of exit path.
	defer func() {
		status := "success"
		if ts.result.FinalError != "" {
			status = "error"
		}
		otelgenai.SetTurnAttributes(span, otelgenai.Turn{
			MemoryKey:    req.Event.MemoryKey,
			Spirit:       req.Spirit.Name,
			Iterations:   ts.result.IterationsUsed,
			InputTokens:  ts.result.TokensUsed.PromptTokens,
			OutputTokens: ts.result.TokensUsed.CompletionTokens,
			Status:       status,
		})
	}()

	for iteration := range r.maxIterations {
		done, err := r.runIteration(ctx, ts, iteration)
		if err != nil {
			return ts.result, err
		}
		if done {
			return ts.result, nil
		}
	}

	return r.handleMaxIterations(ctx, ts)
}

// newTurnState resolves the provider, working context, effective policies, and turn-scoped state.
func (r *ReasoningService) newTurnState(req *ReasoningRequest, emit reasoningEmitter) (*turnState, error) {
	provider, err := r.oracleFactory.GetProvider(req.Spirit.ModelConfig.Provider)
	if err != nil {
		return nil, fmt.Errorf("get LLM provider: %w", err)
	}

	workingContext := r.initializeWorkingContext(req)
	conversationOffset := len(workingContext)
	if req.Event.UserMessage != "" {
		conversationOffset = len(workingContext) - 1
	}

	progressPolicy := r.effectiveProgress(req)
	reflectionPolicy := r.effectiveReflection(req)
	groundingPolicy := r.effectiveGrounding(req)
	planPolicy := r.effectivePlan(req)
	handoffPolicy := r.effectiveHandoff(req)

	// When grounding is enabled and Lore was retrieved, instruct the model to cite sources.
	// Ephemeral for the turn — req is turn-scoped.
	if groundingPolicy.Enabled && retrievedLoreContext(req) != "" {
		req.SystemPrompt += groundingAddendum
	}

	// plan is nil unless the plan policy is enabled; the model maintains it via update_plan.
	var plan *planState
	if planPolicy.Enabled {
		plan = newPlanState(planPolicy.MaxItems)
		if planPolicy.Required {
			req.SystemPrompt += planRequiredInstruction
		}
	}

	stallWindow := 0
	if progressPolicy.Enabled {
		stallWindow = progressPolicy.StallWindow
	}

	// With model tiering, tool-using iterations run on the cheaper draft model; the terminal answer
	// is re-synthesized on the strong model. draftModel is "" (primary) when tiering is off.
	draftModel := ""
	if r.tieringActive(req) {
		draftModel = r.tierDraftModel(req)
	}

	return &turnState{
		req:                  req,
		provider:             provider,
		emit:                 emit,
		actions:              r.availableActions(req),
		workingContext:       workingContext,
		activeTopic:          sessionTopicKey(req.Session),
		conversationOffset:   conversationOffset,
		enforceVoiceDelivery: req.Spirit.EnforceVoiceDelivery,
		bannedSignatures:     make(map[string]bool),
		iterBoundaries:       []int{},
		progress:             progressPolicy,
		reflection:           reflectionPolicy,
		grounding:            groundingPolicy,
		planPolicy:           planPolicy,
		handoff:              handoffPolicy,
		compressionEnabled:   r.effectiveCompression(req).Enabled,
		plan:                 plan,
		stall:                newStallTracker(stallWindow),
		draftModel:           draftModel,
		result: &ReasoningResult{
			Iterations:      []entities.IterationLog{},
			ExecutionErrors: []string{},
			WorkingContext:  workingContext,
			FinalSession:    req.Session,
		},
	}, nil
}

func (r *ReasoningService) appendIter(ts *turnState, log *entities.IterationLog, start time.Time) {
	log.DurationMs = time.Since(start).Milliseconds()
	ts.result.Iterations = append(ts.result.Iterations, *log)
}

// runIteration runs one reasoning iteration. It returns done=true when the turn has produced its final
// result (caller returns result, nil), a non-nil error to abort the turn, or done=false to continue.
func (r *ReasoningService) runIteration(ctx context.Context, ts *turnState, iteration int) (bool, error) {
	req := ts.req
	ts.result.IterationsUsed = iteration + 1

	if err := ctx.Err(); err != nil {
		ts.result.FinalError = fmt.Sprintf("context cancelled at iteration %d", ts.result.IterationsUsed)
		ts.result.FinalSession = req.Session
		return false, fmt.Errorf("context cancelled at iteration %d: %w", ts.result.IterationsUsed, err)
	}

	r.logger.Infow("reasoning iteration", "iteration", ts.result.IterationsUsed, "memory_key", req.Event.MemoryKey)

	iterStart := time.Now()
	iterLog := entities.IterationLog{Iteration: ts.result.IterationsUsed, Errors: []string{}}
	defer r.appendIter(ts, &iterLog, iterStart)

	activeActions, closingHint := resolveIterationActions(ts.actions, ts.infraTerminal)
	if ts.plan != nil {
		// The current plan is injected ephemerally each iteration — always up to date, never persisted.
		closingHint = ts.plan.render() + closingHint
	}

	llmResp, err := r.callLLM(ctx, ts.provider, req, ts.workingContext, activeActions, closingHint, ts.draftModel, ts.emit, nil)
	if err != nil {
		iterLog.Errors = append(iterLog.Errors, fmt.Sprintf("LLM call failed: %v", err))
		ts.result.FinalError = fmt.Sprintf("LLM call failed at iteration %d", ts.result.IterationsUsed)
		ts.result.FinalSession = req.Session
		return false, fmt.Errorf("LLM call at iteration %d: %w", ts.result.IterationsUsed, err)
	}

	iterLog.OracleResponse = llmResp.Content
	iterLog.PromptTokens = llmResp.TokensUsed.PromptTokens
	iterLog.CompletionTokens = llmResp.TokensUsed.CompletionTokens
	iterLog.TotalTokens = llmResp.TokensUsed.TotalTokens
	r.accrueTokens(req, ts.result, ts.draftModel, llmResp.TokensUsed)

	ts.workingContext = append(ts.workingContext, ports.OracleMessage{
		Role:      ports.RoleAssistant,
		Content:   llmResp.Content,
		ToolCalls: llmResp.ToolCalls,
	})
	ts.result.WorkingContext = ts.workingContext

	if len(llmResp.ToolCalls) == 0 {
		if r.isTerminalResponse(llmResp) {
			return r.finalizeTerminalResponse(ctx, ts, llmResp, &iterLog), nil
		}
		return false, nil
	}

	return r.executeIterationActions(ctx, ts, llmResp, &iterLog)
}

// executeIterationActions runs the Action calls the model requested and advances turn state (topic
// switch, stall detection, compression). It returns done=true only when a stall forces a final answer.
func (r *ReasoningService) executeIterationActions(ctx context.Context, ts *turnState, llmResp *ports.OracleResponse, iterLog *entities.IterationLog) (bool, error) {
	req := ts.req

	r.logger.Infow("executing Action calls", "action_count", len(llmResp.ToolCalls), "iteration", ts.result.IterationsUsed)
	if ts.emit != nil {
		for _, tc := range llmResp.ToolCalls {
			ts.emit(ReasoningEvent{Type: ReasoningEventToolStatus, ToolName: tc.Name})
		}
	}

	ts.totalActionCalls += len(llmResp.ToolCalls)
	if r.maxActionsPerCycle > 0 && ts.totalActionCalls > r.maxActionsPerCycle {
		ts.result.FinalError = fmt.Sprintf("action budget of %d exceeded", r.maxActionsPerCycle)
		ts.result.FinalSession = req.Session
		return false, ErrToolBudgetExceeded(r.maxActionsPerCycle)
	}

	newBanned, isInfraTerminal, err := r.processActionCalls(ctx, ts.provider, req, ts.result, llmResp.ToolCalls, iterLog, &ts.workingContext, ts.enforceVoiceDelivery, ts.plan, ts.bannedSignatures)
	for _, sig := range newBanned {
		ts.bannedSignatures[sig] = true
	}
	if len(newBanned) > 0 || isInfraTerminal {
		ts.criticalErrors++
	}
	if isInfraTerminal {
		ts.infraTerminal = true
	}
	if err != nil {
		ts.result.FinalError = fmt.Sprintf("Critical Action failed at iteration %d", ts.result.IterationsUsed)
		ts.result.FinalSession = req.Session
		ts.result.WorkingContext = ts.workingContext
		return false, err
	}

	prevTopic := ts.activeTopic
	ts.workingContext, ts.activeTopic, ts.conversationOffset = r.handleTopicSwitch(ctx, req, ts.workingContext, ts.conversationOffset, ts.activeTopic)
	if ts.activeTopic != prevTopic {
		ts.topicSwitched = true
	}
	ts.result.WorkingContext = ts.workingContext

	if ts.stall.observe(llmResp.ToolCalls) {
		r.logger.Warnw("reasoning loop stalled, forcing final synthesis", "memory_key", req.Event.MemoryKey, "iteration", ts.result.IterationsUsed)
		ts.result.FinalResponse = r.synthesizeFinal(ctx, ts.provider, req, ts.workingContext, ts.result, ts.emit)
		ts.result.FinalError = fmt.Sprintf("reasoning stalled at iteration %d; forced final synthesis", ts.result.IterationsUsed)
		ts.result.FinalSession = req.Session
		return true, nil
	}

	if ts.compressionEnabled && !ts.topicSwitched {
		ts.iterBoundaries = append(ts.iterBoundaries, len(ts.workingContext))
		ts.workingContext, ts.iterBoundaries = r.maybeCompress(ctx, ts.provider, req, ts.workingContext, ts.conversationOffset, ts.iterBoundaries, ts.result)
		ts.result.WorkingContext = ts.workingContext
	}

	return false, nil
}

// finalizeTerminalResponse applies the pre-delivery quality gates to a tool-free answer: optional
// strong-model re-synthesis, plan nudge, reflection, grounding, and confidence handoff, then sets the
// final (optionally structured) response. It returns done=false to send the turn back for another
// iteration (a gate requested a revision) or done=true once the answer is final.
func (r *ReasoningService) finalizeTerminalResponse(ctx context.Context, ts *turnState, llmResp *ports.OracleResponse, iterLog *entities.IterationLog) bool {
	req := ts.req

	// The draft model concluded; re-synthesize the user-facing answer on the primary (strong) model —
	// tools stripped — so the quality gates below evaluate what is actually delivered.
	if r.tieringActive(req) && !ts.result.ResponseDelivered {
		if strongResp, serr := r.callLLM(ctx, ts.provider, req, ts.workingContext, nil, "", "", ts.emit, nil); serr == nil {
			r.accrueTokens(req, ts.result, "", strongResp.TokensUsed)
			llmResp.Content = strongResp.Content
			ts.workingContext[len(ts.workingContext)-1].Content = strongResp.Content
			ts.result.WorkingContext = ts.workingContext
		} else {
			r.logger.Warnw("primary-model synthesis failed, keeping draft answer", "error", serr)
		}
	}

	if ts.plan != nil && !ts.planNudged && !ts.result.ResponseDelivered {
		if inc := ts.plan.incomplete(); len(inc) > 0 {
			ts.planNudged = true
			ts.workingContext = append(ts.workingContext, planNudgeMessage(inc))
			ts.result.WorkingContext = ts.workingContext
			r.logger.Infow("plan has incomplete items, nudging before delivery", "open", inc)
			return false
		}
	}

	if ts.reflection.Enabled && ts.reflectionRounds < ts.reflection.MaxRounds && !ts.result.ResponseDelivered {
		pass, issues, usage := r.reflect(ctx, ts.provider, req, ts.workingContext)
		ts.result.accumulateTokens(usage)
		if !pass {
			ts.reflectionRounds++
			iterLog.ReflectionIssues = issues
			ts.workingContext = append(ts.workingContext, reflectionRevisionMessage(issues))
			ts.result.WorkingContext = ts.workingContext
			r.logger.Infow("reflection requested a revision", "issues", issues, "round", ts.reflectionRounds)
			return false
		}
	}

	if ts.grounding.Enabled && !ts.result.ResponseDelivered {
		revise, blocked := r.enforceGrounding(req, llmResp.Content, ts.result)
		if revise && !ts.groundingRevised {
			ts.groundingRevised = true
			ts.workingContext = append(ts.workingContext, citationRevisionMessage())
			ts.result.WorkingContext = ts.workingContext
			return false
		}
		if blocked {
			ts.result.FinalSession = req.Session
			return true
		}
	}

	if ts.handoff.Enabled && !ts.result.ResponseDelivered {
		if holding, raised := r.maybeHandoff(ctx, req, ts.result, llmResp.Content, ts.criticalErrors, ts.reflectionRounds); raised {
			ts.result.FinalResponse = holding
			ts.result.FinalSession = req.Session
			if ts.plan != nil {
				ts.result.Plan = ts.plan.items
			}
			return true
		}
	}

	if req.ResponseFormat != nil && !ts.result.ResponseDelivered {
		ts.result.FinalResponse = r.finalizeStructured(ctx, ts.provider, req, ts.workingContext, ts.result)
	} else {
		ts.result.FinalResponse = llmResp.Content
	}
	ts.result.FinalSession = req.Session
	if ts.plan != nil {
		ts.result.Plan = ts.plan.items
	}
	r.logger.Infow("reasoning loop ended — terminal response", "stop_reason", llmResp.StopReason, "iteration", ts.result.IterationsUsed)
	return true
}

// handleMaxIterations produces the answer when the loop exhausts its iteration budget: a forced
// synthesis when stall detection is on, otherwise the configured max-iterations message and an error.
func (r *ReasoningService) handleMaxIterations(ctx context.Context, ts *turnState) (*ReasoningResult, error) {
	req := ts.req
	r.logger.Errorw("max iterations reached without completion", "memory_key", req.Event.MemoryKey, "iterations", ts.result.IterationsUsed)
	ts.result.FinalSession = req.Session

	// With stall detection enabled, exhausting the cap yields a forced synthesis instead of a canned
	// message, so the user still gets a real answer built from the gathered context.
	if ts.progress.Enabled {
		ts.result.FinalResponse = r.synthesizeFinal(ctx, ts.provider, req, ts.workingContext, ts.result, ts.emit)
		ts.result.FinalError = fmt.Sprintf("max iterations (%d) reached; forced final synthesis", r.maxIterations)
		return ts.result, nil
	}

	ts.result.FinalResponse = r.maxIterationsMessage
	ts.result.FinalError = fmt.Sprintf("Max iterations (%d) reached without completion", r.maxIterations)
	return ts.result, fmt.Errorf("max reasoning iterations (%d) reached without completion", r.maxIterations)
}
