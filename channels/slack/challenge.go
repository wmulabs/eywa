package slack

import "encoding/json"

// URLVerificationChallenge reports whether the raw request body is Slack's one-time url_verification
// handshake and, if so, the challenge string to echo back. Mount this at the start of the Slack webhook
// route: when ok, respond 200 with the returned challenge; otherwise proceed to normal event ingestion.
// Echoing the challenge is an HTTP-response concern the application route owns — the Receptor only
// produces Pulses.
func URLVerificationChallenge(body []byte) (challenge string, ok bool) {
	var p struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return "", false
	}
	if p.Type != "url_verification" {
		return "", false
	}
	return p.Challenge, true
}
