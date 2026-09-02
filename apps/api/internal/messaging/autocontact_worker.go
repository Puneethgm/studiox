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
	"github.com/projectx/api/internal/studios"
)

const (
	autoContactDestination = "lead_autocontact"
	autoPollInterval       = 5 * time.Second
	autoBatchSize          = 10
)

// defaultGreetingTemplate is used when a studio hasn't set a custom greeting_message.
const defaultGreetingTemplate = "Hi {{lead_first_name}}, we saw your interest in {{studio_name}} — would you like to get started? Please select an option:\n1. Interested\n2. Not Interested"

// AutoContactWorker processes lead.created outbox rows and seeds conversations + outbound jobs.
type AutoContactWorker struct {
	leadsRepo   *leads.Repo
	msgRepo     *Repo
	msgSvc      *Service
	studiosRepo *studios.Repo
	log         *slog.Logger
}

func NewAutoContactWorker(leadsRepo *leads.Repo, msgRepo *Repo, msgSvc *Service, studiosRepo *studios.Repo, logger *slog.Logger) *AutoContactWorker {
	return &AutoContactWorker{leadsRepo: leadsRepo, msgRepo: msgRepo, msgSvc: msgSvc, studiosRepo: studiosRepo, log: logger}
}

func (w *AutoContactWorker) Run(ctx context.Context) {
	if w.leadsRepo == nil || w.msgRepo == nil || w.msgSvc == nil || w.studiosRepo == nil {
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

	// Resolve active channel for phone-based contact (WhatsApp Meta → WhatsApp Web → SMS)
	channelKind := KindWhatsAppMeta
	if _, err := w.msgRepo.GetActiveChannelByKind(ctx, l.StudioID, KindWhatsAppMeta); err != nil {
		if _, waWebErr := w.msgRepo.GetActiveChannelByKind(ctx, l.StudioID, KindWhatsAppWeb); waWebErr == nil {
			channelKind = KindWhatsAppWeb
		} else if _, smsErr := w.msgRepo.GetActiveChannelByKind(ctx, l.StudioID, KindSMS); smsErr == nil {
			channelKind = KindSMS
		} else {
			// No active channel yet — retry via the outbox backoff instead of
			// silently giving up, since a WhatsApp Web session can take a
			// couple minutes to reconnect after a drop.
			return fmt.Errorf("no active whatsapp or sms channel for studio %s", l.StudioID)
		}
	}

	// Create or find conversation and enable AI auto-reply (this lead opted in via form).
	conv, err := w.msgSvc.CreateConversation(ctx, l.StudioID, CreateConversationInput{
		ChannelKind:  channelKind,
		ContactValue: phone,
		DisplayName:  l.Name,
		LeadID:       &l.ID,
	})
	if err != nil {
		return fmt.Errorf("create conversation: %w", err)
	}

	// For leads imported from a studio's external Google Sheet, the studio
	// can opt to send only the initial greeting and leave the rest of the
	// conversation for manual/human follow-up instead of continuing
	// automatically. Every other lead source keeps today's behavior
	// (AI stays on and follow-up nudges are scheduled).
	continueAI := true
	if l.Source == "external_sheet" {
		if sheetSettings, sErr := w.leadsRepo.GetExternalLeadsSheetSettings(ctx, l.StudioID); sErr == nil && sheetSettings != nil {
			continueAI = sheetSettings.ContinueAIAfterGreeting
		}
	}

	if err := w.msgRepo.SetConversationAIEnabled(ctx, l.StudioID, conv.ID, continueAI); err != nil {
		w.log.Error("autocontact: failed to set ai_enabled for conversation", "conv", conv.ID, "err", err)
	}

	studio, err := w.studiosRepo.GetByID(ctx, l.StudioID)
	if err != nil {
		return fmt.Errorf("load studio: %w", err)
	}

	// Build initial message from the studio's greeting message (knowledge base),
	// falling back to a generic template if the studio hasn't set one.
	template := studio.GreetingMessage
	if template == "" {
		template = defaultGreetingTemplate
	}
	body := renderGreeting(template, studio, l)

	// Update auto contact stage to awaiting_interest
	if err := w.leadsRepo.UpdateAutoContactStage(ctx, l.StudioID, l.ID, "awaiting_interest"); err != nil {
		w.log.Error("update lead auto contact stage failed", "lead", l.ID, "err", err)
	}

	// Initial send can be delayed per studio (Settings → WhatsApp Message
	// Pacing) so a burst of freshly-imported leads doesn't all get contacted
	// in the same instant. Defaults to 0 (immediate) if unset.
	initialDelay := time.Duration(0)
	if minutes, dErr := w.studiosRepo.GetInitialContactDelayMinutes(ctx, l.StudioID); dErr == nil {
		initialDelay = time.Duration(minutes) * time.Minute
	}

	// Enqueue outbound message (immediate, unless a studio-configured delay applies)
	if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       l.StudioID,
		ConversationID: conv.ID,
		Body:           body,
		SourceKind:     SourceAutomation,
		SourceRef:      fmt.Sprintf("lead:%s:followup:0", l.ID.String()),
		ScheduledFor:   time.Now().UTC().Add(initialDelay),
	}); err != nil {
		return fmt.Errorf("enqueue outbound initial: %w", err)
	}

	// Mark lead contacted
	if err := w.leadsRepo.MarkLeadContacted(ctx, l.ID); err != nil {
		w.log.Error("mark lead contacted failed", "lead", l.ID, "err", err)
	}

	if !continueAI {
		// Sheet lead with "continue AI after greeting" turned off — only the
		// initial greeting goes out; no trial/no-reply follow-up nudges.
		return nil
	}

	if l.Status == leads.StatusTrialBooked {
		// If they booked directly on registration, schedule a 1-day check-in follow-up presenting options again
		trialFollowupBody := renderGreeting("Hi {{lead_first_name}}, we hope you're excited for your trial! Ready to take the next step and become a member? Please select an option:\n1. Book a Trial\n2. Become a Member", studio, l)
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
		// Schedule the studio's configured no-reply follow-up cascade (Decision
		// Trees → Follow-ups tab). Each step fires only if the lead hasn't
		// replied yet — CancelPendingFollowupJobsForLead wipes any still-pending
		// steps the moment the lead sends a genuine reply. An empty list means
		// this studio has follow-ups turned off.
		steps, err := w.msgRepo.ListFollowupSteps(ctx, l.StudioID)
		if err != nil {
			w.log.Error("load followup steps failed", "studio", l.StudioID, "err", err)
		}
		for _, s := range steps {
			body := renderGreeting(s.MessageTemplate, studio, l)
			if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
				StudioID:       l.StudioID,
				ConversationID: conv.ID,
				Body:           body,
				SourceKind:     SourceAutomation,
				SourceRef:      fmt.Sprintf("lead:%s:followup:%d", l.ID.String(), s.StepOrder),
				ScheduledFor:   time.Now().UTC().Add(time.Duration(s.DelayMinutes) * time.Minute),
			}); err != nil {
				w.log.Error("schedule followup failed", "lead", l.ID, "step", s.StepOrder, "err", err)
			}
		}
	}

	return nil
}

// renderGreeting substitutes the same {{studio_name}} / {{lead_name}} / {{lead_first_name}} /
// {{lead_status}} placeholder convention used by the AI worker's greeting-message path.
func renderGreeting(template string, studio *studios.Studio, l leads.Lead) string {
	body := template
	body = strings.ReplaceAll(body, "{{studio_name}}", studio.Name)
	body = strings.ReplaceAll(body, "{{lead_name}}", l.Name)
	body = strings.ReplaceAll(body, "{{lead_first_name}}", firstName(l.Name))
	body = strings.ReplaceAll(body, "{{lead_status}}", string(l.Status))
	return body
}

var nonDigit = regexp.MustCompile(`[^0-9]+`)

func sanitizePhone(in string) string {
	s := nonDigit.ReplaceAllString(in, "")
	if s == "" {
		return ""
	}
	// A bare 10-digit number is ambiguous: it's either a local Indian number
	// missing its country code, or a Singapore number that already has one
	// (65 + 8-digit local number = 10 digits). Only prepend 91 when it isn't
	// already a recognized country code prefix.
	if len(s) == 10 && !strings.HasPrefix(s, "65") {
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
