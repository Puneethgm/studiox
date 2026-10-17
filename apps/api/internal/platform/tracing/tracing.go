// Package tracing wires the Go API into the OTel Collector + Tempo stack
// (deploy/otel, deploy/tempo) so every request gets a trace_id that's
// visible both in Grafana's "Channel: API" dashboard logs panel and, via
// that same trace_id, in the Tempo trace view (Loki's derivedFields link
// one to the other — see deploy/grafana/provisioning/datasources/loki.yml).
package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Init connects the global TracerProvider to the OTLP/gRPC collector at
// endpoint (host:port, no scheme — e.g. "otel-collector:4317"). An empty
// endpoint is a deliberate no-op: it returns a shutdown func that does
// nothing, so local dev without the observability stack running never
// blocks on a collector that isn't there.
func Init(ctx context.Context, serviceName, endpoint string) (shutdown func(context.Context) error, err error) {
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(), // collector is backend-network-only, same trust boundary as Redis/Postgres
		otlptracegrpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp.Shutdown, nil
}
