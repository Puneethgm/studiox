package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// loadEnv pulls .env from the repo root (parent of `scripts/leads`). Tries a
// few likely locations so the script works whether you run it from the
// scripts dir or the repo root. Each call is independent — godotenv's Load
// short-circuits on the first missing file when given a list, which would
// hide the .env at ../../ when running from scripts/leads/.
func loadEnv() {
	for _, p := range []string{".env", "../.env", "../../.env"} {
		_ = godotenv.Load(p)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustConnect(ctx context.Context) *pgxpool.Pool {
	loadEnv()

	host := envOr("POSTGRES_HOST", "localhost")
	port := envOr("POSTGRES_PORT", "5434")
	user := envOr("POSTGRES_USER", "projectx")
	pass := os.Getenv("POSTGRES_PASSWORD")
	dbName := envOr("POSTGRES_DB", "projectx")
	ssl := envOr("POSTGRES_SSLMODE", "disable")

	if pass == "" {
		fmt.Fprintln(os.Stderr, "POSTGRES_PASSWORD is required (set in .env at the repo root)")
		os.Exit(2)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, pass, host, port, dbName, ssl)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db connect: %v\n", err)
		os.Exit(1)
	}
	if err := pool.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "db ping: %v\n", err)
		pool.Close()
		os.Exit(1)
	}
	return pool
}

// resolveCampaign returns (studioID, campaignID, fitnessPlans) or exits.
func resolveCampaign(ctx context.Context, pool *pgxpool.Pool, studioSlug, campaignSlug string) (uuid.UUID, uuid.UUID, []string) {
	var studioID, campaignID uuid.UUID
	var plans []string
	err := pool.QueryRow(ctx, `
		SELECT s.id, c.id, c.fitness_plans
		FROM campaigns c
		JOIN studios s ON s.id = c.studio_id
		WHERE s.slug = $1 AND c.slug = $2
	`, studioSlug, campaignSlug).Scan(&studioID, &campaignID, &plans)
	if err != nil {
		fmt.Fprintf(os.Stderr, "campaign %q under studio %q not found: %v\n",
			campaignSlug, studioSlug, err)
		os.Exit(1)
	}
	if len(plans) == 0 {
		fmt.Fprintln(os.Stderr, "campaign has no fitness plans configured — add some first via the admin UI")
		os.Exit(1)
	}
	return studioID, campaignID, plans
}
