// Package langfuse provides an OpenTelemetry TracerProvider that ships Eywa's GenAI spans to Langfuse
// over OTLP/HTTP. Set it with otel.SetTracerProvider(tp) before building the Weave; no engine change
// is needed (the engine reads the global tracer lazily).
package langfuse

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	defaultHost        = "https://cloud.langfuse.com"
	defaultServiceName = "eywa"
	otlpTracesPath     = "/api/public/otel/v1/traces"
)

// Config configures the Langfuse exporter.
type Config struct {
	PublicKey   string  // Langfuse project public key (required)
	SecretKey   string  // Langfuse project secret key (required)
	Host        string  // Langfuse host URL incl. scheme; defaults to https://cloud.langfuse.com
	ServiceName string  // service.name on exported spans; defaults to "eywa"
	SampleRatio float64 // 0 or >=1 → always sample; 0<r<1 → sample that fraction of traces
}

// NewTracerProvider builds an OTLP/HTTP TracerProvider that exports spans to Langfuse. The returned
// function flushes pending spans and shuts the provider down — call it (e.g. defer) on shutdown.
func NewTracerProvider(ctx context.Context, cfg Config) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	if cfg.PublicKey == "" || cfg.SecretKey == "" {
		return nil, nil, fmt.Errorf("langfuse: public and secret keys are required")
	}

	host := cfg.Host
	if host == "" {
		host = defaultHost
	}
	parsed, err := url.Parse(host)
	if err != nil || parsed.Host == "" {
		return nil, nil, fmt.Errorf("langfuse: invalid host %q (include the scheme, e.g. https://...): %w", host, err)
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(parsed.Host),
		otlptracehttp.WithURLPath(otlpTracesPath),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Basic " + basicAuth(cfg.PublicKey, cfg.SecretKey),
		}),
	}
	if parsed.Scheme == "http" {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("langfuse: create exporter: %w", err)
	}

	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = defaultServiceName
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", serviceName))),
		sdktrace.WithSampler(sampler(cfg.SampleRatio)),
	)
	return tp, tp.Shutdown, nil
}

func sampler(ratio float64) sdktrace.Sampler {
	if ratio <= 0 || ratio >= 1 {
		return sdktrace.AlwaysSample()
	}
	return sdktrace.TraceIDRatioBased(ratio)
}

func basicAuth(publicKey, secretKey string) string {
	return base64.StdEncoding.EncodeToString([]byte(publicKey + ":" + secretKey))
}
