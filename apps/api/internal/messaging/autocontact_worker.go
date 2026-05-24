package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/projectx/api/internal/leads"
)

const (
	autoContactDestination = "lead_autocontact"
	autoPollInterval       = 5 * time.Second
	autoBatchSize          = 10
)

// AutoContactWorker processes lead.created outbox rows and seeds conversations + outbound jobs.
type AutoContactWorker struct {
	leadsRepo *leads.Repo
	msgRepo   *Repo
	msgSvc    *Service
	log       *slog.Logger
}

func NewAutoContactWorker(leadsRepo *leads.Repo, msgRepo *Repo, msgSvc *Service, logger *slog.Logger) *AutoContactWorker {
	return &AutoContactWorker{leadsRepo: leadsRepo, msgRepo: msgRepo, msgSvc: msgSvc, log: logger}
}

func (w *AutoContactWorker) Run(ctx context.Context) {
	if w.leadsRepo == nil || w.msgRepo == nil || w.msgSvc == nil {
		w.log.Warn("autocontact worker disabled — missing deps")
		return
	}
	w.log.Info("autocontact worker started", "poll_interval", autoPollInterval)
	t := time.NewTicker(autoPollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("autocontact worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *AutoContactWorker) tick(ctx context.Context) {
	items, err := w.leadsRepo.ClaimOutboxBatch(ctx, autoContactDestination, autoBatchSize)
	if err != nil {
		w.log.Error("claim autocontact outbox", "err", err)
		return
	}
	for _, it := range items {
		if err := w.processItem(ctx, it); err != nil {
			w.log.Error("process autocontact item", "outbox_id", it.ID, "err", err)
			// mark failed with backoff: reuse MarkOutboxFailed helper
			if mErr := w.leadsRepo.MarkOutboxFailed(ctx, it.ID, err.Error(), time.Minute, false); mErr != nil {
				w.log.Error("mark outbox failed", "err", mErr)
			}
		} else {
			if err := w.leadsRepo.MarkOutboxSent(ctx, it.ID); err != nil {
				w.log.Error("mark outbox sent", "err", err)
			}
		}
	}
}

func (w *AutoContactWorker) processItem(ctx context.Context, it leads.OutboxItem) error {
	var l leads.Lead
	if err := json.Unmarshal(it.Payload, &l); err != nil {
		return fmt.Errorf("decode lead payload: %w", err)
	}

	phone := sanitizePhone(l.Phone)
	if phone == "" {
		return fmt.Errorf("empty phone for lead %s", l.ID)
	}

	// Create or find conversation on WhatsApp for this studio.
	conv, err := w.msgSvc.CreateConversation(ctx, l.StudioID, CreateConversationInput{
		ChannelKind:  KindWhatsAppMeta,
		ContactValue: phone,
		DisplayName:  l.Name,
		LeadID:       &l.ID,
	})
	if err != nil {
		return fmt.Errorf("create conversation: %w", err)
	}

	// Build initial message with selection options (Interested / Not Interested)
	var body string
	if l.FitnessPlan != "" {
		body = fmt.Sprintf("Hi {{contact.first_name}}, we saw your interest in {{campaign.name}} for %s. I’m from {{studio.name}} — would you like to get started? Please select an option:\n1. Interested\n2. Not Interested", l.FitnessPlan)
	} else {
		body = "Hi {{contact.first_name}}, we saw your interest in {{campaign.name}}. I’m from {{studio.name}} — would you like to get started? Please select an option:\n1. Interested\n2. Not Interested"
	}

	// Update auto contact stage to awaiting_interest
	if err := w.leadsRepo.UpdateAutoContactStage(ctx, l.StudioID, l.ID, "awaiting_interest"); err != nil {
		w.log.Error("update lead auto contact stage failed", "lead", l.ID, "err", err)
	}

	// Enqueue immediate outbound message
	if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       l.StudioID,
		ConversationID: conv.ID,
		Body:           body,
		SourceKind:     SourceAutomation,
		SourceRef:      fmt.Sprintf("lead:%s:followup:0", l.ID.String()),
		ScheduledFor:   time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("enqueue outbound initial: %w", err)
	}

	// Mark lead contacted
	if err := w.leadsRepo.MarkLeadContacted(ctx, l.ID); err != nil {
		w.log.Error("mark lead contacted failed", "lead", l.ID, "err", err)
	}

	if l.Status == leads.StatusTrialBooked {
		// If they booked directly on registration, schedule a 1-day check-in follow-up presenting options again
		trialFollowupBody := "Hi {{contact.first_name}}, we hope you're excited for your trial! Ready to take the next step and become a member? Please select an option:\n1. Book a Trial\n2. Become a Member"
		if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
			StudioID:       l.StudioID,
			ConversationID: conv.ID,
			Body:           trialFollowupBody,
			SourceKind:     SourceAutomation,
			SourceRef:      fmt.Sprintf("lead:%s:trial_followup:1day", l.ID.String()),
			ScheduledFor:   time.Now().UTC().Add(24 * time.Hour),
		}); err != nil {
			w.log.Error("enqueue 1-day trial followup failed", "lead", l.ID, "err", err)
		}
	} else {
		// Schedule normal follow-ups: 40s, 2h, 12h
		delays := []time.Duration{40 * time.Second, 2 * time.Hour, 12 * time.Hour}
		for i, d := range delays {
			if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
				StudioID:       l.StudioID,
				ConversationID: conv.ID,
				Body:           "Just following up on your inquiry — {{contact.first_name}}",
				SourceKind:     SourceAutomation,
				SourceRef:      fmt.Sprintf("lead:%s:followup:%d", l.ID.String(), i+1),
				ScheduledFor:   time.Now().UTC().Add(d),
			}); err != nil {
				w.log.Error("schedule followup failed", "lead", l.ID, "attempt", i+1, "err", err)
			}
		}
	}

	return nil
}

var nonDigit = regexp.MustCompile(`[^0-9]+`)

func sanitizePhone(in string) string {
	s := nonDigit.ReplaceAllString(in, "")
	if s == "" {
		return ""
	}
	// If 10 digits, assume India and prefix 91
	if len(s) == 10 {
		s = "91" + s
	}
	return s
}

func firstName(full string) string {
	parts := strings.Fields(full)
	if len(parts) == 0 {
		return "there"
	}
	return parts[0]
}
