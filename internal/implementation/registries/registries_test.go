package registries

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- minimal stubs ---

type stubVoice struct{ name string }

var _ ports.Voice = (*stubVoice)(nil)

func (v *stubVoice) GetName() string         { return v.name }
func (v *stubVoice) ShouldAutoRespond() bool { return false }
func (v *stubVoice) SendResponse(_ context.Context, _ *entities.Pulse, _ string) error {
	return nil
}
func (v *stubVoice) GetChannelMetadata(_ *entities.Pulse) map[string]any { return nil }

type stubRouter struct{ name string }

var _ ports.LogicRouter = (*stubRouter)(nil)

func (r *stubRouter) GetName() string                                   { return r.name }
func (r *stubRouter) Route(_ context.Context, _ *entities.Pulse) string { return "" }

type stubPathfinder struct{ name string }

var _ ports.Pathfinder = (*stubPathfinder)(nil)

func (p *stubPathfinder) GetName() string { return p.name }
func (p *stubPathfinder) SelectSpirit(_ context.Context, _ *entities.Pulse, _ []string) string {
	return ""
}

type stubConduit struct{ name string }

var _ ports.Conduit = (*stubConduit)(nil)

func (c *stubConduit) GetName() string                 { return c.name }
func (c *stubConduit) Connect(_ context.Context) error { return nil }
func (c *stubConduit) ListTools(_ context.Context) ([]entities.ActionDefinition, error) {
	return nil, nil
}
func (c *stubConduit) Call(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", nil
}
func (c *stubConduit) Close() error { return nil }

type stubScout struct {
	name       string
	applicable bool
	harvestErr error
	harvested  bool
}

var _ ports.Scout = (*stubScout)(nil)

func (s *stubScout) GetName() string                     { return s.name }
func (s *stubScout) IsApplicable(_ *entities.Pulse) bool { return s.applicable }
func (s *stubScout) Harvest(_ context.Context, _ *entities.Pulse) error {
	s.harvested = true
	return s.harvestErr
}

type stubAction struct {
	name        string
	description string
	result      string
	validateErr error
	execErr     error
}

var _ ports.Action = (*stubAction)(nil)

func (a *stubAction) GetName() string                   { return a.name }
func (a *stubAction) GetDescription() string            { return a.description }
func (a *stubAction) GetParameters() map[string]any     { return map[string]any{} }
func (a *stubAction) IsCritical() bool                  { return false }
func (a *stubAction) GetCategory() ports.ActionCategory { return ports.ActionGeneral }
func (a *stubAction) Validate(_ map[string]any) error   { return a.validateErr }
func (a *stubAction) Execute(_ context.Context, _ map[string]any) (string, error) {
	return a.result, a.execErr
}

// =============================================================================
// VoiceRegistry
// =============================================================================

