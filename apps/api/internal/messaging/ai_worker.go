package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/projectx/api/internal/integrations/claude"
	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/studios"
	"github.com/google/uuid"
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

	// Update lead status based on sentiment
	// Update for ALL leads, not just new ones - helps track progression through pipeline
	if lead != nil {
		w.updateLeadStatus(ctx, studioID, lead, sentiment, confidence)
	}

	return nil
}

func (w *AIWorker) buildPrompt(msg *Message, conv *Conversation, lead *leads.Lead, sentiment int, keywords []string) string {
	context := "You are a helpful AI assistant for a fitness studio. "

	if lead != nil {
		context += fmt.Sprintf("Responding to %s about the %s plan. ", lead.Name, lead.FitnessPlan)
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

func (w *AIWorker) updateLeadStatus(ctx context.Context, studioID uuid.UUID, lead *leads.Lead, sentiment int, confidence float64) {
	// Only update if confidence is high enough
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
	}
}
