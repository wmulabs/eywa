package gcs

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/storage"
)

type GCSVault struct {
	client     *storage.Client
	bucketName string
}

func NewGCSVault(ctx context.Context, bucketName string) (*GCSVault, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs media store: failed to create client: %w", err)
	}

	return &GCSVault{
		client:     client,
		bucketName: bucketName,
	}, nil
}

func (s *GCSVault) Store(ctx context.Context, key string, data []byte, mimeType string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("gcs media store: empty data for key %q", key)
	}

	obj := s.client.Bucket(s.bucketName).Object(key)
	w := obj.NewWriter(ctx)
	w.ContentType = mimeType

	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return "", fmt.Errorf("gcs media store: failed to write object %q: %w", key, err)
	}

	if err := w.Close(); err != nil {
		return "", fmt.Errorf("gcs media store: failed to close writer for %q: %w", key, err)
	}

	return buildPublicURL(s.bucketName, key), nil
}

func (s *GCSVault) Close() error {
	return s.client.Close()
}

func buildPublicURL(bucket, key string) string {
	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, strings.TrimPrefix(key, "/"))
}
