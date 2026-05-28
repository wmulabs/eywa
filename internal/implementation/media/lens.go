package media

import (
	"context"
	"fmt"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

type Processor struct {
	audioTranscriber  ports.Ear
	imageAnalyzer     ports.Eye
	documentExtractor ports.DocumentProcessor
	logger            *zap.SugaredLogger
}

func NewLens(
	audioTranscriber ports.Ear,
	imageAnalyzer ports.Eye,
	documentExtractor ports.DocumentProcessor,
	logger *zap.SugaredLogger,
) ports.Lens {
	return &Processor{
		audioTranscriber:  audioTranscriber,
		imageAnalyzer:     imageAnalyzer,
		documentExtractor: documentExtractor,
		logger:            logger,
	}
}

func (p *Processor) Process(ctx context.Context, event *entities.Pulse) (ports.OracleUsage, error) {
	p.notifyVideoNotSupported(event)
	p.notifyDownloadFailed(event)

	var total ports.OracleUsage
	total.Add(p.extractDocuments(ctx, event))
	total.Add(p.transcribeAudio(ctx, event))
	total.Add(p.analyzeImages(ctx, event))

	return total, nil
}

func (p *Processor) notifyVideoNotSupported(event *entities.Pulse) {
	for _, att := range event.Attachments {
		if att.Type != entities.ArtifactTypeVideo {
			continue
		}
		p.logger.Infow("video attachment received — not supported, notifying LLM",
			"event_id", event.ID, "media_id", att.MediaID)
		appendToUserMessage(event, "[Video]", "video messages cannot be processed — let the user know you are unable to handle video content and suggest they send audio or images instead")
	}
}

func (p *Processor) notifyDownloadFailed(event *entities.Pulse) {
	for _, att := range event.Attachments {
		if att.Type == entities.ArtifactTypeVideo {
			continue
		}
		if len(att.Data) > 0 || att.MediaID == "" {
			continue
		}
		p.logger.Warnw("attachment download failed — no data present",
			"event_id", event.ID, "media_id", att.MediaID, "type", att.Type)
		label := attachmentLabel(att)
		appendToUserMessage(event, label, "media unavailable — could not be downloaded; let the user know and ask them to try sending it again")
	}
}

func (p *Processor) extractDocuments(ctx context.Context, event *entities.Pulse) ports.OracleUsage {
	if p.documentExtractor == nil {
		return ports.OracleUsage{}
	}

	var total ports.OracleUsage
	for _, att := range event.Attachments {
		if att.Type != entities.ArtifactTypeDocument || len(att.Data) == 0 {
			continue
		}

		p.logger.Infow("extracting document", "event_id", event.ID, "media_id", att.MediaID, "mime_type", att.MimeType)
		text, err := p.documentExtractor.Process(ctx, att.Data, att.MimeType)
		if err != nil {
			p.logger.Warnw("failed to extract document",
				"media_id", att.MediaID, "event_id", event.ID, "error", err)
			continue
		}

		if text != "" {
			label := "[Document]"
			if att.FileName != "" {
				label = fmt.Sprintf("[Document — %s]", att.FileName)
			}
			appendToUserMessage(event, label, text)
			p.logger.Infow("document extracted", "event_id", event.ID, "media_id", att.MediaID, "chars", len(text))
		}
	}
	return total
}

func (p *Processor) transcribeAudio(ctx context.Context, event *entities.Pulse) ports.OracleUsage {
	if p.audioTranscriber == nil {
		return ports.OracleUsage{}
	}

	var total ports.OracleUsage
	for _, att := range event.Attachments {
		if att.Type != entities.ArtifactTypeAudio || len(att.Data) == 0 {
			continue
		}

		p.logger.Infow("transcribing audio", "event_id", event.ID, "media_id", att.MediaID, "mime_type", att.MimeType, "bytes", len(att.Data))
		text, usage, err := p.audioTranscriber.Transcribe(ctx, att.Data, att.MimeType)
		if err != nil {
			p.logger.Warnw("failed to transcribe audio",
				"media_id", att.MediaID, "event_id", event.ID, "error", err)
			continue
		}

		total.Add(usage)
		if text != "" {
			appendToUserMessage(event, "[Audio]", text)
			p.logger.Infow("audio transcribed", "event_id", event.ID, "media_id", att.MediaID, "chars", len(text))
		}
	}
	return total
}

func (p *Processor) analyzeImages(ctx context.Context, event *entities.Pulse) ports.OracleUsage {
	if p.imageAnalyzer == nil {
		return ports.OracleUsage{}
	}

	var total ports.OracleUsage
	for _, att := range event.Attachments {
		if att.Type != entities.ArtifactTypeImage || len(att.Data) == 0 {
			continue
		}

		p.logger.Infow("analyzing image", "event_id", event.ID, "media_id", att.MediaID, "mime_type", att.MimeType, "bytes", len(att.Data))
		desc, usage, err := p.imageAnalyzer.Analyze(ctx, att.Data, att.MimeType)
		if err != nil {
			p.logger.Warnw("failed to analyze image",
				"media_id", att.MediaID, "event_id", event.ID, "error", err)
			continue
		}

		total.Add(usage)
		if desc != "" {
			label := "[Image]"
			if att.Caption != "" {
				label = fmt.Sprintf("[Image — caption: %s]", att.Caption)
			}
			appendToUserMessage(event, label, desc)
			p.logger.Infow("image analyzed", "event_id", event.ID, "media_id", att.MediaID, "chars", len(desc))
		}
	}
	return total
}

func attachmentLabel(att *entities.Artifact) string {
	typeName := strings.ToLower(string(att.Type))
	if len(typeName) > 0 {
		typeName = strings.ToUpper(typeName[:1]) + typeName[1:]
	}
	return fmt.Sprintf("[%s]", typeName)
}

func appendToUserMessage(event *entities.Pulse, label, content string) {
	entry := label + ": " + content
	if event.UserMessage == "" {
		event.UserMessage = entry
	} else {
		event.UserMessage = event.UserMessage + "\n\n" + entry
	}
}
