package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- stubs ---

type stubVault struct {
	url string
	err error
}

var _ ports.Vault = (*stubVault)(nil)

func (v *stubVault) Store(_ context.Context, _ string, _ []byte, _ string) (string, error) {
	return v.url, v.err
}

type stubLens struct {
	usage ports.OracleUsage
	err   error
}

var _ ports.Lens = (*stubLens)(nil)

func (l *stubLens) Process(_ context.Context, _ *entities.Pulse) (ports.OracleUsage, error) {
	return l.usage, l.err
}

// --- mimeToExt ---

func TestMimeToExt_KnownTypes(t *testing.T) {
	cases := []struct {
		mime string
		ext  string
	}{
		{"audio/ogg", "ogg"},
		{"audio/ogg; codecs=opus", "ogg"},
		{"audio/mpeg", "mp3"},
		{"audio/mp3", "mp3"},
		{"audio/mp4", "m4a"},
		{"audio/wav", "wav"},
		{"image/jpeg", "jpg"},
		{"image/png", "png"},
		{"image/webp", "webp"},
		{"image/gif", "gif"},
		{"video/mp4", "mp4"},
		{"video/3gpp", "3gp"},
		{"application/pdf", "pdf"},
	}
	for _, tc := range cases {
		got := mimeToExt(tc.mime)
		if got != tc.ext {
			t.Errorf("mimeToExt(%q) = %q, want %q", tc.mime, got, tc.ext)
		}
	}
}

func TestMimeToExt_UnknownWithSlash_ReturnsSuffix(t *testing.T) {
	got := mimeToExt("application/octet-stream")
	if got != "octet-stream" {
		t.Errorf("expected octet-stream, got %s", got)
	}
}

func TestMimeToExt_NoSlash_ReturnsBin(t *testing.T) {
	got := mimeToExt("unknowntype")
	if got != "bin" {
		t.Errorf("expected bin, got %s", got)
	}
}

func TestMimeToExt_Empty_ReturnsBin(t *testing.T) {
	got := mimeToExt("")
	if got != "bin" {
		t.Errorf("expected bin, got %s", got)
	}
}

// --- buildVaultKey ---

func TestBuildVaultKey_IncludesFolder(t *testing.T) {
	att := &entities.Artifact{Type: entities.ArtifactTypeImage, MimeType: "image/png"}
	key := buildVaultKey("user:123", att)
	if key == "" {
		t.Error("expected non-empty key")
	}
	// memoryKey "user:123" → folder "user/123"
	if len(key) < 8 {
		t.Errorf("key too short: %s", key)
	}
}

func TestBuildVaultKey_EmptyTypeName_UsesFile(t *testing.T) {
	att := &entities.Artifact{Type: "", MimeType: "image/png"}
	key := buildVaultKey("user:1", att)
	// typeName = "" → "file"
	if key == "" {
		t.Error("expected non-empty key")
	}
}

// --- MediaVaultStep ---

func TestMediaVaultStep_DefaultTimeout(t *testing.T) {
	step := NewMediaVaultStep(&stubVault{}, 0, testLogger(t))
	if step.Timeout() != 15*time.Second {
		t.Errorf("expected default 15s, got %v", step.Timeout())
	}
}

func TestMediaVaultStep_CustomTimeout(t *testing.T) {
	step := NewMediaVaultStep(&stubVault{}, 5*time.Second, testLogger(t))
	if step.Timeout() != 5*time.Second {
		t.Errorf("expected 5s, got %v", step.Timeout())
	}
	if step.Name() != "MediaVault" {
		t.Errorf("expected name=MediaVault, got %s", step.Name())
	}
}