func TestVoiceRegistry_Register_Success(t *testing.T) {
	reg := NewVoiceRegistry()
	if err := reg.Register(&stubVoice{name: "whatsapp"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVoiceRegistry_Register_Duplicate_ReturnsError(t *testing.T) {
	reg := NewVoiceRegistry()
	_ = reg.Register(&stubVoice{name: "whatsapp"})
	if err := reg.Register(&stubVoice{name: "whatsapp"}); err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestVoiceRegistry_RegisterMultiple(t *testing.T) {
	reg := NewVoiceRegistry()
	if err := reg.RegisterMultiple(&stubVoice{name: "ch1"}, &stubVoice{name: "ch2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVoiceRegistry_RegisterMultiple_Duplicate_ReturnsError(t *testing.T) {
	reg := NewVoiceRegistry()
	_ = reg.Register(&stubVoice{name: "ch1"})
	if err := reg.RegisterMultiple(&stubVoice{name: "ch1"}); err == nil {
		t.Error("expected error on duplicate in RegisterMultiple")
	}
}

func TestVoiceRegistry_Get_Found(t *testing.T) {
	reg := NewVoiceRegistry()
	_ = reg.Register(&stubVoice{name: "whatsapp"})
	ch, err := reg.Get("whatsapp")
	if err != nil || ch == nil {
		t.Fatalf("expected channel, got err=%v", err)
	}
}

func TestVoiceRegistry_Get_NotFound(t *testing.T) {
	reg := NewVoiceRegistry()
	_, err := reg.Get("missing")
	if err == nil {
		t.Error("expected error for missing channel")
	}
}

func TestVoiceRegistry_Has(t *testing.T) {
	reg := NewVoiceRegistry()
	_ = reg.Register(&stubVoice{name: "telegram"})
	if !reg.Has("telegram") {
		t.Error("expected Has=true for registered channel")
	}
	if reg.Has("missing") {
		t.Error("expected Has=false for unregistered channel")
	}
}

func TestVoiceRegistry_List(t *testing.T) {
	reg := NewVoiceRegistry()
	_ = reg.Register(&stubVoice{name: "ch1"})
	_ = reg.Register(&stubVoice{name: "ch2"})
	list := reg.List()
	if len(list) != 2 {
		t.Errorf("expected 2 channels, got %d", len(list))
	}
}

// =============================================================================
// LogicRouterRegistry
// =============================================================================

func TestLogicRouterRegistry_Register_Success(t *testing.T) {
	reg := NewLogicRouterRegistry()
	if err := reg.Register(&stubRouter{name: "my-router"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogicRouterRegistry_Register_Duplicate_ReturnsError(t *testing.T) {
	reg := NewLogicRouterRegistry()
	_ = reg.Register(&stubRouter{name: "r1"})
	if err := reg.Register(&stubRouter{name: "r1"}); err == nil {
		t.Error("expected error for duplicate")
	}
}

func TestLogicRouterRegistry_Get_Found(t *testing.T) {
	reg := NewLogicRouterRegistry()
	_ = reg.Register(&stubRouter{name: "r1"})
	router, err := reg.Get("r1")
	if err != nil || router == nil {
		t.Fatalf("expected router, got err=%v", err)
	}
}

func TestLogicRouterRegistry_Get_NotFound(t *testing.T) {
	reg := NewLogicRouterRegistry()
	_, err := reg.Get("missing")
	if err == nil {
		t.Error("expected error for missing router")
	}
}

func TestLogicRouterRegistry_List(t *testing.T) {
	reg := NewLogicRouterRegistry()
	_ = reg.Register(&stubRouter{name: "r1"})
	_ = reg.Register(&stubRouter{name: "r2"})
	if len(reg.List()) != 2 {
		t.Errorf("expected 2 routers in list")
	}
}

// =============================================================================
// PathfinderRegistry
// =============================================================================

func TestPathfinderRegistry_Register_Success(t *testing.T) {
	reg := NewPathfinderRegistry()
	if err := reg.Register(&stubPathfinder{name: "pf1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPathfinderRegistry_Register_Duplicate_ReturnsError(t *testing.T) {
	reg := NewPathfinderRegistry()
	_ = reg.Register(&stubPathfinder{name: "pf1"})
	if err := reg.Register(&stubPathfinder{name: "pf1"}); err == nil {
		t.Error("expected error for duplicate")
	}
}

func TestPathfinderRegistry_RegisterMultiple(t *testing.T) {
	reg := NewPathfinderRegistry()
	if err := reg.RegisterMultiple(&stubPathfinder{name: "a"}, &stubPathfinder{name: "b"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPathfinderRegistry_RegisterMultiple_Duplicate(t *testing.T) {
	reg := NewPathfinderRegistry()
	_ = reg.Register(&stubPathfinder{name: "a"})
	if err := reg.RegisterMultiple(&stubPathfinder{name: "a"}); err == nil {
		t.Error("expected error on duplicate")
	}
}

func TestPathfinderRegistry_Get_Found(t *testing.T) {
	reg := NewPathfinderRegistry()
	_ = reg.Register(&stubPathfinder{name: "pf1"})
	pf := reg.Get("pf1")
	if pf == nil {
		t.Error("expected non-nil pathfinder")
	}
}

func TestPathfinderRegistry_Get_NotFound(t *testing.T) {
	reg := NewPathfinderRegistry()
	if reg.Get("missing") != nil {
		t.Error("expected nil for missing pathfinder")
	}
}

func TestPathfinderRegistry_List(t *testing.T) {
	reg := NewPathfinderRegistry()
	_ = reg.Register(&stubPathfinder{name: "pf1"})
	if len(reg.List()) != 1 {
		t.Errorf("expected 1 in list")
	}
}

// =============================================================================
// ConduitRegistry
// =============================================================================

func TestConduitRegistry_Register_Success(t *testing.T) {
	reg := NewConduitRegistry()
	if err := reg.Register(&stubConduit{name: "mcp-tool"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConduitRegistry_Register_Duplicate_ReturnsError(t *testing.T) {
	reg := NewConduitRegistry()
	_ = reg.Register(&stubConduit{name: "c1"})
	if err := reg.Register(&stubConduit{name: "c1"}); err == nil {
		t.Error("expected error for duplicate")
	}
}

func TestConduitRegistry_Get_Found(t *testing.T) {
	reg := NewConduitRegistry()
	_ = reg.Register(&stubConduit{name: "c1"})
	c, err := reg.Get("c1")
	if err != nil || c == nil {
		t.Fatalf("expected conduit, got err=%v", err)
	}
}

func TestConduitRegistry_Get_NotFound(t *testing.T) {
	reg := NewConduitRegistry()
	_, err := reg.Get("missing")
	if err == nil {
		t.Error("expected error for missing conduit")
	}
}

func TestConduitRegistry_ListAll(t *testing.T) {
	reg := NewConduitRegistry()
	_ = reg.Register(&stubConduit{name: "c1"})
	_ = reg.Register(&stubConduit{name: "c2"})
	if len(reg.ListAll()) != 2 {
		t.Errorf("expected 2 conduits")
	}
}

// =============================================================================
// ScoutRegistry
// =============================================================================

func TestScoutRegistry_Register_Success(t *testing.T) {
	reg := NewScoutRegistry()
	if err := reg.Register(&stubScout{name: "s1", applicable: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScoutRegistry_Register_Duplicate_ReturnsError(t *testing.T) {
	reg := NewScoutRegistry()
	_ = reg.Register(&stubScout{name: "s1"})
	if err := reg.Register(&stubScout{name: "s1"}); err == nil {
		t.Error("expected error for duplicate")
	}
}

func TestScoutRegistry_RegisterMultiple(t *testing.T) {
	reg := NewScoutRegistry()
	if err := reg.RegisterMultiple(&stubScout{name: "s1"}, &stubScout{name: "s2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScoutRegistry_RegisterMultiple_Duplicate(t *testing.T) {
	reg := NewScoutRegistry()
	_ = reg.Register(&stubScout{name: "s1"})
	if err := reg.RegisterMultiple(&stubScout{name: "s1"}); err == nil {
		t.Error("expected error on duplicate")
	}
}

func TestScoutRegistry_Harvest_ApplicableScout_Runs(t *testing.T) {
	reg := NewScoutRegistry()
	s1 := &stubScout{name: "s1", applicable: true}
	_ = reg.Register(s1)

	event := &entities.Pulse{MemoryKey: "user:1"}
	if err := reg.Harvest(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s1.harvested {
		t.Error("expected applicable scout to be called")
	}
}

func TestScoutRegistry_Harvest_NotApplicable_Skipped(t *testing.T) {
	reg := NewScoutRegistry()
	s1 := &stubScout{name: "s1", applicable: false}
	_ = reg.Register(s1)

	event := &entities.Pulse{MemoryKey: "user:1"}
	if err := reg.Harvest(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s1.harvested {
		t.Error("expected not-applicable scout to be skipped")
	}
}

func TestScoutRegistry_Harvest_ScoutError_ReturnsError(t *testing.T) {
	reg := NewScoutRegistry()
	_ = reg.Register(&stubScout{name: "s1", applicable: true, harvestErr: errors.New("harvest failed")})

	event := &entities.Pulse{MemoryKey: "user:1"}
	if err := reg.Harvest(context.Background(), event); err == nil {
		t.Error("expected error when scout fails")
	}
}

func TestScoutRegistry_Harvest_MemoryKeyChanged_UpdatesMetadata(t *testing.T) {
	reg := NewScoutRegistry()
	s1 := &stubScout{name: "memory-scout", applicable: true}
	// We want the scout to change the MemoryKey
	_ = reg.Register(&memoryKeyChangingScout{name: "changer", newKey: "user:new"})

	event := &entities.Pulse{MemoryKey: "user:old", Metadata: map[string]any{}}
	if err := reg.Harvest(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = s1
	if event.MemoryKey != "user:new" {
		t.Errorf("expected memory key updated to user:new, got %s", event.MemoryKey)
	}
	if event.Metadata["memory_key_updated_by"] == nil {
		t.Error("expected memory_key_updated_by metadata set")
	}
}

type memoryKeyChangingScout struct {
	name   string
	newKey string
}

var _ ports.Scout = (*memoryKeyChangingScout)(nil)

func (s *memoryKeyChangingScout) GetName() string                     { return s.name }
func (s *memoryKeyChangingScout) IsApplicable(_ *entities.Pulse) bool { return true }
func (s *memoryKeyChangingScout) Harvest(_ context.Context, event *entities.Pulse) error {
	event.MemoryKey = s.newKey
	return nil
}

func TestScoutRegistry_GetScout_Found(t *testing.T) {
	reg := NewScoutRegistry()
	_ = reg.Register(&stubScout{name: "s1"})
	scout, err := reg.GetScout("s1")
	if err != nil || scout == nil {
		t.Fatalf("expected scout, got err=%v", err)
	}
}

func TestScoutRegistry_GetScout_NotFound(t *testing.T) {
	reg := NewScoutRegistry()
	_, err := reg.GetScout("missing")
	if err == nil {
		t.Error("expected error for missing scout")
	}
}

func TestScoutRegistry_GetMultiple_Found(t *testing.T) {
	reg := NewScoutRegistry()
	_ = reg.Register(&stubScout{name: "s1"})
	_ = reg.Register(&stubScout{name: "s2"})
	scouts, err := reg.GetMultiple([]string{"s1", "s2"})
	if err != nil || len(scouts) != 2 {
		t.Fatalf("expected 2 scouts, got %d, err=%v", len(scouts), err)
	}
}

func TestScoutRegistry_GetMultiple_Missing(t *testing.T) {
	reg := NewScoutRegistry()
	_, err := reg.GetMultiple([]string{"missing"})
	if err == nil {
		t.Error("expected error for missing scouts")
	}
}

func TestScoutRegistry_ListScouts(t *testing.T) {
	reg := NewScoutRegistry()
	_ = reg.Register(&stubScout{name: "s1"})
	_ = reg.Register(&stubScout{name: "s2"})
	names := reg.ListScouts()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

func TestScoutRegistry_List(t *testing.T) {
	reg := NewScoutRegistry()
	_ = reg.Register(&stubScout{name: "s1"})
	list := reg.List()
	if len(list) != 1 {
		t.Errorf("expected 1 scout, got %d", len(list))
	}
}

// =============================================================================
// ActionRegistry
// =============================================================================

func TestActionRegistry_Register_Success(t *testing.T) {
	reg := NewActionRegistry()
	if err := reg.Register(&stubAction{name: "send_email"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestActionRegistry_Register_Duplicate_ReturnsError(t *testing.T) {
	reg := NewActionRegistry()
	_ = reg.Register(&stubAction{name: "a1"})
	if err := reg.Register(&stubAction{name: "a1"}); err == nil {
		t.Error("expected error for duplicate")
	}
}

func TestActionRegistry_RegisterMultiple(t *testing.T) {
	reg := NewActionRegistry()
	if err := reg.RegisterMultiple(&stubAction{name: "a1"}, &stubAction{name: "a2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestActionRegistry_RegisterMultiple_Duplicate(t *testing.T) {
	reg := NewActionRegistry()
	_ = reg.Register(&stubAction{name: "a1"})
	if err := reg.RegisterMultiple(&stubAction{name: "a1"}); err == nil {
		t.Error("expected error on duplicate")
	}
}

func TestActionRegistry_Get_Found(t *testing.T) {
	reg := NewActionRegistry()
	_ = reg.Register(&stubAction{name: "a1"})
	a, err := reg.Get("a1")
	if err != nil || a == nil {
		t.Fatalf("expected action, got err=%v", err)
	}
}

func TestActionRegistry_Get_NotFound(t *testing.T) {
	reg := NewActionRegistry()
	_, err := reg.Get("missing")
	if err == nil {
		t.Error("expected error for missing action")
	}
}

func TestActionRegistry_GetMultiple_Found(t *testing.T) {
	reg := NewActionRegistry()
	_ = reg.Register(&stubAction{name: "a1"})
	_ = reg.Register(&stubAction{name: "a2"})
	actions, err := reg.GetMultiple([]string{"a1", "a2"})
	if err != nil || len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d, err=%v", len(actions), err)
	}
}

func TestActionRegistry_GetMultiple_Missing(t *testing.T) {
	reg := NewActionRegistry()
	_, err := reg.GetMultiple([]string{"missing"})
	if err == nil {
		t.Error("expected error for missing actions")
	}
}

func TestActionRegistry_List(t *testing.T) {
	reg := NewActionRegistry()
	_ = reg.Register(&stubAction{name: "a1"})
	if len(reg.List()) != 1 {
		t.Errorf("expected 1 action in list")
	}
}

func TestActionRegistry_Execute_Success(t *testing.T) {
	reg := NewActionRegistry()
	_ = reg.Register(&stubAction{name: "greet", result: "hello"})
	result, err := reg.Execute(context.Background(), "greet", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected result=hello, got %s", result)
	}
}

func TestActionRegistry_Execute_ActionNotFound(t *testing.T) {
	reg := NewActionRegistry()
	_, err := reg.Execute(context.Background(), "missing", map[string]any{})
	if err == nil {
		t.Error("expected error for missing action")
	}
}

func TestActionRegistry_Execute_ValidationError(t *testing.T) {
	reg := NewActionRegistry()
	_ = reg.Register(&stubAction{name: "a1", validateErr: errors.New("bad args")})
	_, err := reg.Execute(context.Background(), "a1", map[string]any{})
	if err == nil {
		t.Error("expected error on validation failure")
	}
}

func TestActionRegistry_Execute_ExecutionError(t *testing.T) {
	reg := NewActionRegistry()
	_ = reg.Register(&stubAction{name: "a1", execErr: errors.New("action failed")})
	_, err := reg.Execute(context.Background(), "a1", map[string]any{})
	if err == nil {
		t.Error("expected error on execution failure")
	}
}

func TestActionRegistry_GetActionDefinitions(t *testing.T) {
	reg := NewActionRegistry()
	_ = reg.Register(&stubAction{name: "a1", description: "does stuff"})
	_ = reg.Register(&stubAction{name: "a2", description: "does more"})
	defs, err := reg.GetActionDefinitions(context.Background(), []string{"a1", "a2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(defs))
	}
}

func TestActionRegistry_GetActionDefinitions_SomeMissing(t *testing.T) {
	reg := NewActionRegistry()
	_ = reg.Register(&stubAction{name: "a1"})
	// missing a2 — should log warning but not error
	defs, err := reg.GetActionDefinitions(context.Background(), []string{"a1", "missing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 1 {
		t.Errorf("expected 1 definition, got %d", len(defs))
	}
}

func TestActionRegistry_IsRegistered(t *testing.T) {
	reg := NewActionRegistry()
	_ = reg.Register(&stubAction{name: "a1"})
	if !reg.IsRegistered("a1") {
		t.Error("expected IsRegistered=true")
	}
	if reg.IsRegistered("missing") {
		t.Error("expected IsRegistered=false")
	}
}

func TestActionRegistry_ListRegistered(t *testing.T) {
	reg := NewActionRegistry()
	_ = reg.Register(&stubAction{name: "a1"})
	_ = reg.Register(&stubAction{name: "a2"})
	names := reg.ListRegistered()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}
