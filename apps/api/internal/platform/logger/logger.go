package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyTenantID
	keyUserID
)

// New builds a JSON slog logger respecting the configured level.
func New(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

func WithTenantID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyTenantID, id)
}

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyUserID, id)
}

// FromCtx returns a logger pre-attached with whichever fields are present.
func FromCtx(ctx context.Context, base *slog.Logger) *slog.Logger {
	l := base
	if v, ok := ctx.Value(keyRequestID).(string); ok && v != "" {
		l = l.With("request_id", v)
	}
	if v, ok := ctx.Value(keyTenantID).(string); ok && v != "" {
		l = l.With("tenant_id", v)
	}
	if v, ok := ctx.Value(keyUserID).(string); ok && v != "" {
		l = l.With("user_id", v)
	}
	return l
}
