package leads

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/projectx/api/internal/platform/mail"
	"github.com/projectx/api/internal/studios"
)

// followupDigestPollInterval mirrors messaging.debouncePollInterval's
// "tight poll for a short window" shape (internal/messaging/ai_worker.go) —
// since this worker now warns 10 minutes BEFORE a reminder fires rather
// than batching everything already overdue once a day, it has to check
// often enough that no reminder's 10-minute window is missed between polls.
const followupDigestPollInterval = 1 * time.Minute

// followupReminderLeadTime is how far ahead of a lead's reminder_at this
// worker emails a warning.
const followupReminderLeadTime = 10 * time.Minute

// FollowupDigestWorker emails each studio when one or more of its leads'
// manual follow-up reminders (leads.reminder_at/reminder_note, set from the
// Leads tab) is about to come due — within followupReminderLeadTime, not
// already overdue. This is deliberately separate from the "reminder due
// now" condition (ListLeadsFilter.ReminderDue) that powers the Dashboard's
// Reminders card and the Leads tab's reminderDue filter — those keep
// showing every still-unresolved reminder on every page load; this worker
// instead fires exactly once per reminder, via leads.reminder_notified_at
// (see ListRemindersDueWithin/MarkReminderNotified in repo.go).
type FollowupDigestWorker struct {
	leadsRepo   *Repo
	studiosRepo *studios.Repo
	mailer      *mail.Sender
	frontendURL string
	log         *slog.Logger
}

func NewFollowupDigestWorker(leadsRepo *Repo, studiosRepo *studios.Repo, mailer *mail.Sender, frontendURL string, log *slog.Logger) *FollowupDigestWorker {
	return &FollowupDigestWorker{leadsRepo: leadsRepo, studiosRepo: studiosRepo, mailer: mailer, frontendURL: frontendURL, log: log}
}

func (w *FollowupDigestWorker) Run(ctx context.Context) {
	if w.mailer == nil || !w.mailer.Enabled() {
		w.log.Info("Followup digest worker not started — SMTP not configured", "component", "followup_digest")
		return
	}
	w.log.Info("Followup digest worker started", "component", "followup_digest",
		"poll_interval", followupDigestPollInterval, "lead_time", followupReminderLeadTime)

	w.tick(ctx)

	t := time.NewTicker(followupDigestPollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("Followup digest worker stopping", "component", "followup_digest")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *FollowupDigestWorker) tick(ctx context.Context) {
	list, err := w.studiosRepo.List(ctx)
	if err != nil {
		w.log.Error("Followup digest — list studios failed", "component", "followup_digest", "error", err.Error())
		return
	}
	for _, studio := range list {
		if err := w.sendForStudio(ctx, studio); err != nil {
			w.log.Warn("Followup digest — send failed for studio, continuing", "component", "followup_digest",
				"studio_id", studio.ID, "error", err.Error())
		}
	}
}

func (w *FollowupDigestWorker) sendForStudio(ctx context.Context, studio studios.Studio) error {
	if studio.ContactEmail == "" {
		return nil // nothing to send to
	}

	upcoming, err := w.leadsRepo.ListRemindersDueWithin(ctx, studio.ID, followupReminderLeadTime)
	if err != nil {
		return fmt.Errorf("list upcoming reminders: %w", err)
	}
	if len(upcoming) == 0 {
		return nil
	}

	items := make([]mail.FollowupDigestItem, 0, len(upcoming))
	for _, l := range upcoming {
		name := strings.TrimSpace(l.Name)
		if name == "" {
			name = l.Phone
		}
		items = append(items, mail.FollowupDigestItem{
			LeadName: name,
			Note:     l.ReminderNote,
			LeadLink: fmt.Sprintf("%s/admin/studios/%s/leads/%s", strings.TrimRight(w.frontendURL, "/"), studio.ID, l.ID),
		})
	}
	leadsLink := fmt.Sprintf("%s/admin/studios/%s/leads", strings.TrimRight(w.frontendURL, "/"), studio.ID)

	if err := w.mailer.SendFollowupDigest(studio.ContactEmail, studio.Name, items, leadsLink); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	// Mark each one notified individually — a later failure partway through
	// must not re-mark leads that already got the email on this pass if the
	// process restarts, but also must not block already-sent ones.
	for _, l := range upcoming {
		if err := w.leadsRepo.MarkReminderNotified(ctx, studio.ID, l.ID); err != nil {
			w.log.Warn("Followup digest — sent but failed to mark lead notified (may re-notify on next poll)",
				"component", "followup_digest", "studio_id", studio.ID, "lead_id", l.ID, "error", err.Error())
		}
	}
	w.log.Info("Followup digest sent", "component", "followup_digest", "studio_id", studio.ID, "count", len(upcoming))
	return nil
}
