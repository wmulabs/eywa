package langfuse

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestNewTracerProvider_ExportsToLangfuse(t *testing.T) {
	var (
		mu      sync.Mutex
		gotPath string
		gotAuth string
		gotHits int
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotHits++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	tp, shutdown, err := NewTracerProvider(ctx, Config{PublicKey: "pk", SecretKey: "sk", Host: server.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, span := tp.Tracer("test").Start(ctx, "gen_ai.chat")
	span.End()

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown/flush failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotHits == 0 {
		t.Fatal("expected the exporter to POST at least one batch to Langfuse")
	}
	if gotPath != otlpTracesPath {
		t.Errorf("expected path %q, got %q", otlpTracesPath, gotPath)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("pk:sk"))
	if gotAuth != wantAuth {
		t.Errorf("expected Basic auth header %q, got %q", wantAuth, gotAuth)
	}
}

func TestNewTracerProvider_RequiresKeys(t *testing.T) {
	cases := []Config{
		{SecretKey: "sk", Host: "https://x"},
		{PublicKey: "pk", Host: "https://x"},
		{Host: "https://x"},
	}
	for _, cfg := range cases {
		if _, _, err := NewTracerProvider(context.Background(), cfg); err == nil {
			t.Errorf("expected error for missing keys: %+v", cfg)
		}
	}
}

func TestNewTracerProvider_InvalidHost(t *testing.T) {
	// No scheme → url.Parse puts it in Path, leaving Host empty.
	if _, _, err := NewTracerProvider(context.Background(), Config{PublicKey: "pk", SecretKey: "sk", Host: "cloud.langfuse.com"}); err == nil {
		t.Error("expected an error for a host without a scheme")
	}
}

func TestNewTracerProvider_DefaultHost(t *testing.T) {
	tp, shutdown, err := NewTracerProvider(context.Background(), Config{PublicKey: "pk", SecretKey: "sk"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp == nil {
		t.Fatal("expected a non-nil tracer provider")
	}
	_ = shutdown(context.Background())
}

func TestSampler(t *testing.T) {
	cases := []struct {
		ratio float64
		want  string
	}{
		{0, sdktrace.AlwaysSample().Description()},
		{1, sdktrace.AlwaysSample().Description()},
		{1.5, sdktrace.AlwaysSample().Description()},
		{0.25, sdktrace.TraceIDRatioBased(0.25).Description()},
	}
	for _, tc := range cases {
		if got := sampler(tc.ratio).Description(); got != tc.want {
			t.Errorf("sampler(%v).Description() = %q, want %q", tc.ratio, got, tc.want)
		}
	}
}

func TestBasicAuth(t *testing.T) {
	got := basicAuth("pk", "sk")
	want := base64.StdEncoding.EncodeToString([]byte("pk:sk"))
	if got != want {
		t.Errorf("basicAuth = %q, want %q", got, want)
	}
	if decoded, _ := base64.StdEncoding.DecodeString(got); !strings.Contains(string(decoded), "pk:sk") {
		t.Errorf("decoded auth %q missing credentials", decoded)
	}
}
