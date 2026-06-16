package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// ProgressPolicy configures stall detection for the reasoning loop. Defined in entities so a Spirit
// can override it; aliased here for the orchestrator's use.
type ProgressPolicy = entities.ProgressPolicy

// stallSynthesisInstruction is appended ephemerally to the system prompt for the forced final
// synthesis call — never stored in conversation history.
const stallSynthesisInstruction = "\n\n--- FINAL SYNTHESIS ---\n" +
	"You have reached the limit of useful steps. Provide the best possible answer to the user using " +
	"the information gathered so far. Do not request any more actions."

// stallTracker detects when the reasoning loop stops making progress, i.e. the model keeps issuing
// Action calls it has already made (oscillation). An iteration "progresses" when it introduces at
// least one call signature not seen before in the turn; otherwise the no-progress streak grows.
// A window of zero disables detection.
type stallTracker struct {
	window int
	seen   map[string]bool
	streak int
}

func newStallTracker(window int) *stallTracker {
	return &stallTracker{window: window, seen: make(map[string]bool)}
}

// observe records an iteration's Action calls and reports whether the loop is now stalled.
func (s *stallTracker) observe(calls []ports.OracleToolCall) bool {
	if s.window <= 0 {
		return false
	}

	progressed := false
	for _, c := range calls {
		sig := callSignature(c)
		if !s.seen[sig] {
			s.seen[sig] = true
			progressed = true
		}
	}

	if progressed {
		s.streak = 0
		return false
	}

	s.streak++
	return s.streak >= s.window
}

// callSignature identifies an Action call by name and arguments. encoding/json sorts map keys, so
// semantically identical calls hash equal regardless of argument ordering. Used to detect a model
// repeating the exact same call (oscillation).
func callSignature(c ports.OracleToolCall) string {
	argsJSON := []byte("{}")
	if len(c.Arguments) > 0 {
		if b, err := json.Marshal(c.Arguments); err == nil {
			argsJSON = b
		}
	}
	sum := sha256.Sum256(append([]byte(c.Name+"\x00"), argsJSON...))
	return hex.EncodeToString(sum[:])
}
