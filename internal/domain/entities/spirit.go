package entities

import "time"

type SpiritType string

const (
	SpiritTypeConversational SpiritType = "conversational"
	SpiritTypeExecutor       SpiritType = "executor"
	SpiritTypeNotifier       SpiritType = "notifier"
	SpiritTypeOrchestrator   SpiritType = "orchestrator"
)

type ConversationalConfig struct {
	CrossChannelMemory bool `bson:"cross_channel_memory,omitempty" json:"cross_channel_memory,omitempty"`
}

type ExecutorConfig struct {
	WithSessionMemory   bool `bson:"with_session_memory,omitempty" json:"with_session_memory,omitempty"`
	WithMediaProcessing bool `bson:"with_media_processing,omitempty" json:"with_media_processing,omitempty"`
}

type NotifierCondition struct {
	Field    string `bson:"field" json:"field"`
	Operator string `bson:"operator" json:"operator"`
	Value    any    `bson:"value,omitempty" json:"value,omitempty"`
}

type NotifierConfig struct {
	Mode       string              `bson:"mode" json:"mode"`
	Template   string              `bson:"template,omitempty" json:"template,omitempty"`
	Conditions []NotifierCondition `bson:"conditions,omitempty" json:"conditions,omitempty"`
}

type OrchestratorConfig struct {
	Mode            string   `bson:"mode" json:"mode"`
	SubSpirits      []string `bson:"sub_spirits,omitempty" json:"sub_spirits,omitempty"`
	LogicRouterName string   `bson:"logic_router_name,omitempty" json:"logic_router_name,omitempty"`
	MaxDepth        int      `bson:"max_depth,omitempty" json:"max_depth,omitempty"`
	ParallelSummon  bool     `bson:"parallel_summon,omitempty" json:"parallel_summon,omitempty"`
}

type AllowedAction struct {
	Name                string `bson:"name"                  json:"name"`
	IsCritical          *bool  `bson:"is_critical,omitempty" json:"is_critical,omitempty"`
	DescriptionOverride string `bson:"description,omitempty" json:"description,omitempty"`
}

type Spirit struct {
	ID                        string          `bson:"_id,omitempty" json:"id"`
	Name                      string          `bson:"name" json:"name"`
	Description               string          `bson:"description,omitempty" json:"description,omitempty"`
	Version                   int             `bson:"version" json:"version"`
	IsActive                  bool            `bson:"is_active" json:"is_active"`
	IsDeleted                 bool            `bson:"is_deleted" json:"is_deleted"`
	SystemPrompt              string          `bson:"system_prompt" json:"system_prompt"`
	Specialization            string          `bson:"specialization,omitempty" json:"specialization,omitempty"`
	AllowedActions            []AllowedAction `bson:"allowed_actions" json:"allowed_actions"`
	EnforceVoiceDelivery      bool            `bson:"enforce_voice_delivery,omitempty" json:"enforce_voice_delivery,omitempty"`
	VoiceDeliveryInstructions string          `bson:"voice_delivery_instructions,omitempty" json:"voice_delivery_instructions,omitempty"`
	BusinessErrorInstructions string          `bson:"business_error_instructions,omitempty" json:"business_error_instructions,omitempty"`
	ModelConfig               SpiritModel     `bson:"model_config" json:"model_config"`
	Metadata                  map[string]any  `bson:"metadata,omitempty" json:"metadata,omitempty"`

	Type                 SpiritType           `bson:"type,omitempty" json:"type,omitempty"`
	RequireScouts        []string             `bson:"require_scouts,omitempty" json:"require_scouts,omitempty"`
	LoreIDs              []string             `bson:"lore_ids,omitempty" json:"lore_ids,omitempty"`
	ConversationalConfig ConversationalConfig `bson:"conversational_config,omitempty" json:"conversational_config,omitempty"`
	ExecutorConfig       ExecutorConfig       `bson:"executor_config,omitempty" json:"executor_config,omitempty"`
	NotifierConfig       NotifierConfig       `bson:"notifier_config,omitempty" json:"notifier_config,omitempty"`
	OrchestratorConfig   OrchestratorConfig   `bson:"orchestrator_config,omitempty" json:"orchestrator_config,omitempty"`

	CreatedAt     time.Time  `bson:"created_at" json:"created_at"`
	CreatedBy     string     `bson:"created_by,omitempty" json:"created_by,omitempty"`
	UpdatedAt     time.Time  `bson:"updated_at,omitempty" json:"updated_at,omitempty"`
	UpdatedBy     string     `bson:"updated_by,omitempty" json:"updated_by,omitempty"`
	DeletedAt     *time.Time `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
	DeletedBy     string     `bson:"deleted_by,omitempty" json:"deleted_by,omitempty"`
	DeactivatedAt *time.Time `bson:"deactivated_at,omitempty" json:"deactivated_at,omitempty"`
	ChangeLog     string     `bson:"change_log,omitempty" json:"change_log,omitempty"`
}

func (s Spirit) IsConversational() bool {
	return s.Type == "" || s.Type == SpiritTypeConversational
}

func (s Spirit) IsExecutor() bool {
	return s.Type == SpiritTypeExecutor
}

func (s Spirit) IsNotifier() bool {
	return s.Type == SpiritTypeNotifier
}

func (s Spirit) IsOrchestrator() bool {
	return s.Type == SpiritTypeOrchestrator
}

func (s Spirit) NeedsSession() bool {
	return s.IsConversational() || s.IsOrchestrator() ||
		(s.IsExecutor() && s.ExecutorConfig.WithSessionMemory)
}

func (s Spirit) NeedsMessageCoalescing() bool {
	return s.IsConversational()
}

type SpiritModel struct {
	Provider    string         `bson:"provider" json:"provider"`
	Model       string         `bson:"model" json:"model"`
	Temperature float64        `bson:"temperature" json:"temperature"`
	MaxTokens   int            `bson:"max_tokens,omitempty" json:"max_tokens,omitempty"`
	TopP        float64        `bson:"top_p,omitempty" json:"top_p,omitempty"`
	ExtraConfig map[string]any `bson:"extra_config,omitempty" json:"extra_config,omitempty"`
}
