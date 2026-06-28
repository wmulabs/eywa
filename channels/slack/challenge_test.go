package slack

import "testing"

func TestURLVerificationChallenge(t *testing.T) {
	if c, ok := URLVerificationChallenge([]byte(`{"type":"url_verification","challenge":"abc123"}`)); !ok || c != "abc123" {
		t.Errorf("expected challenge abc123, got %q ok=%v", c, ok)
	}
	if _, ok := URLVerificationChallenge([]byte(`{"type":"event_callback"}`)); ok {
		t.Error("event_callback must not be a challenge")
	}
	if _, ok := URLVerificationChallenge([]byte(`not json`)); ok {
		t.Error("invalid JSON must not be a challenge")
	}
}
