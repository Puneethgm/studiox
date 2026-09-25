package messaging

import (
	"context"
	"log/slog"
	"time"
)

// coldScanInterval is how often the scanner re-derives every studio's Cold
// Leads set. Not per-studio configurable (unlike the day thresholds it
// applies) — this is an operational cadence, not a business setting, so it
// lives in code like workerPollInterval does.
const coldScanInterval = 15 * time.Minute

// ColdLeadScanner periodically recomputes which conversations qualify as
// "cold" (see RecomputeColdLeads) for every studio and persists the result
// onto conversations.cold_reason, so ListColdLeads can just read a column
// instead of recomputing live on every Pipeline load. Re-engage and the
// Pipeline's cold-column drag-and-drop clear a single conversation's flag
// immediately (see ClearColdStatus) rather than waiting for the next tick.
type ColdLeadScanner struct {
	repo *Repo
	log  *slog.Logger
}

func NewColdLeadScanner(repo *Repo, log *slog.Logger) *ColdLeadScanner {
	return &ColdLeadScanner{repo: repo, log: log}
}

func (s *ColdLeadScanner) Run(ctx context.Context) {
	s.log.Info("cold lead scanner started", "interval", coldScanInterval)
	s.tick(ctx)
	t := time.NewTicker(coldScanInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			s.log.Info("cold lead scanner stopping")
			return
		case <-t.C:
			s.tick(ctx)
		}
	}
}

func (s *ColdLeadScanner) tick(ctx context.Context) {
	studioIDs, err := s.repo.ListStudioIDs(ctx)
	if err != nil {
		s.log.Error("cold lead scanner: list studios failed", "err", err)
		return
	}
	for _, studioID := range studioIDs {
		neverReplied, stalled, err := s.repo.GetColdLeadThresholds(ctx, studioID)
		if err != nil {
			s.log.Error("cold lead scanner: get thresholds failed", "studio_id", studioID, "err", err)
			continue
		}
		if err := s.repo.RecomputeColdLeads(ctx, studioID, neverReplied, stalled); err != nil {
			s.log.Error("cold lead scanner: recompute failed", "studio_id", studioID, "err", err)
		}
	}
}
