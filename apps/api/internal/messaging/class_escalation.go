package messaging

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/leads"
)

// The AI has no booking system behind it: it cannot book, cancel, move or check
// availability for a class. When a customer asks for any of that, or asks a
// class-related question the knowledge base can't answer, the right move is to hand
// the conversation to a person instead of guessing (or worse, claiming it has "locked
// them in").

var (
	// classActionVerbs are verbs for doing something to a class booking.
	classActionVerbs = regexp.MustCompile(`\b(book|booked|booking|reserve|reserved|cancel|cancelled|canceled|reschedul\w*|re-schedul\w*|move|swap|switch|change|extend|freeze|transfer)\b`)
	// classNouns are the things those verbs apply to.
	classNouns = regexp.MustCompile(`\b(class|classes|session|sessions|slot|slots|lesson|lessons|credit|credits|appointment)\b`)
	// classTopicWords mark a message as being about classes/timetable at all.
	classTopicWords = regexp.MustCompile(`\b(class|classes|session|sessions|slot|slots|timetable|schedule|availability|available|spots?|credit|credits)\b`)
	// questionWords mark a message as asking or requesting something (as opposed to
	// merely mentioning a class in passing, like "thanks, see you at class").
	// "do"/"does" are bare (not "do you") so WhatsApp-style "do u have..." /
	// "does u" still matches — real messages abbreviate "you" to "u" constantly, and
	// requiring the full word silently reclassified real questions as statements.
	questionWords = regexp.MustCompile(`\b(can|could|how|when|what|where|which|who|do|does|is there|are there|any|please|want|wanna|would like|need|looking for|interested|help)\b`)
)

// mentionsTrial reports whether the message talks about a trial. Trial booking has its
// own scripted flow (the Book a Trial / Become a Member menu), so it is not escalated here.
func mentionsTrial(lower string) bool {
	return strings.Contains(lower, "trial") || strings.Contains(lower, "trail")
}

// isClassActionRequest reports whether the customer is asking to book, cancel,
// reschedule, move or otherwise change a class or session. The AI can't do any of
// these, so this always goes to a human, whatever the knowledge base says.
func isClassActionRequest(body string) bool {
	lower := strings.ToLower(body)
	if mentionsTrial(lower) {
		return false
	}
	return classActionVerbs.MatchString(lower) && classNouns.MatchString(lower)
}

// isClassRelatedQuery reports whether the message is a question or request about
// classes, sessions or the timetable. Combined with a weak knowledge-base match it
// means "don't guess, escalate". Plain thank-yous and passing mentions don't count.
func isClassRelatedQuery(body string) bool {
	lower := strings.ToLower(body)
	if !classTopicWords.MatchString(lower) {
		return false
	}
	hasQuestionMark := strings.Contains(lower, "?")
	if !hasQuestionMark && (strings.Contains(lower, "thank") || strings.Contains(lower, "thx")) {
		return false
	}
	return hasQuestionMark || questionWords.MatchString(lower)
}

// complaintPhrases signal a complaint, negative feedback, a refund demand or a threat of
// a bad review. These always go to a person: a bot should never argue with, or
// smooth over, an unhappy customer. The list is deliberately explicit (no bare
// "unfortunately", which appears in ordinary "I can't make it" messages); politely
// worded criticism that these miss is caught by the classifier's complaint_or_feedback
// intent when retrieval is available.
var complaintPhrases = []string{
	"bad review", "negative review", "leave a review", "write a review", "post a review",
	"google review", "1 star", "1-star", "one star", "review online",
	"complain", "complaint", "unacceptable", "disappointed", "disappointing",
	"dissatisfied", "not satisfied", "not happy", "unhappy", "let down", "not impressed",
	"not what i expected", "didn't live up", "did not live up", "could be better", "should be better",
	"bad experience", "negative experience", "terrible", "awful", "horrible", "worst",
	"rude", "unprofessional", "poor service", "poor quality", "waste of money", "rip off", "ripoff", "scam",
	"refund", "lawyer", "legal action", "consumer",
}

