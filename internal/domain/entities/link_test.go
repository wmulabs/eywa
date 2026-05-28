package entities

import (
	"testing"
	"time"
)

func TestCompiledGuard_IsBlocked_WhenValueInBlockList(t *testing.T) {
	guard := compileGuard(Guard{
		BlockList: []string{"spam", "admin", "bot"},
	})

	if !guard.IsBlocked("spam") {
		t.Error("expected spam to be blocked")
	}
}

func TestCompiledGuard_IsBlocked_WhenValueNotInBlockList(t *testing.T) {
	guard := compileGuard(Guard{
		BlockList: []string{"spam", "admin"},
	})

	if guard.IsBlocked("api") {
		t.Error("expected api to not be blocked")
	}
}

func TestCompiledGuard_IsBlocked_WhenBlockListEmpty(t *testing.T) {
	guard := compileGuard(Guard{})

	if guard.IsBlocked("anything") {
		t.Error("expected anything to not be blocked when list is empty")
	}
}

func TestCompiledGuard_IsNotAllowed_WhenAllowListEmpty(t *testing.T) {
	guard := compileGuard(Guard{})

	if guard.IsNotAllowed("anything") {
		t.Error("expected anything to be allowed when allow list is empty")
	}
}

func TestCompiledGuard_IsNotAllowed_WhenValueNotInAllowList(t *testing.T) {
	guard := compileGuard(Guard{
		AllowList: []string{"api", "web"},
	})

	if !guard.IsNotAllowed("bot") {
		t.Error("expected bot to not be allowed")
	}
}

func TestCompiledGuard_IsNotAllowed_WhenValueInAllowList(t *testing.T) {
	guard := compileGuard(Guard{
		AllowList: []string{"api", "web"},
	})

	if guard.IsNotAllowed("api") {
		t.Error("expected api to be allowed")
	}
}

func TestLink_NewLink_SetsEventType(t *testing.T) {
	link := NewLink("user_message")

	if link.EventType != "user_message" {
		t.Errorf("expected EventType 'user_message', got '%s'", link.EventType)
	}
}

func TestLink_WithChannel_SetsInboundConverterNameAndVoiceAndChannelName(t *testing.T) {
	link := NewLink("test_event").WithChannel("whatsapp")

	if link.InboundConverterName != "whatsapp" {
		t.Errorf("expected InboundConverterName 'whatsapp', got '%s'", link.InboundConverterName)
	}
	if link.VoiceName != "whatsapp" {
		t.Errorf("expected VoiceName 'whatsapp', got '%s'", link.VoiceName)
	}
	if link.ChannelName != "whatsapp" {
		t.Errorf("expected ChannelName 'whatsapp', got '%s'", link.ChannelName)
	}
}

func TestLink_WithSpirits_SetsAllowedSpirits(t *testing.T) {
	link := NewLink("test_event").WithSpirits("a", "b", "c")

	if len(link.AllowedSpirits) != 3 {
		t.Errorf("expected 3 spirits, got %d", len(link.AllowedSpirits))
	}
	if link.AllowedSpirits[0] != "a" || link.AllowedSpirits[1] != "b" || link.AllowedSpirits[2] != "c" {
		t.Errorf("expected spirits [a, b, c], got %v", link.AllowedSpirits)
	}
}

func TestLink_AddSpirit_AppendsSpiritToExistingList(t *testing.T) {
	link := NewLink("test_event").
		WithSpirits("a").
		AddSpirit("b")

	if len(link.AllowedSpirits) != 2 {
		t.Errorf("expected 2 spirits, got %d", len(link.AllowedSpirits))
	}
	if link.AllowedSpirits[0] != "a" || link.AllowedSpirits[1] != "b" {
		t.Errorf("expected spirits [a, b], got %v", link.AllowedSpirits)
	}
}

func TestLink_IsSpiritAllowed_WhenAllowedSpiritsEmpty(t *testing.T) {
	link := NewLink("test_event")

	if !link.IsSpiritAllowed("any") {
		t.Error("expected any spirit to be allowed when list is empty")
	}
}

