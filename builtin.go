package eywa

// Built-in converters, actions, and scorers — no external infrastructure required.

import (
	"github.com/wmulabs/eywa/internal/implementation/actions"
	"github.com/wmulabs/eywa/internal/implementation/receptors"
	"github.com/wmulabs/eywa/internal/implementation/trial"
)

// NewAPIDefaultReceptor creates a Receptor that parses JSON API payloads into Pulses.
// Suitable for direct HTTP integrations where the caller sends eywa's native format.
var NewAPIDefaultReceptor = receptors.NewAPIDefaultReceptor

// NewUpdateSubjectAction lets the Oracle set or update the active SubjectKey and
// accumulate subject facts in memory. Register on Spirits that need to identify
// and track business entities (e.g. order IDs, ticket numbers) conversationally.
var NewUpdateSubjectAction = actions.NewUpdateSubjectAction

// NewScheduleRitualAction lets the Oracle create a future or recurring Pulse via the
// Keeper. Requires WeaveBuilder.WithRitualManager to be configured.
var NewScheduleRitualAction = actions.NewScheduleRitualAction

// NewListRitualsAction lets the Oracle list the user's pending Rituals.
// Requires WeaveBuilder.WithRitualManager to be configured.
var NewListRitualsAction = actions.NewListRitualsAction

// NewCancelRitualAction lets the Oracle cancel a pending Ritual by ID.
// Requires WeaveBuilder.WithRitualManager to be configured.
var NewCancelRitualAction = actions.NewCancelRitualAction

// NewSendToContactAction lets the Oracle send a message to a different contact
// than the one currently in the conversation.
var NewSendToContactAction = actions.NewSendToContactAction

// NewSearchLoreAction lets the Oracle search the Lore knowledge base (RAG) and
// inject the retrieved chunks as context before answering.
// Requires WeaveBuilder.WithLoreRepository to be configured.
var NewSearchLoreAction = actions.NewSearchLoreAction

// NewRememberFactAction lets the Oracle store a persistent user fact in Imprint.
// Requires WeaveBuilder.WithImprintRepository to be configured.
var NewRememberFactAction = actions.NewRememberFactAction

// NewForgetFactAction lets the Oracle delete a previously stored Imprint fact by ID.
// Requires WeaveBuilder.WithImprintRepository to be configured.
var NewForgetFactAction = actions.NewForgetFactAction

// NewRequestRiteAction lets the Oracle pause and request human approval before
// proceeding. The Rite is stored in RiteRepository; the operator decides via
// RiteRepository.Decide or the management API.
// Requires WeaveBuilder.WithRiteRepository to be configured.
var NewRequestRiteAction = actions.NewRequestRiteAction

// NewOutputContainsScorer returns a deterministic Scorer that passes when the response
// contains all of the expected strings.
var NewOutputContainsScorer = trial.NewOutputContainsScorer

// NewOutputNotContainsScorer returns a deterministic Scorer that passes when the
// response contains none of the prohibited strings.
var NewOutputNotContainsScorer = trial.NewOutputNotContainsScorer

// NewActionSequenceScorer returns a deterministic Scorer that passes when the response
// executed Actions in the expected sequence.
var NewActionSequenceScorer = trial.NewActionSequenceScorer

// NewLatencyScorer returns a deterministic Scorer that passes when the processing
// time is below the configured threshold.
var NewLatencyScorer = trial.NewLatencyScorer

// NewLLMJudgeScorer returns an LLM-as-judge Scorer. It uses an Oracle to evaluate
// whether the response meets the expectations defined in LLMJudgeExpect.
var NewLLMJudgeScorer = trial.NewLLMJudgeScorer
