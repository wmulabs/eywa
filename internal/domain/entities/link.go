package entities

import "time"

// Guard defines an allow/block filter evaluated against a specific Pulse field.
// Field uses dot-notation to resolve values from the Pulse:
//   - Top-level fields: "contact_phone", "source", "sub_type", "memory_key", "subject_key", "event_type", "idempotency_key", "user_message"
//   - Map fields:       "knowledge.user_id", "knowledge.plan", "metadata.channel", "payload.wamid"
//   - Nested maps:      "knowledge.address.city"
//
// Evaluation order: BlockList first, then AllowList.
// Empty field value (field missing or resolves to "") → rule is skipped (Pulse passes).
type Guard struct {
	Field     string
	BlockList []string
	// AllowList, when non-empty, restricts to only these values. Evaluated after BlockList.
	AllowList []string
}

// CompiledGuard is the O(1)-lookup form of Guard, built once by Link.Build().
// Use IsBlocked/IsAllowed instead of iterating the original slices.
type CompiledGuard struct {
	Field    string
	blockSet map[string]struct{}
	allowSet map[string]struct{}
}

func compileGuard(r Guard) CompiledGuard {
	c := CompiledGuard{Field: r.Field}
	if len(r.BlockList) > 0 {
		c.blockSet = make(map[string]struct{}, len(r.BlockList))
		for _, v := range r.BlockList {
			c.blockSet[v] = struct{}{}
		}
	}
	if len(r.AllowList) > 0 {
		c.allowSet = make(map[string]struct{}, len(r.AllowList))
		for _, v := range r.AllowList {
			c.allowSet[v] = struct{}{}
		}
	}
	return c
}

// IsBlocked reports whether value is in the block list.
func (r *CompiledGuard) IsBlocked(value string) bool {
	_, ok := r.blockSet[value]
	return ok
}

// IsNotAllowed reports whether value is absent from a non-empty allow list.
func (r *CompiledGuard) IsNotAllowed(value string) bool {
	if len(r.allowSet) == 0 {
		return false
	}
	_, ok := r.allowSet[value]
	return !ok
}

// Link defines how a specific event_key should be processed:
// InboundConverter → Scouts → Pathfinder → Spirit selection.
//
// Example:
//
//	config := NewLink("whatsapp_message").
//	    WithInboundConverter("whatsapp_360dialog").
//	    WithScouts("user_lookup", "conversation_history").
//	    WithPathfinder("llm_pathfinder").
//	    WithSpirits("support_spirit", "sales_spirit", "technical_spirit").
//	    WithDefaultSpirit("support_spirit").
//	    Build()
type Link struct {
	EventType            string
	InboundConverterName string

	// RequireScouts lists Scouts to run before Spirit selection (executed in order).
	RequireScouts []string

	// PathfinderName specifies the Pathfinder for Spirit selection.
	// Only used when len(AllowedSpirits) > 1.
	PathfinderName string

	// AllowedSpirits lists Spirits eligible to handle this Pulse.
	// Empty → DefaultSpirit. Single → direct (no Pathfinder). Multiple → Pathfinder selects.
	AllowedSpirits []string

	// DefaultSpirit is the fallback Spirit when AllowedSpirits is empty,
	// Pathfinder returns empty, or the selected Spirit is not found.
	DefaultSpirit string

	// VoiceName is the Voice used for automatic response delivery.
	// "whatsapp": auto-sends if no send_whatsapp_message Action was called.
	// "http": no auto-response (response is in the HTTP body).
	// Empty: no automatic delivery.
	VoiceName string

	// ChannelName is set by WithChannel and mirrors both InboundConverterName and VoiceName.
	// Conversational Spirits should use WithChannel instead of separate WithReceptor/WithVoice calls.
	ChannelName string

	IngestionTimeout  time.Duration // Default: 5s
	ProcessingTimeout time.Duration // Default: 60s

	// Guards defines per-Pulse allow/block rules evaluated against Pulse fields.
	// All rules must pass (AND). Missing field values skip the rule.
	Guards []Guard

	// CompiledRules is the O(1)-lookup form of Guards, populated by Build().
	CompiledRules []CompiledGuard

	Metadata map[string]any
}

