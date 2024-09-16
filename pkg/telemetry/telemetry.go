// Functions for working with OpenTelemetry across all NAIS deploy systems.

package telemetry

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/nais/deploy/pkg/pb"
	"github.com/nais/deploy/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	otrace "go.opentelemetry.io/otel/trace"
)

// How long between each time OT sends something to the collector.
const batchTimeout = 5 * time.Second

// Key for traceparent header in OT libraries.
const traceParentKey = "traceparent"

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

// Given a context and a trace parent header value, returns a new context
// that can be used to set up nested tracing.
func WithTraceParent(ctx context.Context, traceParent string) context.Context {
	traceCarrier := propagation.MapCarrier{}
	traceCarrier.Set(traceParentKey, traceParent)
	traceCtx := propagation.TraceContext{}
	return traceCtx.Extract(ctx, traceCarrier)
}

// TraceParentHeader extracts the trace parent header value from the context.
//
// A trace parent header contains the following data:
//
// Version - Trace ID - Span ID - Flags
//
//	00-3b03c24a4efad25e514890c874dc9e33-59c10f1945da62ca-01
func TraceParentHeader(ctx context.Context) string {
	traceCarrier := propagation.MapCarrier{}
	traceCtx := propagation.TraceContext{}
	traceCtx.Inject(ctx, traceCarrier)
	return traceCarrier.Get(traceParentKey)
}

// TraceID extracts the trace ID from the context.
// If the context does not embed a trace, an empty string is returned.
func TraceID(ctx context.Context) string {
	traceParentHeader := TraceParentHeader(ctx)
	parts := strings.Split(traceParentHeader, "-")
	if len(parts) != 4 {
		return ""
	}
	return parts[1]
}

// Copies interesting values from the deployment request
// onto the span, so it can be filtered in Grafana.
func AddDeploymentRequestSpanAttributes(span otrace.Span, request *pb.DeploymentRequest) {
	span.SetAttributes(
		attribute.KeyValue{
			Key:   "deploy.id",
			Value: attribute.StringValue(request.GetID()),
		}, attribute.KeyValue{
			Key:   "deploy.cluster",
			Value: attribute.StringValue(request.GetCluster()),
		}, attribute.KeyValue{
			Key:   "deploy.team",
			Value: attribute.StringValue(request.GetTeam()),
		}, attribute.KeyValue{
			Key:   "deploy.git-ref-sha",
			Value: attribute.StringValue(request.GetGitRefSha()),
		}, attribute.KeyValue{
			Key:   "deploy.repository",
			Value: attribute.StringValue(request.GetRepository().FullName()),
		},
	)
}

// Holds timestamps from pipeline indicating when certain steps were started or finished.
// If `Validate()` returns nil, this object is safe to use and contains chronologically ordered timestamps
// for every field.
type PipelineTimings struct {
	LatestCommit time.Time
	Start        time.Time
	BuildStart   time.Time
	AttestStart  time.Time
	End          time.Time
}

func (pt *PipelineTimings) Validate() error {
	if pt.LatestCommit.After(pt.BuildStart) || pt.Start.After(pt.BuildStart) || pt.BuildStart.After(pt.AttestStart) || pt.AttestStart.After(pt.End) {
		return fmt.Errorf("pipeline timings are not in expected chronological order, ensure that: latest_commit < pipeline_start < build_start < attest_start < pipeline_end")
	}
	return nil
}

func (pt *PipelineTimings) StartTracing(ctx context.Context) (context.Context, otrace.Span) {
	if pt == nil {
		return Tracer().Start(ctx, "Continuous integration pipeline", otrace.WithSpanKind(otrace.SpanKindServer))
	}

	rootCtx, rootSpan := Tracer().Start(ctx, "Continuous integration pipeline", otrace.WithTimestamp(pt.LatestCommit), otrace.WithSpanKind(otrace.SpanKindServer))
	rootSpan.AddEvent("Latest commit to repository", otrace.WithTimestamp(pt.LatestCommit))
	{
		ciCtx, ciSpan := Tracer().Start(rootCtx, "Github Action: docker-build-push", otrace.WithTimestamp(pt.Start), otrace.WithAttributes())
		{
			_, buildSpan := Tracer().Start(ciCtx, "Docker: Build and push", otrace.WithTimestamp(pt.BuildStart))
			buildSpan.End(otrace.WithTimestamp(pt.AttestStart))
		}
		{
			_, attestSpan := Tracer().Start(ciCtx, "SLSA: SBOM sign and attest", otrace.WithTimestamp(pt.AttestStart))
			attestSpan.End(otrace.WithTimestamp(pt.End))
		}
		ciSpan.End(otrace.WithTimestamp(pt.End))
	}

	return rootCtx, rootSpan
}

// Parse pipeline build timings.
//
// Uses the following input format:
//
//	latest_commit=1726040395,pipeline_start=1726050395,pipeline_end=1726050512,build_start=1726050400,attest_start=1726050492
//
// This output usually comes from `docker-build-push.steps.output.telemetry`.
//
// If there is no timing data, both return values will be nil.
// If all timing data is valid, returns a timings object and nil error.
func ParsePipelineTelemetry(s string) (*PipelineTimings, error) {
	if len(s) == 0 {
		return nil, nil
	}

	timings := &PipelineTimings{}
	fragments := strings.Split(s, ",")
	for _, keyValue := range fragments {
		key, value, found := strings.Cut(keyValue, "=")
		if !found {
			return nil, fmt.Errorf("expected 'key=value', found '%s'", keyValue)
		}

		epoch, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("expected UNIX epoch, found '%s'", value)
		}

		ts := time.Unix(int64(epoch), 0)
		ts = ts.UTC()

		switch key {
		case "latest_commit":
			timings.LatestCommit = ts
		case "pipeline_start":
			timings.Start = ts
		case "pipeline_end":
			timings.End = ts
		case "build_start":
			timings.BuildStart = ts
		case "attest_start":
			timings.AttestStart = ts
		default:
			return nil, fmt.Errorf("expected key to be one of 'latest_commit', 'pipeline_start', 'pipeline_end', 'build_start', 'attest_start'; found '%s'", key)
		}
	}
	err := timings.Validate()
	if err != nil {
		return nil, err
	}
	return timings, nil
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTraceProvider(ctx context.Context, res *resource.Resource, endpointURL string) (*trace.TracerProvider, error) {
	// When debugging, you can replace the existing exporter with this one to get JSON on stdout.
	// traceExporter, err := stdouttrace.New()

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
