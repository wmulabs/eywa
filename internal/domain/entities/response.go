package entities

import (
	"time"

	"github.com/wmulabs/eywa/internal/helpers"
)

type ResponseStatus string

const (
	ResponseSuccess   ResponseStatus = "success"
	ResponsePartial   ResponseStatus = "partial_success"
	ResponseFailed    ResponseStatus = "failed"
	ResponseDuplicate ResponseStatus = "duplicate"
)

type ActionResult struct {
	ActionName string `json:"action_name"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	ErrorType  string `json:"error_type,omitempty"` // "business" | "infrastructure"
}

// Response is returned to the caller after Weave processes a Pulse.
// End-user communication happens via Actions (e.g. send_whatsapp_message);
// this struct tracks processing status for the caller.
type Response struct {
	Status            ResponseStatus `json:"status"`
	EventID           string         `json:"event_id"`
	MemoryKey         string         `json:"memory_key"`
	SpiritUsed        string         `json:"spirit_used,omitempty"`
	ActionsExecuted   []string       `json:"actions_executed,omitempty"`
	ActionResults     []ActionResult `json:"action_results,omitempty"`
	Message           string         `json:"message"`
	FinalResponse     string         `json:"final_response,omitempty"`
	Error             string         `json:"error,omitempty"`
	ProcessingTimeMs  int64          `json:"processing_time_ms,omitempty"`
	Timestamp         time.Time      `json:"timestamp"`
	ResponseDelivered bool           `json:"response_delivered"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

func NewResponse(eventID, sessionKey, spiritUsed string, actionsExecuted []string) *Response {
	return &Response{
		Status:          ResponseSuccess,
		EventID:         eventID,
		MemoryKey:       sessionKey,
		SpiritUsed:      spiritUsed,
		ActionsExecuted: actionsExecuted,
		Message:         "Pulse processed successfully",
		Timestamp:       helpers.NowUTC(),
	}
}

func NewPartialResponse(eventID, sessionKey, spiritUsed string, actionsExecuted []string, actionResults []ActionResult) *Response {
	return &Response{
		Status:          ResponsePartial,
		EventID:         eventID,
		MemoryKey:       sessionKey,
		SpiritUsed:      spiritUsed,
		ActionsExecuted: actionsExecuted,
		ActionResults:   actionResults,
		Message:         "Pulse processed with some Action failures",
		Timestamp:       helpers.NowUTC(),
	}
}

// NewDuplicateResponse marks an event that was dropped because its IdempotencyKey was
// already processed. It is a successful idempotent no-op — callers (and Cloud Tasks) should
// acknowledge it without retrying.
func NewDuplicateResponse(eventID, sessionKey string) *Response {
	return &Response{
		Status:    ResponseDuplicate,
		EventID:   eventID,
		MemoryKey: sessionKey,
		Message:   "duplicate event ignored (already processed)",
		Timestamp: helpers.NowUTC(),
	}
}

func NewErrorResponse(eventID, sessionKey, errorMsg string) *Response {
	return &Response{
		Status:    ResponseFailed,
		EventID:   eventID,
		MemoryKey: sessionKey,
		Message:   "Pulse processing failed",
		Error:     errorMsg,
		Timestamp: helpers.NowUTC(),
	}
}