func TestLink_IsSpiritAllowed_WhenSpiritInList(t *testing.T) {
	link := NewLink("test_event").WithSpirits("a", "b")

	if !link.IsSpiritAllowed("b") {
		t.Error("expected b to be allowed")
	}
}

func TestLink_IsSpiritAllowed_WhenSpiritNotInList(t *testing.T) {
	link := NewLink("test_event").WithSpirits("a", "b")

	if link.IsSpiritAllowed("c") {
		t.Error("expected c to not be allowed")
	}
}

func TestLink_HasMultipleSpirits_WhenTwoSpirits(t *testing.T) {
	link := NewLink("test_event").WithSpirits("a", "b")

	if !link.HasMultipleSpirits() {
		t.Error("expected HasMultipleSpirits to be true for 2 spirits")
	}
}

func TestLink_HasSingleSpirit_WhenOneSpirit(t *testing.T) {
	link := NewLink("test_event").WithSpirits("a")

	if !link.HasSingleSpirit() {
		t.Error("expected HasSingleSpirit to be true for 1 spirit")
	}
}

func TestLink_GetSingleSpirit_WhenSingleSpirit(t *testing.T) {
	link := NewLink("test_event").WithSpirits("support")

	spirit := link.GetSingleSpirit()
	if spirit != "support" {
		t.Errorf("expected spirit 'support', got '%s'", spirit)
	}
}

func TestLink_GetSingleSpirit_WhenMultipleSpirits(t *testing.T) {
	link := NewLink("test_event").WithSpirits("a", "b")

	spirit := link.GetSingleSpirit()
	if spirit != "" {
		t.Errorf("expected empty string for multiple spirits, got '%s'", spirit)
	}
}

func TestLink_GetIngestionTimeout_Default(t *testing.T) {
	link := NewLink("test_event")

	timeout := link.GetIngestionTimeout()
	if timeout != 5*time.Second {
		t.Errorf("expected 5s default timeout, got %v", timeout)
	}
}

func TestLink_GetIngestionTimeout_Custom(t *testing.T) {
	link := NewLink("test_event").WithIngestionTimeout(10 * time.Second)

	timeout := link.GetIngestionTimeout()
	if timeout != 10*time.Second {
		t.Errorf("expected 10s timeout, got %v", timeout)
	}
}

func TestLink_GetProcessingTimeout_Default(t *testing.T) {
	link := NewLink("test_event")

	timeout := link.GetProcessingTimeout()
	if timeout != 60*time.Second {
		t.Errorf("expected 60s default timeout, got %v", timeout)
	}
}

func TestLink_GetProcessingTimeout_Custom(t *testing.T) {
	link := NewLink("test_event").WithProcessingTimeout(30 * time.Second)

	timeout := link.GetProcessingTimeout()
	if timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", timeout)
	}
}

func TestLink_Build_CompilesGuardsToCompiledRules(t *testing.T) {
	link := NewLink("test_event").
		WithGuards(Guard{
			Field:     "type",
			BlockList: []string{"bot"},
		}).
		Build()

	if len(link.CompiledRules) != 1 {
		t.Errorf("expected 1 compiled rule, got %d", len(link.CompiledRules))
	}
	if !link.CompiledRules[0].IsBlocked("bot") {
		t.Error("expected compiled rule to block 'bot'")
	}
}

func TestLink_AddMetadata_SetsKeyValue(t *testing.T) {
	link := NewLink("test_event").AddMetadata("key", "val")

	value, ok := link.Metadata["key"]
	if !ok {
		t.Error("expected metadata key 'key' to exist")
	}
	if value != "val" {
		t.Errorf("expected metadata value 'val', got '%v'", value)
	}
}

func TestLink_MetadataInitialized_InNewLink(t *testing.T) {
	link := NewLink("test_event")

	if link.Metadata == nil {
		t.Error("expected Metadata to be initialized in NewLink")
	}
}

func TestLink_WithInboundConverter_SetsConverterName(t *testing.T) {
	link := NewLink("test_event").WithInboundConverter("whatsapp_360dialog")

	if link.InboundConverterName != "whatsapp_360dialog" {
		t.Errorf("expected InboundConverterName 'whatsapp_360dialog', got '%s'", link.InboundConverterName)
	}
}

