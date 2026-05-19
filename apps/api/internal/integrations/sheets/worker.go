package sheets

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/projectx/api/internal/leads"
)

const (
	pollInterval  = 5 * time.Second
	batchSize     = 25
	maxAttempts   = 8
	destinationID = "google_sheets"
)

// Worker drains the outbox and ships rows to Google Sheets. Runs in-process
// alongside the API for L1 — split into its own deployment when load demands.
type Worker struct {
	repo   *leads.Repo
	client *Client
	logger *slog.Logger
}

func NewWorker(repo *leads.Repo, client *Client, logger *slog.Logger) *Worker {
	return &Worker{repo: repo, client: client, logger: logger}
}

// Run blocks until ctx is cancelled. Idempotent + safe to run more than once
// against the same DB (FOR UPDATE SKIP LOCKED in ClaimOutboxBatch handles it).
func (w *Worker) Run(ctx context.Context) {
	if w.client == nil {
		w.logger.Warn("sheets worker disabled — no credentials configured")
		return
	}
	w.logger.Info("sheets worker started", "poll_interval", pollInterval, "batch_size", batchSize)
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("sheets worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	items, err := w.repo.ClaimOutboxBatch(ctx, destinationID, batchSize)
	if err != nil {
		w.logger.Error("claim outbox", "err", err)
		return
	}
	for _, it := range items {
		if err := w.client.AppendLead(ctx, it.Payload); err != nil {
			attempts := it.Attempts + 1
			dead := attempts >= maxAttempts
			backoff := backoffFor(attempts)
			w.logger.Warn("sheets append failed",
				"outbox_id", it.ID, "attempts", attempts, "dead", dead, "backoff_s", backoff.Seconds(), "err", err)
			if mErr := w.repo.MarkOutboxFailed(ctx, it.ID, err.Error(), backoff, dead); mErr != nil {
				w.logger.Error("mark outbox failed", "outbox_id", it.ID, "err", mErr)
			}
			continue
		}
		if err := w.repo.MarkOutboxSent(ctx, it.ID); err != nil {
			w.logger.Error("mark outbox sent", "outbox_id", it.ID, "err", err)
		}
	}
}

// backoffFor returns an exponential backoff capped at ~30 minutes.
func backoffFor(attempts int) time.Duration {
	secs := math.Pow(2, float64(attempts))
	if secs > 1800 {
		secs = 1800
	}
	return time.Duration(secs) * time.Second
}
