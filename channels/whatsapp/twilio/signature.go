package twilio

import (
	"context"
	"fmt"

	twilioclient "github.com/twilio/twilio-go/client"
	eywa "github.com/wmulabs/eywa"
)

const twilioSignatureHeader = "X-Twilio-Signature"

// SignatureVerifier authenticates inbound Twilio webhooks by validating X-Twilio-Signature. It
// delegates to the official twilio-go RequestValidator, which implements Twilio's documented scheme
// (HMAC-SHA1 over the request URL plus sorted form params, base64) including the form vs JSON body
// cases and Twilio's with/without-port URL ambiguity.
//
// The URL must match exactly what Twilio signed (the webhook URL configured in the Twilio Console).
// Behind a proxy/load balancer, ensure the app reconstructs the original external URL
// (X-Forwarded-Proto/Host) or the signature will not match.
type SignatureVerifier struct {
	validator twilioclient.RequestValidator
}

func NewSignatureVerifier(authToken string) eywa.RequestVerifier {
	return &SignatureVerifier{validator: twilioclient.NewRequestValidator(authToken)}
}

func (v *SignatureVerifier) Verify(_ context.Context, req eywa.VerifiableRequest) (*eywa.AuthClaims, error) {
	provided := req.Header.Get(twilioSignatureHeader)
	if provided == "" {
		return nil, fmt.Errorf("missing %s", twilioSignatureHeader)
	}
	if !v.validator.ValidateBody(req.URL, req.Body, provided) {
		return nil, fmt.Errorf("twilio signature mismatch")
	}
	return &eywa.AuthClaims{Subject: "twilio", Role: "app"}, nil
}
