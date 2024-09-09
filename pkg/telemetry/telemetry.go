// Functions for working with OpenTelemetry across all NAIS deploy systems.

package telemetry

import (
	"context"
	"runtime"
	"time"

	"github.com/nais/deploy/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	otrace "go.opentelemetry.io/otel/trace"
)

// How long between each time OT sends something to the collector.
const batchTimeout = 5 * time.Second

// Singleton instance of the default tracer.
// Access it with `Tracer()`.
var tracer *trace.TracerProvider

// Initialize the OpenTelemetry library.
//
// You MUST call `Shutdown()` on the tracer provider before exiting,
// lest traces are not sent to the collector.
func New(ctx context.Context, serviceName string, collectorEndpointURL string) (*trace.TracerProvider, error) {
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.OSName(runtime.GOOS),
		semconv.ServiceVersion(version.Version()),
	)

	tracerProvider, err := newTraceProvider(ctx, res, collectorEndpointURL)
	if err != nil {
		return nil, err
	}

	otel.SetTracerProvider(tracerProvider)

	tracer = tracerProvider

	return tracerProvider, nil
}

// Returns the top-level tracer.
//
// "Library Name" in Grafana will be set to the default value, which currently is the path to the Go OpenTelemetry library.
//
// Panics when `New()` has not been called or returned with an error.
func Tracer() otrace.Tracer {
	if tracer == nil {
		panic("BUG: tracing not initialized, have you called New()?")
	}
	return tracer.Tracer("")
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTraceProvider(ctx context.Context, res *resource.Resource, endpointURL string) (*trace.TracerProvider, error) {
	// When debugging, you can replace the existing exporter with this one to get JSON on stdout.
	//traceExporter, err := stdouttrace.New()

	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpointURL))
	if err != nil {
		return nil, err
	}

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter,
			trace.WithBatchTimeout(batchTimeout)),
		trace.WithResource(res),
	)

	return traceProvider, nil
}
