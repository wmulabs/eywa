package voices

import (
	"context"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

func TestHTTPVoice_GetName(t *testing.T) {
	v := NewHTTPVoice()
	if v.GetName() != "http" {
		t.Errorf("expected name=http, got %s", v.GetName())
	}
}

func TestHTTPVoice_ShouldAutoRespond_False(t *testing.T) {
	v := NewHTTPVoice()
	if v.ShouldAutoRespond() {
		t.Error("expected ShouldAutoRespond=false")
	}
}

func TestHTTPVoice_SendResponse_NoError(t *testing.T) {
	v := NewHTTPVoice()
	if err := v.SendResponse(context.Background(), &entities.Pulse{}, "response"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPVoice_GetChannelMetadata(t *testing.T) {
	v := NewHTTPVoice()
	pulse := &entities.Pulse{Source: "api"}
	meta := v.GetChannelMetadata(pulse)
	if meta["channel"] != "http" {
		t.Errorf("expected channel=http, got %v", meta["channel"])
	}
	if meta["source"] != "api" {
		t.Errorf("expected source=api, got %v", meta["source"])
	}
}
