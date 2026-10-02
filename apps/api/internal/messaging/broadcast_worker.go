package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	broadcastPollInterval = time.Minute
	// broadcastBatchCap bounds how many recipients get enqueued in a single
	// tick even when the daily limit is 0 (unlimited) or very high — enqueue
	// is cheap but there's no reason to dump thousands of outbound_jobs rows
	// at once instead of spreading them over a few ticks.
	broadcastBatchCap = 200
)

// BroadcastWorker is the "Manual Actions" bulk-send scheduler: every minute
// it looks at every due, not-yet-fully-sent campaign and enqueues as many
// pending recipients as the studio's remaining daily WhatsApp allowance
// allows (see messaging/worker.go's existing daily-limit enforcement,
// which still applies — this worker just avoids enqueueing recipients
// doomed to dead-letter instead of pacing them).
//
// "Resume tomorrow" falls out for free: CountAutomatedWhatsAppSentToday
// already buckets by the studio's SGT calendar day, so once a new day
// starts the count resets to 0 and this worker's next tick simply enqueues
// the next slice — no separate date-rollover logic needed.
//
// It never sends anything directly — it only enqueues into the existing
// outbound_jobs queue (SourceAutomation), which the existing outbound
// worker (worker.go) actually delivers, retries, and daily-limit-enforces
// exactly as it already does for every other automation message.
type BroadcastWorker struct {
	svc  *Service
	repo *Repo
	log  *slog.Logger
}

func NewBroadcastWorker(svc *Service, repo *Repo, log *slog.Logger) *BroadcastWorker {
	return &BroadcastWorker{svc: svc, repo: repo, log: log}
}

