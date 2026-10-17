package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestFromCtx_AttachesTraceID(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	traceID, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("traceID: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("0123456789abcdef")
	if err != nil {
		t.Fatalf("spanID: %v", err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	FromCtx(ctx, base).Info("test event")

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal log line: %v", err)
	}
	if out["trace_id"] != traceID.String() {
		t.Errorf("trace_id = %v, want %v", out["trace_id"], traceID.String())
	}
	if out["span_id"] != spanID.String() {
		t.Errorf("span_id = %v, want %v", out["span_id"], spanID.String())
	}
}

func TestFromCtx_NoSpan_OmitsTraceFields(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	FromCtx(context.Background(), base).Info("test event")

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal log line: %v", err)
	}
	if _, ok := out["trace_id"]; ok {
		t.Errorf("trace_id present with no span in context: %v", out)
	}
	if _, ok := out["span_id"]; ok {
		t.Errorf("span_id present with no span in context: %v", out)
	}
}