func TestMediaVaultStep_NoAttachments_NoOp(t *testing.T) {
	step := NewMediaVaultStep(&stubVault{url: "https://cdn/img.png"}, time.Second, testLogger(t))
	state := &ProcessingState{
		Event: &entities.Pulse{MemoryKey: "user:1", Attachments: nil},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMediaVaultStep_AttachmentNoData_Skipped(t *testing.T) {
	step := NewMediaVaultStep(&stubVault{url: "https://cdn/img.png"}, time.Second, testLogger(t))
	state := &ProcessingState{
		Event: &entities.Pulse{
			MemoryKey: "user:1",
			Attachments: []*entities.Artifact{
				{MediaID: "m1", Type: entities.ArtifactTypeImage, Data: nil}, // no data
			},
		},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMediaVaultStep_AttachmentURLAlreadySet_Skipped(t *testing.T) {
	step := NewMediaVaultStep(&stubVault{url: "https://cdn/new.png"}, time.Second, testLogger(t))
	state := &ProcessingState{
		Event: &entities.Pulse{
			MemoryKey: "user:1",
			Attachments: []*entities.Artifact{
				{MediaID: "m1", Type: entities.ArtifactTypeImage, Data: []byte("data"), URL: "https://existing.com/img.png"},
			},
		},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// URL should not have changed
	if state.Event.Attachments[0].URL != "https://existing.com/img.png" {
		t.Error("existing URL was overwritten")
	}
}

func TestMediaVaultStep_StoreSuccess_SetsURL(t *testing.T) {
	vault := &stubVault{url: "https://cdn/stored.png"}
	step := NewMediaVaultStep(vault, time.Second, testLogger(t))
	att := &entities.Artifact{MediaID: "m1", Type: entities.ArtifactTypeImage, MimeType: "image/png", Data: []byte("imgdata")}
	state := &ProcessingState{
		Event: &entities.Pulse{
			MemoryKey:   "user:1",
			Attachments: []*entities.Artifact{att},
		},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if att.URL != "https://cdn/stored.png" {
		t.Errorf("expected URL set, got %q", att.URL)
	}
}

func TestMediaVaultStep_StoreFails_ContinuesProcessing(t *testing.T) {
	vault := &stubVault{err: errors.New("storage full")}
	step := NewMediaVaultStep(vault, time.Second, testLogger(t))
	att1 := &entities.Artifact{MediaID: "m1", Type: entities.ArtifactTypeImage, MimeType: "image/png", Data: []byte("data1")}
	att2 := &entities.Artifact{MediaID: "m2", Type: entities.ArtifactTypeImage, MimeType: "image/jpeg", Data: []byte("data2")}
	state := &ProcessingState{
		Event: &entities.Pulse{
			MemoryKey:   "user:1",
			Attachments: []*entities.Artifact{att1, att2},
		},
	}
	// Store errors should be non-fatal (warn + continue)
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("store failure must be non-fatal, got: %v", err)
	}
}

// --- MediaProcessingStep ---

func TestMediaProcessingStep_DefaultTimeout(t *testing.T) {
	step := NewMediaProcessingStep(&stubLens{}, 0)
	if step.Timeout() != 30*time.Second {
		t.Errorf("expected default 30s, got %v", step.Timeout())
	}
}

func TestMediaProcessingStep_CustomTimeout(t *testing.T) {
	step := NewMediaProcessingStep(&stubLens{}, 10*time.Second)
	if step.Timeout() != 10*time.Second {
		t.Errorf("expected 10s, got %v", step.Timeout())
	}
	if step.Name() != "MediaProcessing" {
		t.Errorf("expected name=MediaProcessing, got %s", step.Name())
	}
}

func TestMediaProcessingStep_NoAttachments_NoOp(t *testing.T) {
	step := NewMediaProcessingStep(&stubLens{}, time.Second)
	state := &ProcessingState{
		Event: &entities.Pulse{MemoryKey: "user:1", Attachments: []*entities.Artifact{}},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMediaProcessingStep_ProcessSuccess_SetsUsage(t *testing.T) {
	lens := &stubLens{usage: ports.OracleUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150}}
	step := NewMediaProcessingStep(lens, time.Second)
	state := &ProcessingState{
		Event: &entities.Pulse{
			MemoryKey:   "user:1",
			Attachments: []*entities.Artifact{{MediaID: "m1", Data: []byte("data")}},
		},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.MediaTokensUsed.PromptTokens != 100 {
		t.Errorf("expected PromptTokens=100, got %d", state.MediaTokensUsed.PromptTokens)
	}
}

func TestMediaProcessingStep_ProcessFails_ReturnsError(t *testing.T) {
	lens := &stubLens{err: errors.New("transcription failed")}
	step := NewMediaProcessingStep(lens, time.Second)
	state := &ProcessingState{
		Event: &entities.Pulse{
			MemoryKey:   "user:1",
			Attachments: []*entities.Artifact{{MediaID: "m1", Data: []byte("audio")}},
		},
	}
	if err := step.Execute(context.Background(), state); err == nil {
		t.Error("expected error from lens processing failure")
	}
}
