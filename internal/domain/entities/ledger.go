package entities

import "time"

type TokenBudget struct {
	SpiritName        string      `bson:"spirit_name" json:"spirit_name"`
	MonthlyTokenLimit int64       `bson:"monthly_token_limit" json:"monthly_token_limit"`
	OnExceed          string      `bson:"on_exceed" json:"on_exceed"` // "block"|"downgrade"|"alert"
	DowngradeModel    SpiritModel `bson:"downgrade_model,omitempty" json:"downgrade_model,omitempty"`
	AlertThreshold    float64     `bson:"alert_threshold,omitempty" json:"alert_threshold,omitempty"` // 0.0–1.0
	// DowngradeAtThreshold switches the Spirit to DowngradeModel once usage reaches AlertThreshold,
	// before the hard limit is hit — slowing the burn proactively. Requires AlertThreshold and
	// DowngradeModel to be set. Disabled by default; independent of OnExceed (which still governs
	// behaviour at the hard limit).
	DowngradeAtThreshold bool `bson:"downgrade_at_threshold,omitempty" json:"downgrade_at_threshold,omitempty"`
}

type LedgerEntry struct {
	SpiritName string    `bson:"spirit_name" json:"spirit_name"`
	Month      string    `bson:"month" json:"month"` // "2026-05"
	TokensUsed int64     `bson:"tokens_used" json:"tokens_used"`
	EstCostUSD float64   `bson:"est_cost_usd" json:"est_cost_usd"`
	UpdatedAt  time.Time `bson:"updated_at" json:"updated_at"`
}

type ModelRoutingRule struct {
	Name      string
	Condition ModelRoutingCondition
	Model     SpiritModel
}

// ModelRoutingCondition specifies when a ModelRoutingRule should apply to a Pulse.
// All non-zero fields must match (AND semantics). Zero-value fields are ignored.
//
// Note: ModelRoutingCondition fields carry no json or bson struct tags because they are
// used as in-memory routing config only and are never persisted to a database.
type ModelRoutingCondition struct {
	// InputLengthGte matches when len(Pulse.UserMessage) >= this value (character count, not tokens). 0 = no lower bound.
	InputLengthGte int
	// InputLengthLt matches when len(Pulse.UserMessage) < this value (character count, not tokens). 0 = no upper bound.
	InputLengthLt  int
	HasAttachments bool
	TopicPrefix    string
	EventType      string
	UserTier       string
}
