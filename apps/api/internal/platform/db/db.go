package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func contains(s, substr string) bool { return strings.Contains(s, substr) }

func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	// Append simple_protocol so pgx skips prepared statements.
	// Required when routing through PgBouncer in transaction pool mode.
	if dsn != "" && !contains(dsn, "default_query_exec_mode") {
		sep := "?"
		if contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "default_query_exec_mode=simple_protocol"
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("new pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}
