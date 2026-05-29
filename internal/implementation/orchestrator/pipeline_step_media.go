package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers"
	"go.uber.org/zap"
)

type MediaVaultStep struct {
	store   ports.Vault
	timeout time.Duration
	logger  *zap.SugaredLogger
}

func NewMediaVaultStep(store ports.Vault, timeout time.Duration, logger *zap.SugaredLogger) *MediaVaultStep {
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &MediaVaultStep{store: store, timeout: timeout, logger: logger}
}

func (s *MediaVaultStep) Name() string           { return "MediaVault" }
func (s *MediaVaultStep) Timeout() time.Duration { return s.timeout }

func (s *MediaVaultStep) Execute(ctx context.Context, state *ProcessingState) error {
	for _, att := range state.Event.Attachments {
		if len(att.Data) == 0 {
			s.logger.Infow("attachment skipped: no data (download failed or not attempted)",
				"event_id", state.Event.ID, "media_id", att.MediaID, "type", att.Type)
			continue
		}
		if att.URL != "" {
			s.logger.Debugw("attachment skipped: URL already set",
				"event_id", state.Event.ID, "media_id", att.MediaID, "type", att.Type)
			continue
		}

		key := buildVaultKey(state.Event.MemoryKey, att)
		url, err := s.store.Store(ctx, key, att.Data, att.MimeType)
		if err != nil {
			s.logger.Warnw("failed to store attachment",
				"key", key, "event_id", state.Event.ID, "media_id", att.MediaID, "type", att.Type, "error", err)
			continue
		}

		att.URL = url
		s.logger.Infow("attachment stored",
			"event_id", state.Event.ID, "media_id", att.MediaID, "type", att.Type, "url", url)
	}

	return nil
}

type MediaProcessingStep struct {
	processor ports.Lens
	timeout   time.Duration
}

func NewMediaProcessingStep(processor ports.Lens, timeout time.Duration) *MediaProcessingStep {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &MediaProcessingStep{processor: processor, timeout: timeout}
}

func (s *MediaProcessingStep) Name() string           { return "MediaProcessing" }
func (s *MediaProcessingStep) Timeout() time.Duration { return s.timeout }

func (s *MediaProcessingStep) Execute(ctx context.Context, state *ProcessingState) error {
	if len(state.Event.Attachments) == 0 {
		return nil
	}
	usage, err := s.processor.Process(ctx, state.Event)
	state.MediaTokensUsed = usage
	if err != nil {
		return fmt.Errorf("process media: %w", err)
	}
	return nil
}

func buildVaultKey(memoryKey string, att *entities.Artifact) string {
	folder := strings.ReplaceAll(memoryKey, ":", "/")
	ext := mimeToExt(att.MimeType)
	typeName := strings.ToLower(string(att.Type))
	if typeName == "" {
		typeName = "file"
	}
	return fmt.Sprintf("%s/%s_%s.%s", folder, typeName, helpers.GenerateRandomID(), ext)
}

func mimeToExt(mimeType string) string {
	switch mimeType {
	case "audio/ogg", "audio/ogg; codecs=opus":
		return "ogg"
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	case "audio/mp4":
		return "m4a"
	case "audio/wav":
		return "wav"
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	case "video/mp4":
		return "mp4"
	case "video/3gpp":
		return "3gp"
	case "application/pdf":
		return "pdf"
	}
	if idx := strings.Index(mimeType, "/"); idx >= 0 {
		return mimeType[idx+1:]
	}
	return "bin"
}
