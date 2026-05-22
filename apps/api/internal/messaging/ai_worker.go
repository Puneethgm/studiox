package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/projectx/api/internal/integrations/claude"
	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/studios"
)

const (
	aiResyncInterval = 30 * time.Second
)

type AIWorker struct {
	bus         Bus
	msgRepo     *Repo
	msgSvc      *Service
	studiosRepo *studios.Repo
	leadsRepo   *leads.Repo
	claude      *claude.Client
	log         *slog.Logger
	subs        map[uuid.UUID]func()
}

func NewAIWorker(bus Bus, msgRepo *Repo, msgSvc *Service, studiosRepo *studios.Repo, leadsRepo *leads.Repo, cl *claude.Client, log *slog.Logger) *AIWorker {
	return &AIWorker{bus: bus, msgRepo: msgRepo, msgSvc: msgSvc, studiosRepo: studiosRepo, leadsRepo: leadsRepo, claude: cl, log: log, subs: make(map[uuid.UUID]func())}
}

func (w *AIWorker) Run(ctx context.Context) {
	if w.claude == nil {
		w.log.Info("claude not configured; ai worker disabled")
		return
	}
	w.log.Info("ai worker started")
	t := time.NewTicker(aiResyncInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("ai worker stopping")
			// unsubscribe
			for _, u := range w.subs {
				u()
			}
			return
		case <-t.C:
			w.syncSubscriptions(ctx)
		}
	}
}

func (w *AIWorker) syncSubscriptions(ctx context.Context) {
	list, err := w.studiosRepo.List(ctx)
	if err != nil {
		w.log.Error("list studios for ai worker", "err", err)
		return
	}
	// subscribe to any new studios
	for _, s := range list {
		if _, ok := w.subs[s.ID]; ok {
			continue
		}
		ch, unsub := w.bus.Subscribe(s.ID)
		w.subs[s.ID] = unsub
		go w.listenStudio(ctx, s.ID, ch)
		w.log.Info("ai worker subscribed to studio", "studio", s.ID)
	}
}

func (w *AIWorker) listenStudio(ctx context.Context, studioID uuid.UUID, ch <-chan Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			if evt.Kind != EvtMessageReceived || evt.MessageID == nil {
				continue
			}
			go func(msgID uuid.UUID) {
				if err := w.handleMessage(ctx, studioID, msgID); err != nil {
					w.log.Error("ai handle message", "err", err)
				}
			}(*evt.MessageID)
		}
	}
}

// analyzeSentiment returns: sentiment (-1=negative, 0=neutral, 1=positive), confidence score, and detected keywords
func (w *AIWorker) analyzeSentiment(text string) (int, float64, []string) {
	text = strings.ToLower(text)

	positiveKeywords := []string{"yes", "interested", "great", "love", "good", "perfect", "thanks", "thank you", "definitely", "sure", "count me in", "sign me up", "book it"}
	negativeKeywords := []string{"no", "not interested", "bad", "hate", "no thanks", "never", "not now", "maybe later", "skip", "cancel"}

	var detectedKeywords []string
	positiveScore := 0.0
	negativeScore := 0.0

	for _, kw := range positiveKeywords {
		if strings.Contains(text, kw) {
			positiveScore += 1.0
			detectedKeywords = append(detectedKeywords, kw)
		}
	}

	for _, kw := range negativeKeywords {
		if strings.Contains(text, kw) {
			negativeScore += 1.0
			detectedKeywords = append(detectedKeywords, kw)
		}
	}

	sentiment := 0
	confidence := 0.0

	if positiveScore > negativeScore {
		sentiment = 1
		confidence = positiveScore / (positiveScore + negativeScore + 1)
	} else if negativeScore > positiveScore {
		sentiment = -1
		confidence = negativeScore / (positiveScore + negativeScore + 1)
	} else {
		confidence = 0.5
	}

	return sentiment, confidence, detectedKeywords
}

