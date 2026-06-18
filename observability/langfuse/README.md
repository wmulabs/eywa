# eywa — Langfuse exporter

Ships Eywa's GenAI spans to [Langfuse](https://langfuse.com) over OTLP/HTTP. Combined with the
`gen_ai.*` semantic-convention attributes Eywa already emits, a turn shows up in Langfuse as a trace
with per-LLM-call spans (model, tokens, finish reason), tool spans, and turn-level metadata.

```go
import (
    "go.opentelemetry.io/otel"
    "github.com/wmulabs/eywa/observability/langfuse"
)

tp, shutdown, err := langfuse.NewTracerProvider(ctx, langfuse.Config{
    PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
    SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
    Host:      "https://cloud.langfuse.com", // or your self-hosted URL
    // ServiceName: "my-agent",  // optional; defaults to "eywa"
    // SampleRatio: 0.1,         // optional; 0 / >=1 = always sample
})
if err != nil { /* handle */ }
defer shutdown(context.Background())

otel.SetTracerProvider(tp) // before building the Weave

// Optional: capture prompt/response text on spans (off by default — PII-sensitive).
eywa.SetTraceContentCapture(true)
```

No engine change is needed — the Weave reads the global tracer lazily, so setting the provider before
`Build()` is enough.

## Notes

- Uses Langfuse's native OTLP endpoint (`/api/public/otel/v1/traces`) with Basic auth derived from the
  public/secret keys. Works against Langfuse Cloud or a self-hosted instance.
- Spans are batched and exported asynchronously; `shutdown` flushes pending spans. Export failures
  never block or fail a turn.
- The same provider works with any OTLP backend (Phoenix, Jaeger, Grafana Tempo) — point `Host` at it.
