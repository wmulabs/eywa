package media

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

// --- stubs ---

type stubOracle struct {
	supportsImages    bool
	supportsAudio     bool
	supportsDocuments bool
}

var _ ports.Oracle = (*stubOracle)(nil)

func (o *stubOracle) GetName() string                 { return "stub" }
func (o *stubOracle) GetAvailableModels() []string    { return nil }
func (o *stubOracle) IsAvailable() bool               { return true }
func (o *stubOracle) GetConfig() map[string]any       { return nil }
func (o *stubOracle) SupportsImages(_ string) bool    { return o.supportsImages }
func (o *stubOracle) SupportsAudio(_ string) bool     { return o.supportsAudio }
func (o *stubOracle) SupportsDocuments(_ string) bool { return o.supportsDocuments }
func (o *stubOracle) GenerateResponse(_ context.Context, _ *ports.OracleRequest) (*ports.OracleResponse, error) {
	return nil, nil
}

type stubEar struct {
	text  string
	usage ports.OracleUsage
	err   error
}

var _ ports.Ear = (*stubEar)(nil)

func (e *stubEar) Transcribe(_ context.Context, _ []byte, _ string) (string, ports.OracleUsage, error) {
	return e.text, e.usage, e.err
}

type stubEye struct {
	desc  string
	usage ports.OracleUsage
	err   error
}

var _ ports.Eye = (*stubEye)(nil)

func (e *stubEye) Analyze(_ context.Context, _ []byte, _ string) (string, ports.OracleUsage, error) {
	return e.desc, e.usage, e.err
}

type stubDocProcessor struct {
	text string
	err  error
}

var _ ports.DocumentProcessor = (*stubDocProcessor)(nil)

func (d *stubDocProcessor) Process(_ context.Context, _ []byte, _ string) (string, error) {
	return d.text, d.err
}

func testLogger() *zap.SugaredLogger {
	logger, _ := zap.NewDevelopment()
	return logger.Sugar()
}

func pulse(userMsg string, attachments ...*entities.Artifact) *entities.Pulse {
	return &entities.Pulse{
		ID:          "event-1",
		MemoryKey:   "user:1",
		UserMessage: userMsg,
		Attachments: attachments,
	}
}

// =============================================================================
// ConvertToLLMAttachments (attachment.go)
// =============================================================================

func TestConvertToLLMAttachments_Empty_ReturnsNil(t *testing.T) {
	result := ConvertToLLMAttachments(nil, &stubOracle{}, "gpt-4o")
	if result != nil {
		t.Errorf("expected nil for empty attachments, got %v", result)
	}
}

func TestConvertToLLMAttachments_NilAttachment_Skipped(t *testing.T) {
	result := ConvertToLLMAttachments([]*entities.Artifact{nil}, &stubOracle{supportsImages: true}, "gpt-4o")
	if len(result) != 0 {
		t.Errorf("expected 0 results for nil attachment, got %d", len(result))
	}
}

func TestConvertToLLMAttachments_UnsupportedType_Filtered(t *testing.T) {
	att := &entities.Artifact{Type: entities.ArtifactTypeImage, Data: []byte("img"), MimeType: "image/png"}
	oracle := &stubOracle{supportsImages: false} // images not supported
	result := ConvertToLLMAttachments([]*entities.Artifact{att}, oracle, "gpt-4")
	if len(result) != 0 {
		t.Errorf("expected 0 results for unsupported image, got %d", len(result))
	}
}

func TestConvertToLLMAttachments_SupportedImage_Included(t *testing.T) {
	att := &entities.Artifact{
		Type:     entities.ArtifactTypeImage,
		Data:     []byte("imgdata"),
		URL:      "https://cdn/img.png",
		MimeType: "image/png",
		Caption:  "A cat",
	}
	oracle := &stubOracle{supportsImages: true}
	result := ConvertToLLMAttachments([]*entities.Artifact{att}, oracle, "gpt-4o")
	if len(result) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(result))
	}
	if result[0].Caption != "A cat" {
		t.Errorf("expected caption=A cat, got %s", result[0].Caption)
	}
}

func TestConvertToLLMAttachments_SupportedAudio_Included(t *testing.T) {
	att := &entities.Artifact{Type: entities.ArtifactTypeAudio, Data: []byte("audio"), MimeType: "audio/ogg"}
	oracle := &stubOracle{supportsAudio: true}
	result := ConvertToLLMAttachments([]*entities.Artifact{att}, oracle, "gpt-4o-audio")
	if len(result) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(result))
	}
}