func NewLink(eventType string) *Link {
	return &Link{
		EventType: eventType,
		Metadata:  make(map[string]any),
	}
}

func (e *Link) WithInboundConverter(name string) *Link {
	e.InboundConverterName = name
	return e
}

func (e *Link) WithScouts(enrichers ...string) *Link {
	e.RequireScouts = enrichers
	return e
}

func (e *Link) AddScout(enricher string) *Link {
	e.RequireScouts = append(e.RequireScouts, enricher)
	return e
}

func (e *Link) WithPathfinder(name string) *Link {
	e.PathfinderName = name
	return e
}

func (e *Link) WithSpirits(spirits ...string) *Link {
	e.AllowedSpirits = spirits
	return e
}

func (e *Link) AddSpirit(spirit string) *Link {
	e.AllowedSpirits = append(e.AllowedSpirits, spirit)
	return e
}

func (e *Link) WithDefaultSpirit(spirit string) *Link {
	e.DefaultSpirit = spirit
	return e
}

func (e *Link) WithVoice(channelName string) *Link {
	e.VoiceName = channelName
	return e
}

// WithChannel pairs the inbound Receptor and outbound Voice under a single channel name.
// Use this for conversational Spirits (WhatsApp, Telegram, etc.) where
// the response returns through the same channel that delivered the message.
func (e *Link) WithChannel(name string) *Link {
	e.ChannelName = name
	e.InboundConverterName = name
	e.VoiceName = name
	return e
}

func (e *Link) WithMetadata(metadata map[string]any) *Link {
	e.Metadata = metadata
	return e
}

func (e *Link) AddMetadata(key string, value any) *Link {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}

func (e *Link) WithGuards(rules ...Guard) *Link {
	e.Guards = rules
	return e
}

func (e *Link) AddGuard(rule Guard) *Link {
	e.Guards = append(e.Guards, rule)
	return e
}

func (e *Link) Build() *Link {
	e.CompiledRules = make([]CompiledGuard, len(e.Guards))
	for i, r := range e.Guards {
		e.CompiledRules[i] = compileGuard(r)
	}
	return e
}

// IsSpiritAllowed reports whether the given Spirit name is permitted to handle this Link.
// When AllowedSpirits is empty, all Spirits are allowed (returns true).
func (e *Link) IsSpiritAllowed(spiritName string) bool {
	if len(e.AllowedSpirits) == 0 {
		return true
	}
	for _, allowed := range e.AllowedSpirits {
		if allowed == spiritName {
			return true
		}
	}
	return false
}

// HasMultipleSpirits reports whether more than one Spirit is configured on this Link,
// indicating that a Pathfinder is needed to select among them.
func (e *Link) HasMultipleSpirits() bool {
	return len(e.AllowedSpirits) > 1
}

// HasSingleSpirit reports whether exactly one Spirit is configured on this Link.
// When true, Spirit selection bypasses the Pathfinder and uses that Spirit directly.
func (e *Link) HasSingleSpirit() bool {
	return len(e.AllowedSpirits) == 1
}

// GetSingleSpirit returns the sole configured Spirit name when HasSingleSpirit is true.
// Returns "" if there is not exactly one Spirit in AllowedSpirits.
func (e *Link) GetSingleSpirit() string {
	if e.HasSingleSpirit() {
		return e.AllowedSpirits[0]
	}
	return ""
}

func (e *Link) WithIngestionTimeout(timeout time.Duration) *Link {
	e.IngestionTimeout = timeout
	return e
}

func (e *Link) WithProcessingTimeout(timeout time.Duration) *Link {
	e.ProcessingTimeout = timeout
	return e
}

func (e *Link) GetIngestionTimeout() time.Duration {
	if e.IngestionTimeout == 0 {
		return 5 * time.Second
	}
	return e.IngestionTimeout
}

func (e *Link) GetProcessingTimeout() time.Duration {
	if e.ProcessingTimeout == 0 {
		return 60 * time.Second
	}
	return e.ProcessingTimeout
}
