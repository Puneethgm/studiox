package sheets

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/google/uuid"
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
	repo            *leads.Repo
	client          *Client
	credentialsPath string
	lastModTime     time.Time
	logger          *slog.Logger
}

func NewWorker(repo *leads.Repo, client *Client, credentialsPath string, logger *slog.Logger) *Worker {
	var modTime time.Time
	if credentialsPath != "" {
		if info, err := os.Stat(credentialsPath); err == nil {
			modTime = info.ModTime()
		}
	}
	return &Worker{
		repo:            repo,
		client:          client,
		credentialsPath: credentialsPath,
		lastModTime:     modTime,
		logger:          logger,
	}
}

// Run blocks until ctx is cancelled. Idempotent + safe to run more than once
// against the same DB (FOR UPDATE SKIP LOCKED in ClaimOutboxBatch handles it).
func (w *Worker) Run(ctx context.Context) {
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
	// Check for credentials reload
	if w.credentialsPath != "" {
		if info, err := os.Stat(w.credentialsPath); err == nil {
			if info.ModTime().After(w.lastModTime) || w.client == nil {
				w.logger.Info("loading sheets client from credentials file", "path", w.credentialsPath)
				client, err := NewClient(ctx, w.credentialsPath)
				if err != nil {
					w.logger.Error("failed to load sheets client", "err", err)
				} else {
					w.client = client
					w.lastModTime = info.ModTime()
					w.logger.Info("sheets client loaded successfully")
				}
			}
		}
	}

	if w.client == nil {
		return
	}

	items, err := w.repo.ClaimOutboxBatch(ctx, destinationID, batchSize)
	if err != nil {
		w.logger.Error("claim outbox", "err", err)
		return
	}
	for _, it := range items {
		// Deserialize the payload to find the studio ID
		var exp LeadExport
		if err := json.Unmarshal(it.Payload, &exp); err != nil {
			w.logger.Error("unmarshal outbox lead payload", "outbox_id", it.ID, "err", err)
			_ = w.repo.MarkOutboxFailed(ctx, it.ID, err.Error(), 1*time.Minute, true)
			continue
		}

		studioID, err := uuid.Parse(exp.StudioID)
		if err != nil {
			w.logger.Error("invalid studio id in lead payload", "outbox_id", it.ID, "studio_id", exp.StudioID)
			_ = w.repo.MarkOutboxFailed(ctx, it.ID, "invalid studio ID", 1*time.Minute, true)
			continue
		}

		// Retrieve the sheets configuration for this studio
		settings, err := w.repo.GetSheetsSettings(ctx, studioID)
		if err != nil {
			w.logger.Error("failed to get sheets settings", "studio_id", studioID, "err", err)
			_ = w.repo.MarkOutboxFailed(ctx, it.ID, err.Error(), 1*time.Minute, false)
			continue
		}

		// If no active sheet config, skip appending and mark outbox sent
		if settings == nil || !settings.Active || settings.SpreadsheetID == "" {
			if err := w.repo.MarkOutboxSent(ctx, it.ID); err != nil {
				w.logger.Error("mark outbox sent", "outbox_id", it.ID, "err", err)
			}
			continue
		}

		// Ship to the studio's configured sheet
		if err := w.client.AppendLead(ctx, settings.SpreadsheetID, settings.TabName, it.Payload); err != nil {
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