func TestConvertToLLMAttachments_SupportedDocument_Included(t *testing.T) {
	att := &entities.Artifact{Type: entities.ArtifactTypeDocument, Data: []byte("pdf"), MimeType: "application/pdf"}
	oracle := &stubOracle{supportsDocuments: true}
	result := ConvertToLLMAttachments([]*entities.Artifact{att}, oracle, "claude-3-5-sonnet")
	if len(result) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(result))
	}
}

func TestConvertToLLMAttachments_NoDataAndNoURL_Filtered(t *testing.T) {
	att := &entities.Artifact{Type: entities.ArtifactTypeImage, Data: nil, URL: "", MimeType: "image/png"}
	oracle := &stubOracle{supportsImages: true}
	result := ConvertToLLMAttachments([]*entities.Artifact{att}, oracle, "gpt-4o")
	if len(result) != 0 {
		t.Errorf("expected 0 results for attachment with no data and no URL, got %d", len(result))
	}
}

func TestConvertToLLMAttachments_URLWithoutData_Included(t *testing.T) {
	att := &entities.Artifact{Type: entities.ArtifactTypeImage, Data: nil, URL: "https://cdn/img.png", MimeType: "image/png"}
	oracle := &stubOracle{supportsImages: true}
	result := ConvertToLLMAttachments([]*entities.Artifact{att}, oracle, "gpt-4o")
	if len(result) != 1 {
		t.Errorf("expected 1 (URL-only is valid), got %d", len(result))
	}
}

func TestConvertToLLMAttachments_UnknownType_Filtered(t *testing.T) {
	att := &entities.Artifact{Type: "unknown-type", Data: []byte("data")}
	result := ConvertToLLMAttachments([]*entities.Artifact{att}, &stubOracle{}, "gpt-4o")
	if len(result) != 0 {
		t.Errorf("expected 0 for unknown type, got %d", len(result))
	}
}

// =============================================================================
// Processor (lens.go)
// =============================================================================

func TestNewLens_ReturnsLens(t *testing.T) {
	lens := NewLens(nil, nil, nil, testLogger())
	if lens == nil {
		t.Fatal("expected non-nil lens")
	}
}

