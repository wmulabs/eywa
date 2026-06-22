package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

const (
	defaultHMACHeader    = "X-Eywa-Signature"
	defaultHMACTolerance = 5 * time.Minute
)

// HMACVerifier authenticates a request by a Stripe-style signature over the raw body. The signature
// header has the form "t=<unix-seconds>,v1=<hex-hmac-sha256>"; the signed payload is "<t>.<body>".
// The timestamp must be within Tolerance of now, which (with downstream idempotency) blocks replay.
type HMACVerifier struct {
	secret    []byte
	header    string
	tolerance time.Duration
	now       func() time.Time
}

// HMACOption customizes an HMACVerifier.
type HMACOption func(*HMACVerifier)

// WithHMACHeader overrides the signature header name (default "X-Eywa-Signature").
func WithHMACHeader(name string) HMACOption { return func(v *HMACVerifier) { v.header = name } }

// WithHMACTolerance overrides the max timestamp skew accepted (default 5m).
func WithHMACTolerance(d time.Duration) HMACOption {
	return func(v *HMACVerifier) { v.tolerance = d }
}

// NewHMACVerifier returns a RequestVerifier that checks a Stripe-style HMAC-SHA256 signature.
func NewHMACVerifier(secret string, opts ...HMACOption) ports.RequestVerifier {
	v := &HMACVerifier{
		secret:    []byte(secret),
		header:    defaultHMACHeader,
		tolerance: defaultHMACTolerance,
		now:       time.Now,
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

func (v *HMACVerifier) Verify(_ context.Context, req ports.VerifiableRequest) (*ports.AuthClaims, error) {
	ts, sig, err := parseSignatureHeader(req.Header.Get(v.header))
	if err != nil {
		return nil, err
	}

	skew := v.now().Unix() - ts
	if skew < 0 {
		skew = -skew
	}
	if time.Duration(skew)*time.Second > v.tolerance {
		return nil, fmt.Errorf("hmac signature timestamp outside tolerance")
	}

	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(strconv.FormatInt(ts, 10) + "." + string(req.Body)))
	expected := mac.Sum(nil)

	if subtle.ConstantTimeCompare(expected, sig) != 1 {
		return nil, fmt.Errorf("hmac signature mismatch")
	}
	return &ports.AuthClaims{Subject: "hmac", Role: appTokenRole}, nil
}

// parseSignatureHeader parses "t=<unix>,v1=<hex>" into the timestamp and decoded signature bytes.
func parseSignatureHeader(header string) (ts int64, sig []byte, err error) {
	if header == "" {
		return 0, nil, fmt.Errorf("missing signature header")
	}
	var tsStr, sigHex string
	for _, part := range strings.Split(header, ",") {
		k, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			tsStr = val
		case "v1":
			sigHex = val
		}
	}
	if tsStr == "" || sigHex == "" {
		return 0, nil, fmt.Errorf("signature header must contain t and v1")
	}
	ts, err = strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid signature timestamp")
	}
	sig, err = hex.DecodeString(sigHex)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid signature encoding")
	}
	return ts, sig, nil
}
