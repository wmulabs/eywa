package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_SendMessage_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewClient("tok")
	c.baseURL = srv.URL
	if err := c.SendMessage(context.Background(), 1, "hi"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_SendMessage_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"bad chat"}`))
	}))
	defer srv.Close()

	c := NewClient("tok")
	c.baseURL = srv.URL
	if err := c.SendMessage(context.Background(), 1, "hi"); err == nil {
		t.Error("expected error on non-200 status")
	}
}

func TestClient_SendMessage_TransportError(t *testing.T) {
	c := NewClient("tok")
	c.baseURL = "http://127.0.0.1:0" // invalid port -> dial error
	if err := c.SendMessage(context.Background(), 1, "hi"); err == nil {
		t.Error("expected transport error")
	}
}
