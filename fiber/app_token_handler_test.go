package fiber

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

type stubAppTokenRepo struct {
	created   *eywa.AppToken
	tokens    []*eywa.AppToken
	createErr error
	listErr   error
	revokeErr error
}

func (s *stubAppTokenRepo) Create(_ context.Context, t *eywa.AppToken) error {
	s.created = t
	return s.createErr
}
func (s *stubAppTokenRepo) FindByHash(_ context.Context, _ []byte) (*eywa.AppToken, error) {
	return nil, eywa.ErrNotFound
}
func (s *stubAppTokenRepo) List(_ context.Context) ([]*eywa.AppToken, error) {
	return s.tokens, s.listErr
}
func (s *stubAppTokenRepo) Revoke(_ context.Context, _ string) error { return s.revokeErr }

func buildAppTokenApp(t *testing.T, repo eywa.AppTokenRepository) *fiberlib.App {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	if err := RegisterRoutes(app, minimalTestWeave(t), RouteDeps{AppTokenRepo: repo, APIKeys: authedAPIKeys()}); err != nil {
		t.Fatalf("register: %v", err)
	}
	return app
}

func TestAppTokenHandler_Create_Returns201WithSecret(t *testing.T) {
	repo := &stubAppTokenRepo{}
	app := buildAppTokenApp(t, repo)

	body, _ := json.Marshal(map[string]any{"name": "ci", "ttl_seconds": 3600})
	req := authedRequest("POST", "/api/v1/app-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 201 {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out) //nolint:errcheck
	if out["token"] == nil || out["token"] == "" {
		t.Error("expected plaintext token in response")
	}
	if repo.created == nil || len(repo.created.Hash) == 0 {
		t.Error("expected the stored token to carry a hash")
	}
}

func TestAppTokenHandler_Create_MissingName_Returns400(t *testing.T) {
	app := buildAppTokenApp(t, &stubAppTokenRepo{})
	req := authedRequest("POST", "/api/v1/app-tokens", bytes.NewReader([]byte(`{"ttl_seconds":0}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestAppTokenHandler_Unauthenticated_Returns401(t *testing.T) {
	app := buildAppTokenApp(t, &stubAppTokenRepo{})
	resp, _ := app.Test(plainRequest("GET", "/api/v1/app-tokens", nil))
	if resp.StatusCode != 401 {
		t.Errorf("want 401 without a token, got %d", resp.StatusCode)
	}
}

func TestAppTokenHandler_List_Returns200(t *testing.T) {
	app := buildAppTokenApp(t, &stubAppTokenRepo{tokens: []*eywa.AppToken{{ID: "t1", Name: "a"}}})
	resp, _ := app.Test(authedRequest("GET", "/api/v1/app-tokens", nil))
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestAppTokenHandler_Revoke_Returns204(t *testing.T) {
	app := buildAppTokenApp(t, &stubAppTokenRepo{})
	resp, _ := app.Test(authedRequest("DELETE", "/api/v1/app-tokens/t1", nil))
	if resp.StatusCode != 204 {
		t.Errorf("want 204, got %d", resp.StatusCode)
	}
}

func TestAppTokenHandler_Revoke_NotFound_Returns404(t *testing.T) {
	app := buildAppTokenApp(t, &stubAppTokenRepo{revokeErr: eywa.ErrNotFound})
	resp, _ := app.Test(authedRequest("DELETE", "/api/v1/app-tokens/missing", nil))
	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestAppTokenHandler_Create_PersistError_Returns500(t *testing.T) {
	app := buildAppTokenApp(t, &stubAppTokenRepo{createErr: errInternal})
	req := authedRequest("POST", "/api/v1/app-tokens", bytes.NewReader([]byte(`{"name":"ci"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestAppTokenHandler_Create_InvalidBody_Returns400(t *testing.T) {
	app := buildAppTokenApp(t, &stubAppTokenRepo{})
	req := authedRequest("POST", "/api/v1/app-tokens", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestAppTokenHandler_List_Error_Returns500(t *testing.T) {
	app := buildAppTokenApp(t, &stubAppTokenRepo{listErr: errInternal})
	resp, _ := app.Test(authedRequest("GET", "/api/v1/app-tokens", nil))
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestAppTokenHandler_Revoke_Error_Returns500(t *testing.T) {
	app := buildAppTokenApp(t, &stubAppTokenRepo{revokeErr: errInternal})
	resp, _ := app.Test(authedRequest("DELETE", "/api/v1/app-tokens/t1", nil))
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

// --- event auth gating ---

func TestEventAuth_RequiresToken(t *testing.T) {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	eventValidator := eywa.NewAPIKeyValidator(map[string]string{testAdminKey: "app"})
	if err := RegisterRoutes(app, minimalTestWeave(t), RouteDeps{EventAuth: []eywa.TokenValidator{eventValidator}}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// No token → 401.
	resp, _ := app.Test(plainRequest("POST", "/api/v1/events/chat", bytes.NewReader([]byte(`{}`))))
	if resp.StatusCode != 401 {
		t.Fatalf("want 401 without a token, got %d", resp.StatusCode)
	}

	// Valid token → auth passes (handler runs; non-401 even if the event itself is rejected).
	req := authedRequest("POST", "/api/v1/events/chat", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	resp2, _ := app.Test(req)
	if resp2.StatusCode == 401 {
		t.Error("valid token must pass event auth")
	}
}

func TestEventVerifiers_HMACGatesEvents(t *testing.T) {
	secret := "ingest-secret"
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	if err := RegisterRoutes(app, minimalTestWeave(t), RouteDeps{
		EventVerifiers: []eywa.RequestVerifier{eywa.NewHMACVerifier(secret)},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	body := []byte(`{"user":"u"}`)

	// No signature → 401.
	resp, _ := app.Test(plainRequest("POST", "/api/v1/events/chat", bytes.NewReader(body)))
	if resp.StatusCode != 401 {
		t.Fatalf("want 401 without a signature, got %d", resp.StatusCode)
	}

	// Valid signature → auth passes (non-401).
	ts := time.Now().Unix()
	req := plainRequest("POST", "/api/v1/events/chat", bytes.NewReader(body))
	req.Header.Set("X-Eywa-Signature", signEywa(secret, ts, body))
	req.Header.Set("Content-Type", "application/json")
	resp2, _ := app.Test(req)
	if resp2.StatusCode == 401 {
		t.Error("valid HMAC signature must pass event auth")
	}
}

func signEywa(secret string, ts int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10) + "." + string(body)))
	return "t=" + strconv.FormatInt(ts, 10) + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestEventAuth_OpenWhenUnset(t *testing.T) {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	if err := RegisterRoutes(app, minimalTestWeave(t), RouteDeps{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	// No EventAuth → no token required; request reaches the handler (non-401).
	resp, _ := app.Test(plainRequest("POST", "/api/v1/events/chat", bytes.NewReader([]byte(`{}`))))
	if resp.StatusCode == 401 {
		t.Error("events must be open when EventAuth is unset")
	}
}
