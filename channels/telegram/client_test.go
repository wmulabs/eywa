package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestClient_DownloadFile_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/getFile"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"file_path":"a/b.ogg"}}`))
		case strings.Contains(r.URL.Path, "/file/bottok/a/b.ogg"):
			_, _ = w.Write([]byte("BYTES"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := NewClient("tok")
	c.baseURL = srv.URL
	data, err := c.DownloadFile(context.Background(), "fid")
	if err != nil || string(data) != "BYTES" {
		t.Fatalf("data=%q err=%v", string(data), err)
	}
}

func TestClient_DownloadFile_Errors(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"getFile status": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
		"empty path": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		},
		"bad json": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`not json`)) },
		"download status": func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/getFile") {
				_, _ = w.Write([]byte(`{"ok":true,"result":{"file_path":"a/b"}}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		},
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(h)
			defer srv.Close()
			c := NewClient("tok")
			c.baseURL = srv.URL
			if _, err := c.DownloadFile(context.Background(), "fid"); err == nil {
				t.Errorf("expected error for %s", name)
			}
		})
	}
}
