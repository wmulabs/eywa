package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

func TestNewMessageManager_ReturnsNonNil(t *testing.T) {
	mgr := NewMessageManager(&stubEchoRepo{}, testLogger(t), noopTracer())
	if mgr == nil {
		t.Fatal("expected non-nil message manager")
	}
}

func TestMessageManager_Append_Success(t *testing.T) {
	repo := &stubEchoRepo{}
	mgr := NewMessageManager(repo, testLogger(t), noopTracer())
	msg := entities.Echo{Role: "user", Content: "hello"}
	if err := mgr.Append(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMessageManager_Append_RepoError(t *testing.T) {
	repo := &stubEchoRepo{appendErr: errors.New("db error")}
	mgr := NewMessageManager(repo, testLogger(t), noopTracer())
	msg := entities.Echo{Role: "user", Content: "hello"}
	if err := mgr.Append(context.Background(), msg); err == nil {
		t.Fatal("expected error from repo")
	}
}