func (w *AIWorker) handleMessage(ctx context.Context, studioID uuid.UUID, messageID uuid.UUID) error {
	msg, err := w.msgRepo.GetMessageByID(ctx, studioID, messageID)
	if err != nil {
		return fmt.Errorf("fetch message: %w", err)
	}
	if msg == nil {
		return nil
	}
	// Only respond to customer inbound messages
	if msg.Direction != DirectionInbound || msg.SourceKind != SourceCustomer {
		return nil
	}

	// Get conversation context
	conv, err := w.msgRepo.GetConversation(ctx, studioID, msg.ConversationID)
	if err != nil {
		return fmt.Errorf("fetch conversation: %w", err)
	}
	if conv == nil {
		return nil
	}

	// Get channel details to check if we should reply
	channel, err := w.msgRepo.GetChannelByID(ctx, studioID, conv.ChannelAccountID)
	if err != nil {
		return fmt.Errorf("fetch channel: %w", err)
	}
	if channel == nil {
		return nil
	}

	// Only reply on WhatsApp and Facebook Messenger
	if channel.Kind != KindWhatsAppMeta && channel.Kind != KindMessengerMeta {
		w.log.Debug("skipping ai reply", "channel", channel.Kind, "reason", "not whatsapp or messenger")
		return nil
	}

	// Get lead associated with this conversation (if any)
	var lead *leads.Lead
	if conv.LeadID != nil {
		lead, err = w.leadsRepo.GetLead(ctx, studioID, *conv.LeadID)
		if err != nil {
			w.log.Error("fetch lead for ai context", "err", err)
		}
	}

	// Analyze sentiment and keywords
	sentiment, confidence, keywords := w.analyzeSentiment(msg.Body)
	w.log.Debug("sentiment analyzed", "message_id", msg.ID, "sentiment", sentiment, "confidence", confidence, "keywords", keywords)

	// Generate AI response with context
	prompt := w.buildPrompt(msg, conv, lead, sentiment, keywords)
	resp, err := w.claude.GenerateReply(ctx, prompt)
	if err != nil {
		return fmt.Errorf("claude generate: %w", err)
	}

	w.log.Info("ai response generated", "message_id", msg.ID, "response_len", len(resp), "channel", channel.Kind)

	// Enqueue outbound reply
	if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       studioID,
		ConversationID: msg.ConversationID,
		Body:           resp,
		SourceKind:     SourceAI,
		SourceRef:      "claude",
		ScheduledFor:   time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("enqueue ai outbound: %w", err)
	}

	// Update lead status based on sentiment or explicit choices
	// Update for ALL leads, not just new ones - helps track progression through pipeline
	if lead != nil {
		w.updateLeadStatus(ctx, studioID, lead, msg.Body, sentiment, confidence)
	}

	return nil
}

func (w *AIWorker) buildPrompt(msg *Message, conv *Conversation, lead *leads.Lead, sentiment int, keywords []string) string {
	context := "You are a helpful AI assistant for a fitness studio. "

	if lead != nil {
		context += fmt.Sprintf("Responding to %s about the %s plan. The lead's current status is '%s'. ", lead.Name, lead.FitnessPlan, lead.Status)
		if lead.Status == leads.StatusNew || lead.Status == leads.StatusContacted {
			context += "When presenting next steps or asking the customer to decide between a trial and membership, always end your reply by offering these two clear options to choose from: '1. Book a Trial' and '2. Become a Member'. "
		}
	}

	context += "Be friendly, professional, and helpful. Keep responses concise (1-3 sentences). "

	// Tailor response based on sentiment
	if sentiment == 1 {
		context += "The customer seems interested - try to book a trial session or get more details. "
	} else if sentiment == -1 {
		context += "The customer seems hesitant or uninterested - try to understand their concerns and address them. "
	} else {
		context += "Ask clarifying questions to understand their needs and interest level. "
	}

	return fmt.Sprintf("%s\nCustomer message: %s\n\nRespond naturally and try to move the conversation toward booking a trial or getting commitment.", context, msg.Body)
}

func (w *AIWorker) detectOptionChoice(body string) (leads.LeadStatus, bool) {
	text := strings.ToLower(strings.TrimSpace(body))

	hasTrialKeywords := text == "1" ||
		strings.Contains(text, "book a trial") ||
		strings.Contains(text, "book trial") ||
		strings.Contains(text, "take a trial") ||
		strings.Contains(text, "take trial") ||
		strings.Contains(text, "trial booked") ||
		strings.Contains(text, "trial booking") ||
		strings.Contains(text, "trial")

	hasMemberKeywords := text == "2" ||
		strings.Contains(text, "become a member") ||
		strings.Contains(text, "become member") ||
		strings.Contains(text, "becoming a member") ||
		strings.Contains(text, "membership") ||
		strings.Contains(text, "member")

	// If both types of keywords are present (e.g. asking a question comparing them), it's ambiguous.
	if hasTrialKeywords && hasMemberKeywords {
		return "", false
	}

	if hasTrialKeywords {
		return leads.StatusTrialBooked, true
	}
	if hasMemberKeywords {
		return leads.StatusMember, true
	}
	return "", false
}

