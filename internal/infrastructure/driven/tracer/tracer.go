package tracer

import (
	"sync"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

var (
	instance trace.Tracer
	once     sync.Once
)

// Defaults to a no-op tracer. To use a real exporter, call otel.SetTracerProvider before the first call,
// or use the gcp sub-module which initialises Cloud Trace.
func GetTracer() trace.Tracer {
	once.Do(func() {
		instance = noop.NewTracerProvider().Tracer("eywa")
	})
	return instance
}
