package logger

import (
	"context"
	"log/slog"
	"os"
	"time"
)

type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyTenantID
	keyUserID
)

// sgt is Singapore Standard Time (UTC+8). All log timestamps use this zone
// so timestamps match what the team sees in local dashboards and Glofox.
var sgt = time.FixedZone("SGT", 8*60*60)

// New builds a JSON slog logger respecting the configured level.
// Timestamps appear as "2006-01-02 15:04:05 SGT" — human-readable, local time.
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
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().In(sgt).Format("2006-01-02 15:04:05 SGT"))
			}
			return a
		},
	})
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