// isComplaintOrNegativeFeedback reports whether the message reads as a complaint,
// negative feedback or a bad-review threat.
func isComplaintOrNegativeFeedback(body string) bool {
	lower := strings.ToLower(body)
	for _, p := range complaintPhrases {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// complaintHandoffBody is sent when a complaint is escalated: acknowledge, don't defend.
const complaintHandoffBody = "I'm really sorry to hear that, and thank you for telling us. I've asked a team member to look into this personally and they'll be in touch with you shortly."

// escalateWithHandoff marks the conversation escalated (which also turns the AI off for
// it and emails the studio) and queues a short handoff message to the customer.
func (w *AIWorker) escalateWithHandoff(ctx context.Context, studioID, convID uuid.UUID, reason, body, sourceRef string, delay time.Duration) {
	if err := w.msgSvc.EscalateAndNotify(ctx, studioID, convID, reason); err != nil {
		w.log.Warn("failed to mark conversation escalated", "studio_id", studioID, "conv_id", convID, "reason", reason, "err", err)
		return
	}
	if _, err := w.msgRepo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       studioID,
		ConversationID: convID,
		Body:           body,
		SourceKind:     SourceAI,
		SourceRef:      sourceRef,
		ScheduledFor:   time.Now().UTC().Add(delay),
	}); err != nil {
		w.log.Error("failed to enqueue handoff message", "err", err, "conv_id", convID, "source_ref", sourceRef)
		return
	}
	w.bus.Publish(ctx, Event{
		Kind:           EvtOutboundJobEnqueued,
		StudioID:       studioID,
		ConversationID: convID,
	})
}

// escalateKnowledgeGap is the shared handoff for every "the AI doesn't actually have
// this" case (low Gemini confidence, the deterministic kb-coverage pre-check, or the
// AI's own reply admitting it doesn't know) — logs the question to the Needs Answers
// tab (best-effort) and hands the conversation to a person with the same apologetic
// reply, regardless of which check caught it or which model was involved.
func (w *AIWorker) escalateKnowledgeGap(ctx context.Context, studioID uuid.UUID, conv *Conversation, lead *leads.Lead, question, reason, sourceRef string, replyDelay time.Duration) {
	var leadID *uuid.UUID
	if lead != nil {
		leadID = &lead.ID
	}
	convID := conv.ID
	if err := w.studiosRepo.CreateKnowledgeGap(ctx, studioID, &convID, leadID, question); err != nil {
		w.log.Warn("failed to log knowledge gap", "studio_id", studioID, "conversation_id", conv.ID, "err", err)
	}
	w.escalateWithHandoff(ctx, studioID, conv.ID, reason,
		"That's a great question — let me get one of our team members to help you with the details. They'll be with you shortly!",
		sourceRef, replyDelay)
}

// aiDoesNotKnowPhrases catches a model's reply admitting it has no grounding for the
// answer — the instructed phrase ("I don't have that on file") plus common hedges models
// produce on their own instead.
var aiDoesNotKnowPhrases = []string{
	"don't have that on file", "do not have that on file",
	"don't have details about", "do not have details about",
	"don't have information about", "do not have information about",
	"don't have that information", "do not have that information",
	"don't have specific details", "do not have specific details",
	"i'm not sure about that", "i am not sure about that",
	"i don't have access to that", "i do not have access to that",
	"not something i have information on", "not something i have details on",
}

// looksLikeAIDoesNotKnow reports whether resp reads as the AI admitting it doesn't
// know the answer rather than actually answering. Used as a last-resort, provider-
// agnostic escalation trigger (see the call site in handleMessage) for studios
// without a Gemini key, where the classifier-driven low-confidence escalation never
// runs. A false positive here just means an extra escalation, not a wrong answer
// sent — an acceptable trade-off.
func looksLikeAIDoesNotKnow(resp string) bool {
	lower := strings.ToLower(resp)
	for _, p := range aiDoesNotKnowPhrases {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// kbStopWords are skipped when checking whether the knowledge base covers a
// question — short, topic-free words that would make almost any sentence look
// "covered". Not a full NLP stopword list, just enough for the short questions
// customers actually send over WhatsApp.
var kbStopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true, "do": true, "does": true,
	"you": true, "your": true, "yours": true, "i": true, "we": true, "us": true,
	"can": true, "could": true, "would": true, "will": true, "to": true, "of": true,
	"in": true, "on": true, "at": true, "for": true, "and": true, "or": true,
	"what": true, "when": true, "where": true, "how": true, "who": true, "which": true,
	"please": true, "me": true, "my": true, "it": true, "that": true, "this": true,
	"have": true, "has": true, "had": true, "there": true, "any": true, "about": true,
	"with": true, "from": true, "if": true, "but": true, "not": true, "all": true,
	"just": true, "also": true, "want": true, "need": true, "like": true,
}

var significantWordRe = regexp.MustCompile(`[a-zA-Z']+`)

// significantWords extracts the topic-bearing words from s: lowercased, stopwords
// and anything under 3 letters dropped.
func significantWords(s string) []string {
	var out []string
	for _, w := range significantWordRe.FindAllString(strings.ToLower(s), -1) {
		if len(w) < 3 || kbStopWords[w] {
			continue
		}
		out = append(out, w)
	}
	return out
}

// looksLikeAQuestionOrRequest reuses the same "is the customer actually asking for
// something" signal as isClassRelatedQuery, generalized beyond class topics — a
// question mark, or a common question/request word.
func looksLikeAQuestionOrRequest(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "?") || questionWords.MatchString(lower)
}