func (w *AIWorker) updateLeadStatus(ctx context.Context, studioID uuid.UUID, lead *leads.Lead, body string, sentiment int, confidence float64) {
	// 1. Check for explicit option choices first
	if targetStatus, ok := w.detectOptionChoice(body); ok {
		if lead.Status != targetStatus {
			err := w.leadsRepo.UpdateStatus(ctx, studioID, lead.ID, targetStatus)
			if err != nil {
				w.log.Error("update lead status via choice selection", "lead", lead.ID, "target", targetStatus, "err", err)
			} else {
				w.log.Info("lead status auto-updated (choice selection)", "lead", lead.ID, "from", lead.Status, "to", targetStatus)
				if targetStatus == leads.StatusTrialBooked {
					w.scheduleTrialFollowup(ctx, studioID, lead, lead.ID)
				}
			}
		}
		return
	}

	// 2. Only update if confidence is high enough
	if confidence < 0.6 {
		return
	}

	// Determine new status based on sentiment and current status
	var newStatus leads.LeadStatus
	var shouldUpdate bool

	if sentiment == 1 {
		// Positive sentiment - progress lead forward
		switch lead.Status {
		case leads.StatusNew:
			newStatus = leads.StatusContacted
			shouldUpdate = true
		case leads.StatusContacted:
			newStatus = leads.StatusTrialBooked
			shouldUpdate = true
		case leads.StatusTrialBooked:
			newStatus = leads.StatusMember
			shouldUpdate = true
		case leads.StatusMember, leads.StatusDropped:
			// Already at final status
			shouldUpdate = false
		}
	} else if sentiment == -1 {
		// Negative sentiment - mark as dropped (unless already completed)
		switch lead.Status {
		case leads.StatusMember:
			// Don't drop members
			shouldUpdate = false
		default:
			newStatus = leads.StatusDropped
			shouldUpdate = true
		}
	} else {
		// Neutral - only move if New
		if lead.Status == leads.StatusNew {
			newStatus = leads.StatusContacted
			shouldUpdate = true
		}
	}

	if !shouldUpdate {
		return
	}

	err := w.leadsRepo.UpdateStatus(ctx, studioID, lead.ID, newStatus)
	if err != nil {
		w.log.Error("update lead status", "lead", lead.ID, "current", lead.Status, "new", newStatus, "err", err)
	} else {
		w.log.Info("lead status auto-updated", "lead", lead.ID, "from", lead.Status, "to", newStatus, "sentiment", sentiment, "confidence", confidence)
		if newStatus == leads.StatusTrialBooked {
			w.scheduleTrialFollowup(ctx, studioID, lead, lead.ID)
		}
	}
}

func (w *AIWorker) scheduleTrialFollowup(ctx context.Context, studioID uuid.UUID, lead *leads.Lead, fallbackConvID uuid.UUID) {
	// Try to find the actual conversation ID for this lead to avoid any mismatch.
	convID := fallbackConvID
	err := w.msgRepo.Pool().QueryRow(ctx, `
		SELECT id FROM conversations 
		WHERE studio_id = $1 AND lead_id = $2
		ORDER BY updated_at DESC
		LIMIT 1
	`, studioID, lead.ID).Scan(&convID)
	if err != nil {
		w.log.Warn("could not find conversation for lead to schedule followup", "lead", lead.ID, "err", err)
	}

	first := firstName(lead.Name)
	body := fmt.Sprintf("Hi %s, we hope you're enjoying your trial! Are you ready to take the next step and become a member? Please select an option:\n1. Book a Trial\n2. Become a Member", first)

	if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       studioID,
		ConversationID: convID,
		Body:           body,
		SourceKind:     SourceAutomation,
		SourceRef:      fmt.Sprintf("lead:%s:trial_followup:1day", lead.ID.String()),
		ScheduledFor:   time.Now().UTC().Add(24 * time.Hour),
	}); err != nil {
		w.log.Error("enqueue 1-day trial followup failed", "lead", lead.ID, "err", err)
	}
}