func TestLink_WithScouts_SetsRequireScouts(t *testing.T) {
	link := NewLink("test_event").WithScouts("user_lookup", "history")

	if len(link.RequireScouts) != 2 {
		t.Errorf("expected 2 scouts, got %d", len(link.RequireScouts))
	}
	if link.RequireScouts[0] != "user_lookup" || link.RequireScouts[1] != "history" {
		t.Errorf("expected scouts [user_lookup, history], got %v", link.RequireScouts)
	}
}

func TestLink_AddScout_AppendsScoutToExistingList(t *testing.T) {
	link := NewLink("test_event").
		WithScouts("user_lookup").
		AddScout("history")

	if len(link.RequireScouts) != 2 {
		t.Errorf("expected 2 scouts, got %d", len(link.RequireScouts))
	}
	if link.RequireScouts[1] != "history" {
		t.Errorf("expected second scout 'history', got '%s'", link.RequireScouts[1])
	}
}

func TestLink_WithPathfinder_SetsPathfinderName(t *testing.T) {
	link := NewLink("test_event").WithPathfinder("llm_pathfinder")

	if link.PathfinderName != "llm_pathfinder" {
		t.Errorf("expected PathfinderName 'llm_pathfinder', got '%s'", link.PathfinderName)
	}
}

func TestLink_WithDefaultSpirit_SetsDefaultSpirit(t *testing.T) {
	link := NewLink("test_event").WithDefaultSpirit("support_spirit")

	if link.DefaultSpirit != "support_spirit" {
		t.Errorf("expected DefaultSpirit 'support_spirit', got '%s'", link.DefaultSpirit)
	}
}

func TestLink_WithVoice_SetsVoiceName(t *testing.T) {
	link := NewLink("test_event").WithVoice("whatsapp")

	if link.VoiceName != "whatsapp" {
		t.Errorf("expected VoiceName 'whatsapp', got '%s'", link.VoiceName)
	}
}

func TestLink_WithMetadata_ReplacesEntireMetadataMap(t *testing.T) {
	link := NewLink("test_event").
		AddMetadata("old_key", "old_val").
		WithMetadata(map[string]any{"new_key": "new_val"})

	if _, ok := link.Metadata["old_key"]; ok {
		t.Error("expected old_key to be removed after WithMetadata")
	}
	if link.Metadata["new_key"] != "new_val" {
		t.Errorf("expected new_key 'new_val', got '%v'", link.Metadata["new_key"])
	}
}

func TestLink_AddGuard_AppendsGuardToExistingList(t *testing.T) {
	link := NewLink("test_event").
		WithGuards(Guard{Field: "source", BlockList: []string{"bot"}}).
		AddGuard(Guard{Field: "type", AllowList: []string{"api"}})

	if len(link.Guards) != 2 {
		t.Errorf("expected 2 guards, got %d", len(link.Guards))
	}
	if link.Guards[1].Field != "type" {
		t.Errorf("expected second guard field 'type', got '%s'", link.Guards[1].Field)
	}
}

func TestLink_AddMetadata_NilInit(t *testing.T) {
	l := &Link{} // Metadata is nil
	l.AddMetadata("k", "v")
	if l.Metadata == nil {
		t.Fatal("Metadata still nil after AddMetadata")
	}
	if l.Metadata["k"] != "v" {
		t.Errorf("Metadata[\"k\"] = %v, want %q", l.Metadata["k"], "v")
	}
}

func TestLink_AddMetadata_ExistingMap(t *testing.T) {
	l := &Link{Metadata: map[string]any{"a": "1"}}
	l.AddMetadata("b", "2")
	if l.Metadata["b"] != "2" {
		t.Errorf("Metadata[\"b\"] = %v, want %q", l.Metadata["b"], "2")
	}
	if l.Metadata["a"] != "1" {
		t.Errorf("Metadata[\"a\"] must be preserved, got %v", l.Metadata["a"])
	}
}