func (w *BroadcastWorker) Run(ctx context.Context) {
	w.log.Info("Broadcast worker started", "component", "broadcast_worker", "poll_interval", broadcastPollInterval)

	w.tick(ctx)

	t := time.NewTicker(broadcastPollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("Broadcast worker stopping", "component", "broadcast_worker")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *BroadcastWorker) tick(ctx context.Context) {
	campaigns, err := w.repo.ListActiveBroadcastCampaigns(ctx)
	if err != nil {
		w.log.Warn("Broadcast worker — failed to list active campaigns, will retry next poll",
			"component", "broadcast_worker", "error", err.Error())
		return
	}
	for _, c := range campaigns {
		w.tickCampaign(ctx, c)
	}
}

func (w *BroadcastWorker) tickCampaign(ctx context.Context, c BroadcastCampaign) {
	batchSize := broadcastBatchCap

	// The configurable daily cap is specifically a WhatsApp-number-tier
	// safety rail (see GetWhatsAppDailyMessageLimit's doc comment) and
	// doesn't mean anything for plain SMTP — email pacing instead relies on
	// the same conservative per-tick batchSize cap below, same as every
	// other automated send this worker makes.
	if c.Channel != BroadcastChannelEmail {
		limit, err := w.repo.GetWhatsAppDailyMessageLimit(ctx, c.StudioID)
		if err != nil {
			w.log.Warn("Broadcast worker — failed to load daily limit, skipping this tick",
				"component", "broadcast_worker", "campaign_id", c.ID, "error", err.Error())
			return
		}
		if limit > 0 {
			sentToday, err := w.repo.CountAutomatedWhatsAppSentToday(ctx, c.StudioID)
			if err != nil {
				w.log.Warn("Broadcast worker — failed to count today's sends, skipping this tick",
					"component", "broadcast_worker", "campaign_id", c.ID, "error", err.Error())
				return
			}
			remaining := limit - sentToday
			if remaining <= 0 {
				// Limit reached for today — do nothing. Next poll (today, once
				// something frees up, or tomorrow once the SGT day rolls over
				// and the count resets) picks up right where this left off.
				return
			}
			if remaining < batchSize {
				batchSize = remaining
			}
		}
	}

	recipients, err := w.repo.NextPendingRecipients(ctx, c.ID, batchSize)
	if err != nil {
		w.log.Warn("Broadcast worker — failed to load pending recipients",
			"component", "broadcast_worker", "campaign_id", c.ID, "error", err.Error())
		return
	}
	if len(recipients) == 0 {
		// Nothing left pending — make sure the campaign reflects that
		// (covers the case where the last batch was enqueued on a
		// previous tick but progress wasn't advanced for some reason).
		if err := w.repo.AdvanceBroadcastCampaignProgress(ctx, c.ID); err != nil {
			w.log.Warn("Broadcast worker — failed to advance campaign progress",
				"component", "broadcast_worker", "campaign_id", c.ID, "error", err.Error())
		}
		return
	}

	channelKind := w.resolveChannelKind(ctx, c.StudioID, c.Channel)
	if channelKind == "" {
		w.log.Warn("Broadcast worker — no active channel for studio/channel, skipping",
			"component", "broadcast_worker", "campaign_id", c.ID, "studio_id", c.StudioID, "channel", c.Channel)
		return
	}

	enqueued := 0
	for _, recipient := range recipients {
		contactValue := recipient.Contact.Phone
		if c.Channel == BroadcastChannelEmail {
			contactValue = strings.TrimSpace(recipient.Contact.Email)
			if contactValue == "" {
				_ = w.repo.MarkRecipientFailed(ctx, recipient.RecipientID, "contact has no email address")
				continue
			}
		}

		conv, err := w.svc.CreateConversation(ctx, c.StudioID, CreateConversationInput{
			ChannelKind:  channelKind,
			ContactValue: contactValue,
			DisplayName:  recipient.Contact.Name,
		})
		if err != nil {
			w.log.Warn("Broadcast worker — failed to open conversation for recipient",
				"component", "broadcast_worker", "campaign_id", c.ID, "contact_id", recipient.Contact.ID, "error", err.Error())
			_ = w.repo.MarkRecipientFailed(ctx, recipient.RecipientID, err.Error())
			continue
		}

		jobID, err := w.repo.EnqueueOutbound(ctx, OutboundJob{
			StudioID:       c.StudioID,
			ConversationID: conv.ID,
			Subject:        c.Subject,
			Body:           c.Body,
			Attachments:    c.Attachments,
			SourceKind:     SourceAutomation,
			SourceRef:      fmt.Sprintf("broadcast:%s", c.ID),
		})
		if err != nil {
			w.log.Warn("Broadcast worker — failed to enqueue outbound job for recipient",
				"component", "broadcast_worker", "campaign_id", c.ID, "contact_id", recipient.Contact.ID, "error", err.Error())
			_ = w.repo.MarkRecipientFailed(ctx, recipient.RecipientID, err.Error())
			continue
		}

		if err := w.repo.MarkRecipientEnqueued(ctx, recipient.RecipientID, jobID); err != nil {
			w.log.Warn("Broadcast worker — failed to mark recipient enqueued",
				"component", "broadcast_worker", "campaign_id", c.ID, "error", err.Error())
		}
		enqueued++
	}

	if err := w.repo.AdvanceBroadcastCampaignProgress(ctx, c.ID); err != nil {
		w.log.Warn("Broadcast worker — failed to advance campaign progress",
			"component", "broadcast_worker", "campaign_id", c.ID, "error", err.Error())
	}

	w.log.Info("Broadcast worker cycle complete",
		"component", "broadcast_worker", "campaign_id", c.ID, "enqueued", enqueued, "batch_size", len(recipients))
}

// resolveChannelKind picks the actual connected ChannelKind for the
// campaign's chosen channel family. For WhatsApp it mirrors firstsession's
// fallback: prefer the Meta Cloud API, fall back to WhatsApp Web (QR).
// Returns "" if nothing matching is active — the studio may have
// disconnected a channel after the campaign was already scheduled.
func (w *BroadcastWorker) resolveChannelKind(ctx context.Context, studioID uuid.UUID, channel BroadcastChannel) ChannelKind {
	if channel == BroadcastChannelEmail {
		if _, err := w.repo.GetActiveChannelByKind(ctx, studioID, KindEmailSMTP); err == nil {
			return KindEmailSMTP
		}
		return ""
	}
	if _, err := w.repo.GetActiveChannelByKind(ctx, studioID, KindWhatsAppMeta); err == nil {
		return KindWhatsAppMeta
	}
	if _, err := w.repo.GetActiveChannelByKind(ctx, studioID, KindWhatsAppWeb); err == nil {
		return KindWhatsAppWeb
	}
	return ""
}
