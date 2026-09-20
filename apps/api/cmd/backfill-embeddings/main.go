// Command backfill-embeddings re-embeds every existing studio_knowledge_chunks
// and messages row with the local embedding service, replacing Gemini-era
// vectors. Vector spaces between different embedding models aren't
// compatible — a table with a mix of old and new vectors will rank search
// results plausibly but incorrectly, so this must be run to completion in
// one sitting after switching EMBEDDINGS_SERVICE_URL over.
//
// Re-embeds content already stored in the DB in place (no re-chunking, no
// replay of the KB sync job) via a direct UPDATE — idempotent, safe to
// re-run.
//
// Usage: cd apps/api && go run ./cmd/backfill-embeddings
package main

import (
	"context"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/integrations/embeddings"
	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/db"
	"github.com/projectx/api/internal/platform/logger"
	"github.com/projectx/api/internal/studios"
)

var startedAt = time.Now()

func embeddingsServiceURL() string {
	if u := os.Getenv("EMBEDDINGS_SERVICE_URL"); u != "" {
		return u
	}
	return "http://localhost:8001"
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel)

	embClient := embeddings.New(embeddingsServiceURL())
	if embClient == nil {
		log.Error("EMBEDDINGS_SERVICE_URL not set")
		os.Exit(1)
	}

	// No timeout: this is a manual, operator-run command, not a
	// request-scoped background job — a large messages table on CPU
	// inference can legitimately take a while.
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DB.DSN())
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Sanity check the service is actually up before scanning either table.
	if _, err := embClient.EmbedPassage(ctx, "connectivity check"); err != nil {
		log.Error("embeddings service unreachable", "url", embeddingsServiceURL(), "err", err)
		os.Exit(1)
	}

	// --- studio_knowledge_chunks ---
	kcOK, kcTotal := 0, 0
	rows, err := pool.Query(ctx, `SELECT id, content FROM studio_knowledge_chunks`)
	if err != nil {
		log.Error("query studio_knowledge_chunks", "err", err)
		os.Exit(1)
	}
	for rows.Next() {
		var id uuid.UUID
		var content string
		if err := rows.Scan(&id, &content); err != nil {
			log.Error("scan studio_knowledge_chunks row", "err", err)
			continue
		}
		kcTotal++

		vec, err := embClient.EmbedPassage(ctx, content)
		if err != nil {
			log.Warn("embed knowledge chunk failed", "id", id, "err", err)
			continue
		}
		if _, err := pool.Exec(ctx,
			`UPDATE studio_knowledge_chunks SET embedding = $1::vector WHERE id = $2`,
			studios.FormatVectorAsString(vec), id,
		); err != nil {
			log.Warn("update knowledge chunk embedding failed", "id", id, "err", err)
			continue
		}
		kcOK++
		if kcOK%50 == 0 {
			log.Info("backfill progress: studio_knowledge_chunks", "done", kcOK, "seen", kcTotal)
		}
	}
	rows.Close()
	log.Info("backfilled studio_knowledge_chunks", "ok", kcOK, "seen", kcTotal)

	// --- messages ---
	// Only rows that already have an embedding (i.e. were previously
	// Gemini-embedded) — re-embed exactly that set, don't newly embed
	// messages that were never embedded before, that's the live/async
	// worker paths' job, not this backfill's.
	msgOK, msgTotal := 0, 0
	msgRows, err := pool.Query(ctx, `SELECT id, body FROM messages WHERE embedding IS NOT NULL AND body != ''`)
	if err != nil {
		log.Error("query messages", "err", err)
		os.Exit(1)
	}
	for msgRows.Next() {
		var id uuid.UUID
		var body string
		if err := msgRows.Scan(&id, &body); err != nil {
			log.Error("scan messages row", "err", err)
			continue
		}
		msgTotal++

		vec, err := embClient.EmbedPassage(ctx, body)
		if err != nil {
			log.Warn("embed message failed", "id", id, "err", err)
			continue
		}
		if _, err := pool.Exec(ctx,
			`UPDATE messages SET embedding = $1::vector WHERE id = $2`,
			studios.FormatVectorAsString(vec), id,
		); err != nil {
			log.Warn("update message embedding failed", "id", id, "err", err)
			continue
		}
		msgOK++
		if msgOK%50 == 0 {
			log.Info("backfill progress: messages", "done", msgOK, "seen", msgTotal, "elapsed", time.Since(startedAt))
		}
	}
	msgRows.Close()
	log.Info("backfilled messages", "ok", msgOK, "seen", msgTotal)

	if kcOK < kcTotal || msgOK < msgTotal {
		log.Warn("backfill completed with skipped rows — re-run to retry them",
			"knowledge_chunks_skipped", kcTotal-kcOK, "messages_skipped", msgTotal-msgOK)
		os.Exit(1)
	}
	log.Info("backfill complete", "knowledge_chunks", kcOK, "messages", msgOK, "elapsed", time.Since(startedAt))
}