// kbCoversQuestion is a deterministic, provider-agnostic gate: does the text about to be
// shown to the model as "KNOWLEDGE BASE" have any topical overlap with what the customer
// asked? No LLM/embeddings call, so it holds even without a Gemini key. True (covered) is
// the permissive default — only a genuine question with zero matching topic words blocks.
func kbCoversQuestion(availableKnowledge, body string) bool {
	if !looksLikeAQuestionOrRequest(body) {
		return true
	}
	words := significantWords(body)
	if len(words) == 0 {
		return true
	}
	if strings.TrimSpace(availableKnowledge) == "" {
		return false
	}
	lower := strings.ToLower(availableKnowledge)
	for _, w := range words {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// parseMenuChoice reads a reply to our own "1. Book a Trial / 2. Become a Member"
// menu and reports which option (if any) it picked. Extracted so the trial/member
// distinction has one definition instead of being re-derived at each call site.
// isExplicitPurchaseIntent reports whether an already-lowercased message explicitly asks
// to buy/pay for a trial — but not when that's phrased as a question ("how do I pay?",
// "what's the payment link?" are asking, not confirming).
func isExplicitPurchaseIntent(lowerMsg string) bool {
	if isLikelyQuestion(lowerMsg) {
		return false
	}
	return strings.Contains(lowerMsg, "buy") ||
		strings.Contains(lowerMsg, "purchase") ||
		strings.Contains(lowerMsg, "how to pay") ||
		strings.Contains(lowerMsg, "how do i pay") ||
		strings.Contains(lowerMsg, "payment link") ||
		strings.Contains(lowerMsg, "pay for") ||
		strings.Contains(lowerMsg, "checkout")
}

func parseMenuChoice(body string) (wantsTrial, wantsMember bool) {
	choice := strings.ToLower(strings.TrimSpace(body))
	// A bare "trial"/"member" mention only counts as picking that option when the
	// reply isn't itself a question — "what does the trial include?" is asking,
	// not choosing.
	notQuestion := !isLikelyQuestion(choice)
	wantsTrial = choice == "1" || (notQuestion && (strings.Contains(choice, "trial") || strings.Contains(choice, "trail")))
	wantsMember = choice == "2" || (notQuestion && strings.Contains(choice, "member"))
	return
}

// lastOutboundSentOurMenu reports whether one of the last few outbound messages in
// this conversation was our own "1. Book a Trial / 2. Become a Member" menu — same
// "check up to the last 3 outbound messages, not just the very last one" approach
// the yes-to-trial shortcut already uses, since back-to-back bot replies (e.g. a
// staff member chiming in) can otherwise push the actual menu just out of view.
func lastOutboundSentOurMenu(history []Message) bool {
	checked := 0
	for i := len(history) - 1; i >= 0 && checked < 3; i-- {
		m := history[i]
		if m.Direction != DirectionOutbound {
			continue
		}
		checked++
		if strings.Contains(m.Body, "1. Book a Trial") {
			return true
		}
	}
	return false
}

// conversationResumedAfterGap reports whether the customer's latest message arrived more
// than gap after the one before it — i.e. this reads as resuming a paused conversation,
// not the next turn of an active back-and-forth. Used to gate re-greeting by name.
func conversationResumedAfterGap(history []Message, gap time.Duration) bool {
	if len(history) < 2 {
		return true // nothing to compare against — treat as a fresh start
	}
	last := history[len(history)-1]
	prev := history[len(history)-2]
	return last.SentAt.Sub(prev.SentAt) > gap
}
