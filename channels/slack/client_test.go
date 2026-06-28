package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_PostMessage_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := NewClient("tok")
	c.baseURL = srv.URL
	if err := c.PostMessage(context.Background(), "C1", "hi"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_PostMessage_Errors(t *testing.T) {
	t.Run("ok false", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"ok":false,"error":"not_in_channel"}`))
		}))
		defer srv.Close()
		c := NewClient("tok")
		c.baseURL = srv.URL
		if err := c.PostMessage(context.Background(), "C1", "hi"); err == nil {
			t.Error("expected error on ok:false")
		}
	})
	t.Run("non-200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer srv.Close()
		c := NewClient("tok")
		c.baseURL = srv.URL
		if err := c.PostMessage(context.Background(), "C1", "hi"); err == nil {
			t.Error("expected error on non-200")
		}
	})
	t.Run("transport", func(t *testing.T) {
		c := NewClient("tok")
		c.baseURL = "http://127.0.0.1:0"
		if err := c.PostMessage(context.Background(), "C1", "hi"); err == nil {
			t.Error("expected transport error")
		}
	})
	t.Run("bad json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`not json`))
		}))
		defer srv.Close()
		c := NewClient("tok")
		c.baseURL = srv.URL
		if err := c.PostMessage(context.Background(), "C1", "hi"); err == nil {
			t.Error("expected parse error")
		}
	})
}

func TestClient_DownloadFile_HostGuard(t *testing.T) {
	c := NewClient("tok")
	if _, err := c.DownloadFile(context.Background(), "https://evil.example.com/x"); err == nil {
		t.Error("must refuse non-slack host")
	}
	if _, err := c.DownloadFile(context.Background(), "://bad"); err == nil {
		t.Error("must reject unparseable url")
	}
}

func TestClient_DownloadFile_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("DATA"))
	}))
	defer srv.Close()
	c := NewClient("tok")
	c.skipHostCheck = true
	data, err := c.DownloadFile(context.Background(), srv.URL+"/f")
	if err != nil || string(data) != "DATA" {
		t.Fatalf("data=%q err=%v", string(data), err)
	}
}

func TestClient_DownloadFile_Errors(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()
		c := NewClient("tok")
		c.skipHostCheck = true
		if _, err := c.DownloadFile(context.Background(), srv.URL+"/f"); err == nil {
			t.Error("expected error on non-200")
		}
	})
	t.Run("transport", func(t *testing.T) {
		c := NewClient("tok")
		c.skipHostCheck = true
		if _, err := c.DownloadFile(context.Background(), "http://127.0.0.1:0/f"); err == nil {
			t.Error("expected transport error")
		}
	})
}

func TestIsSlackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"slack.com":         true,
		"files.slack.com":   true,
		"evil.com":          false,
		"slack.com.evil.io": false,
	} {
		if got := isSlackHost(host); got != want {
			t.Errorf("isSlackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
