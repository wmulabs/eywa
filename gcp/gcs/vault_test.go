package gcs

import (
	"context"
	"testing"
)

// --- buildPublicURL ---

func TestBuildPublicURL_NormalKey(t *testing.T) {
	got := buildPublicURL("my-bucket", "media/file.jpg")
	want := "https://storage.googleapis.com/my-bucket/media/file.jpg"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestBuildPublicURL_KeyWithLeadingSlash(t *testing.T) {
	got := buildPublicURL("my-bucket", "/media/file.jpg")
	want := "https://storage.googleapis.com/my-bucket/media/file.jpg"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestBuildPublicURL_EmptyKey(t *testing.T) {
	got := buildPublicURL("my-bucket", "")
	want := "https://storage.googleapis.com/my-bucket/"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// --- Store guard clause ---

func TestStore_EmptyData_ReturnsErrorBeforeAPI(t *testing.T) {
	// nil client is fine — empty data guard returns before any client call.
	vault := &GCSVault{client: nil, bucketName: "test-bucket"}
	_, err := vault.Store(context.Background(), "key/file.jpg", nil, "image/jpeg")
	if err == nil {
		t.Error("expected error for empty data")
	}
}
