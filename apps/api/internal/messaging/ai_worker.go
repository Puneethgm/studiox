package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
		w.log.Info("claude not configured; ai worker will run using studio-configured Gemini API keys where available")
	} else {
		w.log.Info("claude configured; starting ai worker")
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

	// Get studio to access knowledge base and timezone
	studio, err := w.studiosRepo.GetByID(ctx, studioID)
	if err != nil {
		return fmt.Errorf("fetch studio for ai context: %w", err)
	}

	// Analyze sentiment and keywords
	sentiment, confidence, keywords := w.analyzeSentiment(msg.Body)
	w.log.Debug("sentiment analyzed", "message_id", msg.ID, "sentiment", sentiment, "confidence", confidence, "keywords", keywords)

	// Generate AI response with context
	prompt := w.buildPrompt(msg, conv, lead, studio, sentiment, keywords)
	
	var resp string
	var sourceRef string
	if studio.GeminiAPIKey != "" {
		w.log.Info("generating ai reply using studio gemini api key", "studio_id", studioID, "message_id", msg.ID)
		resp, err = w.generateGeminiReply(ctx, studio.GeminiAPIKey, prompt)
		sourceRef = "gemini"
	} else if w.claude != nil {
		w.log.Info("generating ai reply using claude", "studio_id", studioID, "message_id", msg.ID)
		resp, err = w.claude.GenerateReply(ctx, prompt)
		sourceRef = "claude"
	} else {
		w.log.Warn("skipping ai reply: neither claude nor studio gemini key configured", "studio_id", studioID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("ai generate reply failed: %w", err)
	}

	w.log.Info("ai response generated", "message_id", msg.ID, "response_len", len(resp), "channel", channel.Kind, "model", sourceRef)

	// Enqueue outbound reply
	if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       studioID,
		ConversationID: msg.ConversationID,
		Body:           resp,
		SourceKind:     SourceAI,
		SourceRef:      sourceRef,
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

func (w *AIWorker) buildPrompt(msg *Message, conv *Conversation, lead *leads.Lead, studio *studios.Studio, sentiment int, keywords []string) string {
	context := "You are a helpful AI assistant for a fitness studio. "

	hour := time.Now().UTC().Hour()
	if studio != nil && studio.AvailabilityTimezone != "" {
		loc, err := time.LoadLocation(studio.AvailabilityTimezone)
		if err == nil {
			hour = time.Now().In(loc).Hour()
		}
	}
	greeting := "Good evening"
	if hour < 12 {
		greeting = "Good morning"
	} else if hour < 17 {
		greeting = "Good afternoon"
	}
	context += fmt.Sprintf("Always start your reply with the appropriate greeting ('%s') based on the current local time. ", greeting)

	kbText := ""
	if studio != nil {
		kbText = studio.KnowledgeBase
		for _, f := range studio.KnowledgeBaseFiles {
			if f.Text != "" {
				kbText += fmt.Sprintf("\n\nDocument (%s):\n%s", f.Name, f.Text)
			}
		}
	}

	if kbText != "" {
		context += fmt.Sprintf("\nHere are the specific company details and knowledge base you MUST use to answer questions:\n\"\"\"\n%s\n\"\"\"\nDo not invent information outside of this knowledge base.\n\n", kbText)
	}

	if lead != nil {
		context += fmt.Sprintf("Responding to %s about the %s plan. The lead's current status is '%s'. ", lead.Name, lead.FitnessPlan, lead.Status)
		if lead.Status == leads.StatusNew || lead.Status == leads.StatusContacted {
			context += "When presenting next steps or asking the customer to decide between a trial and membership, always end your reply by offering these two clear options to choose from: '1. Book a Trial' and '2. Become a Member'. "
		} else if lead.Status == leads.StatusTrialBooked {
			context += "The customer has already booked or taken a trial. When presenting next steps, offer these two clear options to choose from: '1. Yes, I am ready to become a member!' and '2. Not right now'. "
		}
	}

	context += "Be friendly, professional, and helpful. Keep responses concise (1-3 sentences). "

	// Tailor response based on sentiment
	if sentiment == 1 {
		if lead != nil && lead.Status == leads.StatusTrialBooked {
			context += "The customer seems interested - try to get them to commit to a membership. "
		} else {
			context += "The customer seems interested - try to book a trial session or get more details. "
		}
	} else if sentiment == -1 {
		context += "The customer seems hesitant or uninterested - try to understand their concerns and address them. "
	} else {
		context += "Ask clarifying questions to understand their needs and interest level. "
	}

	if lead != nil && lead.Status == leads.StatusTrialBooked {
		return fmt.Sprintf("%s\nCustomer message: %s\n\nRespond naturally and try to move the conversation toward getting a membership commitment.", context, msg.Body)
	}
	return fmt.Sprintf("%s\nCustomer message: %s\n\nRespond naturally and try to move the conversation toward booking a trial or getting commitment.", context, msg.Body)
}

func (w *AIWorker) detectOptionChoice(body string, status leads.LeadStatus) (leads.LeadStatus, bool) {
	text := strings.ToLower(strings.TrimSpace(body))

	if status == leads.StatusTrialBooked {
		hasMemberKeywords := text == "1" ||
			strings.Contains(text, "become a member") ||
			strings.Contains(text, "ready") ||
			strings.Contains(text, "yes")
		hasDroppedKeywords := text == "2" ||
			strings.Contains(text, "not right now") ||
			strings.Contains(text, "no") ||
			strings.Contains(text, "later")

		if hasMemberKeywords && hasDroppedKeywords {
			return "", false
		}
		if hasMemberKeywords {
			return leads.StatusMember, true
		}
		if hasDroppedKeywords {
			return leads.StatusDropped, true
		}
		return "", false
	}

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
	if targetStatus, ok := w.detectOptionChoice(body, lead.Status); ok {
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

	body := "Hi {{contact.first_name}}, we hope you're enjoying your trial! Are you ready to take the next step and become a member? Please select an option:\n1. Book a Trial\n2. Become a Member"

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

func (w *AIWorker) generateGeminiReply(ctx context.Context, apiKey string, prompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", apiKey)

	reqBody, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": prompt},
				},
			},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("gemini API error (HTTP %d): %s", resp.StatusCode, string(respBytes))
	}

	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBytes, &res); err != nil {
		return "", err
	}

	if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini API")
	}

	return res.Candidates[0].Content.Parts[0].Text, nil
}