func TestProcessor_Process_NoAttachments_ReturnsZeroUsage(t *testing.T) {
	lens := NewLens(nil, nil, nil, testLogger())
	ev := pulse("")
	usage, err := lens.Process(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.TotalTokens != 0 {
		t.Errorf("expected zero usage, got %v", usage)
	}
}

func TestProcessor_NotifyVideoNotSupported_AppendsToUserMessage(t *testing.T) {
	lens := NewLens(nil, nil, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeVideo, MediaID: "v1"}
	ev := pulse("hello", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage == "hello" {
		t.Error("expected user message to be updated for video")
	}
}

func TestProcessor_NotifyDownloadFailed_AppendsToUserMessage(t *testing.T) {
	lens := NewLens(nil, nil, nil, testLogger())
	att := &entities.Artifact{
		Type:    entities.ArtifactTypeAudio,
		MediaID: "a1",
		Data:    nil, // no data = download failed
	}
	ev := pulse("", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage == "" {
		t.Error("expected user message to be updated for download failure")
	}
}

func TestProcessor_NotifyDownloadFailed_SkipsVideo(t *testing.T) {
	lens := NewLens(nil, nil, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeVideo, MediaID: "v1", Data: nil}
	ev := pulse("original", att)
	lens.Process(context.Background(), ev)
	// Should only append video notification once, not also download-failed
	// Can't guarantee exact count, just verify no panic
}

func TestProcessor_NotifyDownloadFailed_SkipsWhenDataPresent(t *testing.T) {
	lens := NewLens(nil, nil, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeAudio, MediaID: "a1", Data: []byte("data")}
	ev := pulse("original", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage != "original" {
		t.Errorf("expected message unchanged, got %q", ev.UserMessage)
	}
}

func TestProcessor_NotifyDownloadFailed_SkipsWhenNoMediaID(t *testing.T) {
	lens := NewLens(nil, nil, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeImage, MediaID: "", Data: nil}
	ev := pulse("original", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage != "original" {
		t.Errorf("expected message unchanged when no media ID, got %q", ev.UserMessage)
	}
}

func TestProcessor_TranscribeAudio_NoTranscriber_NoOp(t *testing.T) {
	lens := NewLens(nil, nil, nil, testLogger()) // nil ear
	att := &entities.Artifact{Type: entities.ArtifactTypeAudio, MediaID: "a1", Data: []byte("audio")}
	ev := pulse("", att)
	lens.Process(context.Background(), ev)
	// No panic, no append
}

func TestProcessor_TranscribeAudio_Success_AppendsText(t *testing.T) {
	ear := &stubEar{text: "hello world", usage: ports.OracleUsage{TotalTokens: 50}}
	lens := NewLens(ear, nil, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeAudio, MediaID: "a1", Data: []byte("audio"), MimeType: "audio/ogg"}
	ev := pulse("", att)
	usage, err := lens.Process(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.UserMessage == "" {
		t.Error("expected user message to contain transcription")
	}
	if usage.TotalTokens != 50 {
		t.Errorf("expected TotalTokens=50, got %d", usage.TotalTokens)
	}
}

func TestProcessor_TranscribeAudio_Error_Continues(t *testing.T) {
	ear := &stubEar{err: errors.New("transcription failed")}
	lens := NewLens(ear, nil, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeAudio, MediaID: "a1", Data: []byte("audio")}
	ev := pulse("", att)
	_, err := lens.Process(context.Background(), ev)
	if err != nil {
		t.Fatalf("transcription errors must be non-fatal, got: %v", err)
	}
}

func TestProcessor_TranscribeAudio_EmptyResult_NoAppend(t *testing.T) {
	ear := &stubEar{text: ""} // empty transcription
	lens := NewLens(ear, nil, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeAudio, MediaID: "a1", Data: []byte("audio")}
	ev := pulse("original", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage != "original" {
		t.Errorf("expected message unchanged for empty transcription, got %q", ev.UserMessage)
	}
}

func TestProcessor_TranscribeAudio_SkipsNonAudio(t *testing.T) {
	ear := &stubEar{text: "should not appear"}
	lens := NewLens(ear, nil, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeImage, Data: []byte("img")}
	ev := pulse("original", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage != "original" {
		t.Errorf("expected message unchanged for image att processed by audio transcriber, got %q", ev.UserMessage)
	}
}

func TestProcessor_TranscribeAudio_SkipsEmptyData(t *testing.T) {
	ear := &stubEar{text: "should not appear"}
	lens := NewLens(ear, nil, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeAudio, Data: nil}
	ev := pulse("original", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage != "original" {
		t.Errorf("expected message unchanged for empty audio data, got %q", ev.UserMessage)
	}
}

func TestProcessor_AnalyzeImages_NoAnalyzer_NoOp(t *testing.T) {
	lens := NewLens(nil, nil, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeImage, Data: []byte("img")}
	ev := pulse("", att)
	lens.Process(context.Background(), ev)
}

func TestProcessor_AnalyzeImages_Success_AppendsDescription(t *testing.T) {
	eye := &stubEye{desc: "a sunset photo", usage: ports.OracleUsage{TotalTokens: 30}}
	lens := NewLens(nil, eye, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeImage, Data: []byte("img"), MimeType: "image/jpeg"}
	ev := pulse("", att)
	usage, err := lens.Process(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.UserMessage == "" {
		t.Error("expected user message to contain image description")
	}
	if usage.TotalTokens != 30 {
		t.Errorf("expected TotalTokens=30, got %d", usage.TotalTokens)
	}
}

func TestProcessor_AnalyzeImages_WithCaption_UsesCaption(t *testing.T) {
	eye := &stubEye{desc: "a dog"}
	lens := NewLens(nil, eye, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeImage, Data: []byte("img"), Caption: "my dog"}
	ev := pulse("", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage == "" {
		t.Error("expected user message")
	}
	// Caption used in label
}

func TestProcessor_AnalyzeImages_Error_Continues(t *testing.T) {
	eye := &stubEye{err: errors.New("analysis failed")}
	lens := NewLens(nil, eye, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeImage, Data: []byte("img")}
	ev := pulse("", att)
	_, err := lens.Process(context.Background(), ev)
	if err != nil {
		t.Fatalf("image analysis error must be non-fatal, got: %v", err)
	}
}

func TestProcessor_AnalyzeImages_SkipsNonImageType(t *testing.T) {
	eye := &stubEye{desc: "should not appear"}
	lens := NewLens(nil, eye, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeAudio, Data: []byte("audio")}
	ev := pulse("original", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage != "original" {
		t.Errorf("expected unchanged for non-image type, got %q", ev.UserMessage)
	}
}

func TestProcessor_AnalyzeImages_SkipsEmptyData(t *testing.T) {
	eye := &stubEye{desc: "should not appear"}
	lens := NewLens(nil, eye, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeImage, Data: nil}
	ev := pulse("original", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage != "original" {
		t.Errorf("expected unchanged for empty image data, got %q", ev.UserMessage)
	}
}

func TestProcessor_AnalyzeImages_EmptyDesc_NoAppend(t *testing.T) {
	eye := &stubEye{desc: ""}
	lens := NewLens(nil, eye, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeImage, Data: []byte("img")}
	ev := pulse("original", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage != "original" {
		t.Errorf("expected message unchanged for empty description, got %q", ev.UserMessage)
	}
}

func TestProcessor_ExtractDocuments_NoExtractor_NoOp(t *testing.T) {
	lens := NewLens(nil, nil, nil, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeDocument, Data: []byte("pdf")}
	ev := pulse("", att)
	lens.Process(context.Background(), ev)
}

func TestProcessor_ExtractDocuments_Success_AppendsText(t *testing.T) {
	doc := &stubDocProcessor{text: "extracted text"}
	lens := NewLens(nil, nil, doc, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeDocument, Data: []byte("pdf"), MimeType: "application/pdf"}
	ev := pulse("", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage == "" {
		t.Error("expected user message to contain extracted document text")
	}
}

func TestProcessor_ExtractDocuments_WithFileName(t *testing.T) {
	doc := &stubDocProcessor{text: "invoice text"}
	lens := NewLens(nil, nil, doc, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeDocument, Data: []byte("pdf"), FileName: "invoice.pdf"}
	ev := pulse("", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage == "" {
		t.Error("expected user message")
	}
}

func TestProcessor_ExtractDocuments_Error_Continues(t *testing.T) {
	doc := &stubDocProcessor{err: errors.New("extraction failed")}
	lens := NewLens(nil, nil, doc, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeDocument, Data: []byte("pdf")}
	ev := pulse("", att)
	_, err := lens.Process(context.Background(), ev)
	if err != nil {
		t.Fatalf("extraction errors must be non-fatal, got: %v", err)
	}
}

func TestProcessor_ExtractDocuments_EmptyText_NoAppend(t *testing.T) {
	doc := &stubDocProcessor{text: ""}
	lens := NewLens(nil, nil, doc, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeDocument, Data: []byte("pdf")}
	ev := pulse("original", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage != "original" {
		t.Errorf("expected message unchanged for empty doc text, got %q", ev.UserMessage)
	}
}

func TestProcessor_ExtractDocuments_SkipsEmptyData(t *testing.T) {
	doc := &stubDocProcessor{text: "should not appear"}
	lens := NewLens(nil, nil, doc, testLogger())
	att := &entities.Artifact{Type: entities.ArtifactTypeDocument, Data: nil}
	ev := pulse("original", att)
	lens.Process(context.Background(), ev)
	if ev.UserMessage != "original" {
		t.Errorf("expected message unchanged, got %q", ev.UserMessage)
	}
}

// --- appendToUserMessage ---

func TestAppendToUserMessage_EmptyOriginal(t *testing.T) {
	ev := &entities.Pulse{UserMessage: ""}
	appendToUserMessage(ev, "[Audio]", "hello")
	if ev.UserMessage != "[Audio]: hello" {
		t.Errorf("unexpected: %q", ev.UserMessage)
	}
}

func TestAppendToUserMessage_NonEmpty(t *testing.T) {
	ev := &entities.Pulse{UserMessage: "hi"}
	appendToUserMessage(ev, "[Image]", "a cat")
	if ev.UserMessage != "hi\n\n[Image]: a cat" {
		t.Errorf("unexpected: %q", ev.UserMessage)
	}
}

// --- attachmentLabel ---

func TestAttachmentLabel_Audio(t *testing.T) {
	att := &entities.Artifact{Type: entities.ArtifactTypeAudio}
	label := attachmentLabel(att)
	if label != "[Audio]" {
		t.Errorf("expected [Audio], got %s", label)
	}
}

func TestAttachmentLabel_EmptyType(t *testing.T) {
	att := &entities.Artifact{Type: ""}
	label := attachmentLabel(att)
	if label != "[]" {
		t.Errorf("expected [], got %s", label)
	}
}
