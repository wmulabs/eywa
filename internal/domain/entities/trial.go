package entities

import "time"

type TrialCase struct {
	ID          string                 `json:"id" yaml:"id"`
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	EventType   string                 `json:"event_type" yaml:"event_type"`
	UserMessage string                 `json:"user_message" yaml:"user_message"`
	Knowledge   map[string]any `json:"knowledge,omitempty" yaml:"knowledge,omitempty"`
	Expect      TrialExpectations      `json:"expect" yaml:"expect"`
}

type TrialExpectations struct {
	OutputContains    []string        `json:"output_contains,omitempty" yaml:"output_contains,omitempty"`
	OutputNotContains []string        `json:"output_not_contains,omitempty" yaml:"output_not_contains,omitempty"`
	ActionsUsed       []string        `json:"actions_used,omitempty" yaml:"actions_used,omitempty"`
	ActionsNotUsed    []string        `json:"actions_not_used,omitempty" yaml:"actions_not_used,omitempty"`
	ActionSequence    []string        `json:"action_sequence,omitempty" yaml:"action_sequence,omitempty"`
	MaxIterations     int             `json:"max_iterations,omitempty" yaml:"max_iterations,omitempty"`
	MaxTokens         int             `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	MaxLatencyMs      int             `json:"max_latency_ms,omitempty" yaml:"max_latency_ms,omitempty"`
	LLMJudge          *LLMJudgeExpect `json:"llm_judge,omitempty" yaml:"llm_judge,omitempty"`
}

type LLMJudgeExpect struct {
	Criteria string      `json:"criteria" yaml:"criteria"`
	MinScore float64     `json:"min_score" yaml:"min_score"`
	Model    SpiritModel `json:"model" yaml:"model"`
}

type TrialSuite struct {
	ID    string      `json:"id" yaml:"id"`
	Name  string      `json:"name" yaml:"name"`
	Cases []TrialCase `json:"cases" yaml:"cases"`
}

type TrialResult struct {
	CaseID     string             `json:"case_id"`
	Passed     bool               `json:"passed"`
	Scores     map[string]float64 `json:"scores"`
	Failures   []string           `json:"failures,omitempty"`
	Response   Response           `json:"response"`
	DurationMs int64              `json:"duration_ms"`
}

type TrialReport struct {
	SuiteID  string        `json:"suite_id"`
	RunAt    time.Time     `json:"run_at"`
	PassRate float64       `json:"pass_rate"`
	Results  []TrialResult `json:"results"`
}
