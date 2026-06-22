package fiber

import (
	"context"
	"encoding/json"
	"testing"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
)

// stubEchoRepo is a configurable EchoRepository for message handler tests.
type stubEchoRepo struct {
	echoes   []*eywa.Echo
	count    int64
	countErr error
	findErr  error
}

func (s *stubEchoRepo) Append(_ context.Context, _ eywa.Echo) error { return nil }
func (s *stubEchoRepo) FindByMemoryKey(_ context.Context, _ string, _, _ int) ([]*eywa.Echo, error) {
	return s.echoes, s.findErr
}
func (s *stubEchoRepo) FindByMemoryKeyAndSubject(_ context.Context, _, _ string, _, _ int) ([]*eywa.Echo, error) {
	return s.echoes, s.findErr
}
func (s *stubEchoRepo) FindRecentByMemoryKey(_ context.Context, _ string, _ int) ([]*eywa.Echo, error) {
	return s.echoes, s.findErr
}
func (s *stubEchoRepo) FindRecentByMemoryKeyAndSubject(_ context.Context, _, _ string, _ int) ([]*eywa.Echo, error) {
	return s.echoes, s.findErr
}
func (s *stubEchoRepo) FindBySubjectKey(_ context.Context, _ string, _, _ int) ([]*eywa.Echo, error) {
	return s.echoes, s.findErr
}
func (s *stubEchoRepo) CountByMemoryKey(_ context.Context, _ string) (int64, error) {
	return s.count, s.countErr
}
func (s *stubEchoRepo) CountByMemoryKeyAndSubject(_ context.Context, _, _ string) (int64, error) {
	return s.count, s.countErr
}
func (s *stubEchoRepo) CountBySubjectKey(_ context.Context, _ string) (int64, error) {
	return s.count, s.countErr
}

func buildMessageTestApp(echoRepo eywa.EchoRepository, weave *eywa.Weave) *fiberlib.App {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	if err := RegisterRoutes(app, weave, RouteDeps{EchoRepo: echoRepo, APIKeys: authedAPIKeys()}); err != nil {
		panic(err)
	}
	return app
}

// --- GetMessages ---

func TestMessageHandler_MissingBothKeys_Returns400(t *testing.T) {
	weave := minimalTestWeave(t)
	app := buildMessageTestApp(&stubEchoRepo{}, weave)

	req := authedRequest("GET", "/api/v1/messages", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestMessageHandler_ByMemoryKey_Returns200(t *testing.T) {
	echoes := []*eywa.Echo{{ID: "e1"}, {ID: "e2"}}
	weave := minimalTestWeave(t)
	app := buildMessageTestApp(&stubEchoRepo{echoes: echoes, count: 2}, weave)

	req := authedRequest("GET", "/api/v1/messages?memory_key=mem1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body["items"] == nil {
		t.Error("expected items in response")
	}
}

func TestMessageHandler_BySubjectKey_Returns200(t *testing.T) {
	weave := minimalTestWeave(t)
	app := buildMessageTestApp(&stubEchoRepo{}, weave)

	req := authedRequest("GET", "/api/v1/messages?subject_key=subj1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestMessageHandler_ByBothKeys_Returns200(t *testing.T) {
	weave := minimalTestWeave(t)
	app := buildMessageTestApp(&stubEchoRepo{}, weave)

	req := authedRequest("GET", "/api/v1/messages?memory_key=mem1&subject_key=subj1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestMessageHandler_NilEchoes_ReturnsEmptySlice(t *testing.T) {
	weave := minimalTestWeave(t)
	app := buildMessageTestApp(&stubEchoRepo{}, weave) // echoes is nil

	req := authedRequest("GET", "/api/v1/messages?memory_key=mem1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	items, _ := body["items"].([]any)
	if len(items) != 0 {
		t.Errorf("expected empty items, got %v", items)
	}
}

func TestMessageHandler_CountError_Returns500(t *testing.T) {
	weave := minimalTestWeave(t)
	app := buildMessageTestApp(&stubEchoRepo{countErr: errInternal}, weave)

	req := authedRequest("GET", "/api/v1/messages?memory_key=mem1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestMessageHandler_FindError_Returns500(t *testing.T) {
	weave := minimalTestWeave(t)
	app := buildMessageTestApp(&stubEchoRepo{findErr: errInternal}, weave)

	req := authedRequest("GET", "/api/v1/messages?memory_key=mem1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestMessageHandler_SubjectKey_CountError_Returns500(t *testing.T) {
	weave := minimalTestWeave(t)
	app := buildMessageTestApp(&stubEchoRepo{countErr: errInternal}, weave)

	req := authedRequest("GET", "/api/v1/messages?subject_key=subj1", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 500 {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}
