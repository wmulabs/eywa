package media

import (
	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// ConvertToLLMAttachments builds the LLM attachment list from a Pulse's Artifacts,
// stripping bytes for media types the provider does not support natively to prevent
// API errors on text-only models.
func ConvertToLLMAttachments(attachments []*entities.Artifact, provider ports.Oracle, model string) []ports.LLMAttachment {
	if len(attachments) == 0 {
		return nil
	}

	result := make([]ports.LLMAttachment, 0, len(attachments))
	for _, att := range attachments {
		if a := toNativeLLMAttachment(att, provider, model); a != nil {
			result = append(result, *a)
		}
	}
	return result
}

func toNativeLLMAttachment(att *entities.Artifact, provider ports.Oracle, model string) *ports.LLMAttachment {
	if att == nil {
		return nil
	}

	if !isMediaSupported(att.Type, provider, model) {
		return nil
	}

	if len(att.Data) == 0 && att.URL == "" {
		return nil
	}

	return &ports.LLMAttachment{
		Type:     att.Type,
		URL:      att.URL,
		Data:     att.Data,
		MimeType: att.MimeType,
		Caption:  att.Caption,
	}
}

func isMediaSupported(t entities.ArtifactType, provider ports.Oracle, model string) bool {
	switch t {
	case entities.ArtifactTypeAudio:
		return provider.SupportsAudio(model)
	case entities.ArtifactTypeImage:
		return provider.SupportsImages(model)
	case entities.ArtifactTypeDocument:
		return provider.SupportsDocuments(model)
	default:
		return false
	}
}
