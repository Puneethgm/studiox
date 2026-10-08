package messaging

import (
	"strings"
	"testing"
	"time"

	"github.com/projectx/api/internal/leads"
)

func TestIsClassActionRequest(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		// real customer messages from prod that the AI mishandled
		{"I actually booked two classes today to finish up my package, but I’m afraid I won’t be able to make it for my 12pm class. I’m so sorry about that!", true},
		{"This friday’s 6:10pm reformx class by terence please", false}, // naming a class slot alone isn't an action verb; the surrounding conversation was already escalated
		// requests to book / change a class
		{"can I book the 6pm class on Friday?", true},
		{"Please cancel my session tomorrow", true},
		{"can you move my class to Saturday", true},
		{"I need to reschedule my slot", true},
		{"Could you extend my credits?", true},
		// trial booking has its own scripted flow, so it is not escalated here
		{"I'd like to book a trial class", false},
		{"how do I book a trail", false},
		// no class noun or no action verb
		{"what are your prices?", false},
		{"thanks for the class!", false},
		{"we booked a holiday for June", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isClassActionRequest(c.msg); got != c.want {
			t.Errorf("isClassActionRequest(%q) = %v, want %v", c.msg, got, c.want)
		}
	}
}

func TestIsClassRelatedQuery(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"what time is the Friday class?", true},
		{"any slots on Saturday?", true},
		{"do you offer reformer classes", true},
		{"Is there a session after 7pm", true},
		{"I want to know the timetable", true},
		{"This friday’s 6:10pm reformx class by terence please", true}, // a booking request with no action verb
		// thank-yous and passing mentions are not questions
		{"Thanks for the class!", false},
		{"thanks so much, loved the session", false},
		{"see you at class tomorrow", false},
		{"Thanks!!", false},
		// not about classes at all
		{"what are your prices?", false},
		{"where are you located?", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isClassRelatedQuery(c.msg); got != c.want {
			t.Errorf("isClassRelatedQuery(%q) = %v, want %v", c.msg, got, c.want)
		}
	}
}

func TestStripTimeGreeting(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Good morning Mandy! Your class is at 6pm.", "Mandy! Your class is at 6pm."},
		{"Good evening, let me check that for you.", "Let me check that for you."},
		{"good afternoon! we open at 9", "We open at 9"},
		{"  Good night\n\nSee you soon", "See you soon"},
		{"GOOD MORNING - it's on Friday", "It's on Friday"},
		// nothing to strip
		{"Hi Mandy, your class is at 6pm.", "Hi Mandy, your class is at 6pm."},
		{"The class is good morning-themed", "The class is good morning-themed"},
		{"", ""},
		// a reply that is only a greeting is left alone rather than emptied
		{"Good morning!", "Good morning!"},
	}
	for _, c := range cases {
		if got := stripTimeGreeting(c.in); got != c.want {
			t.Errorf("stripTimeGreeting(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Deterministic code-level backup for the same complaint TestConversationResumedAfterGap
// covers at the prompt level — buildPrompt tells the model not to re-greet by
// name mid-conversation, but models don't follow that reliably (same reason
// stripTimeGreeting exists), so this strips it from the actual output too.
// Deliberately NOT matched against the lead's stored name: the real bug
// report showed the model greeting "Hi Puneeth!" for a lead whose stored
// first_name was literally "puneethgm21" — the model invents a friendlier
// name on its own, so an exact-match-only first attempt at this fix missed
// the actual greeting entirely. This matches any capitalized-word greeting
// generically instead.
func TestStripNameGreeting(t *testing.T) {
	cases := []struct {
		resp, want string
	}{
		{"Hi Puneeth! Project 100 is our current 8-week program.", "Project 100 is our current 8-week program."},
		{"Hey Puneeth, sure thing!", "Sure thing!"},
		{"Hello Puneeth. Let me check that.", "Let me check that."},
		{"HI PUNEETH! here's the info.", "Here's the info."},
		// the exact real bug: stored first_name was "puneethgm21", but the
		// model greeted "Hi Puneeth!" — must still be caught generically.
		{"Hi Puneeth! The HIIT sessions in Project 100 are designed to boost your anaerobic capacity.", "The HIIT sessions in Project 100 are designed to boost your anaerobic capacity."},
		// a two-word name
		{"Hi John Smith! Sure, here's the schedule.", "Sure, here's the schedule."},
		// a different name in the text must not be stripped
		{"Hi Puneeth's trainer here to help.", "Hi Puneeth's trainer here to help."},
		// no greeting to strip
		{"Project 100 is our current 8-week program.", "Project 100 is our current 8-week program."},
		{"", ""},
		// a reply that is only a greeting is left alone rather than emptied
		{"Hi Puneeth!", "Hi Puneeth!"},
	}
	for _, c := range cases {
		if got := stripNameGreeting(c.resp); got != c.want {
			t.Errorf("stripNameGreeting(%q) = %q, want %q", c.resp, got, c.want)
		}
	}
}

func TestIsComplaintOrNegativeFeedback(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		// complaints, bad-review threats, refund demands
		{"I'm very disappointed with the class today", true},
		{"This is unacceptable", true},
		{"If this isn't fixed I'll leave a bad review", true},
		{"I'm going to post a Google review about this", true},
		{"I want a refund", true},
		{"The instructor was rude to me", true},
		{"Honestly I'm not happy with the service", true},
		{"It wasn't what I expected and I felt let down", true},
		{"The sessions could be better organised", true},
		{"That was a terrible experience", true},
		// politely worded or neutral/positive messages that must NOT escalate
		{"Great class, loved it!", false},
		{"Unfortunately I can't make it tomorrow, so sorry!", false},
		{"I'm afraid I won't be able to make it for my 12pm class", false},
		{"Thanks so much, appreciate it", false},
		{"what are your prices?", false},
		{"can I book the Friday class?", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isComplaintOrNegativeFeedback(c.msg); got != c.want {
			t.Errorf("isComplaintOrNegativeFeedback(%q) = %v, want %v", c.msg, got, c.want)
		}
	}
}

func TestLooksLikeAIDoesNotKnow(t *testing.T) {
	cases := []struct {
		resp string
		want bool
	}{
		// the exact real reply this was built to catch (Groq, no Gemini key, "Do u
		// provide shower option in the studio" — escalation never ran before this fix)
		{"Hi! I don't have details about our shower facilities on file, but a team member can share that info with you.\n\nDo you have a specific fitness goal in mind for the upcoming weeks?", true},
		{"I do not have that information on file, but I'll check with the team!", true},
		{"That's not something I have details on right now — let me find out for you.", true},
		{"I'm not sure about that, sorry!", true},
		// legitimate answers must NOT escalate
		{"Yes, we have showers and lockers at the Tanjong Pagar studio!", false},
		{"We're open Monday to Saturday, 6am to 9pm.", false},
		{"Great! I'd love to help you get started. What are your main fitness goals?", false},
		{"", false},
	}
	for _, c := range cases {
		if got := looksLikeAIDoesNotKnow(c.resp); got != c.want {
			t.Errorf("looksLikeAIDoesNotKnow(%q) = %v, want %v", c.resp, got, c.want)
		}
	}
}

func TestKbCoversQuestion(t *testing.T) {
	// Deliberately avoids "studio", "option" and "provide" — those appear in the shower
	// question below and would coincidentally "cover" it via plain word overlap, which
	// would mask the exact real-world gap (Puneeth.G.M's real KB mentions neither the
	// studio's facilities nor showers at all) this test is meant to catch.
	kb := "Open Monday to Saturday, 6am to 9pm. We offer Pilates, Yoga, and HIIT classes. Membership starts at S$89/month."

	cases := []struct {
		name string
		kb   string
		body string
		want bool
	}{
		// the real case this was built for: Groq-only studio, no Gemini, "shower" and
		// "studio" (as a facility word) nowhere in the knowledge base
		{"real failure case: shower question, topic not in KB", kb, "Do u provide shower option in the studio", false},
		{"topic word present in KB", kb, "What time are you open?", true},
		{"another present topic", kb, "Do you offer Pilates classes?", true},
		{"price is covered", kb, "How much is membership?", true},
		// empty knowledge base: a real question must never be free-answered
		{"empty KB entirely, real question", "", "What's your cancellation policy?", false},
		{"empty KB, small talk", "", "Thanks so much!", true},
		{"empty KB, no question mark or question word", "", "ok great", true},
		// small talk / statements pass through even when nothing matches
		{"greeting", kb, "Hi there!", true},
		{"thanks, no question mark", kb, "Thanks for the info", true},
		// a genuine new-topic question must block even with a populated KB
		{"KB populated but topic absent", kb, "Do you have a swimming pool?", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := kbCoversQuestion(c.kb, c.body); got != c.want {
				t.Errorf("kbCoversQuestion(%q) = %v, want %v", c.body, got, c.want)
			}
		})
	}
}

func TestSignificantWords(t *testing.T) {
	got := significantWords("Do u provide shower option in the studio")
	want := map[string]bool{"provide": true, "shower": true, "option": true, "studio": true}
	if len(got) != len(want) {
		t.Fatalf("significantWords = %v, want exactly %v", got, want)
	}
	for _, w := range got {
		if !want[w] {
			t.Errorf("unexpected significant word %q (stopwords like 'do'/'u'/'in'/'the' should be dropped)", w)
		}
	}
}

func TestAvailableKnowledgeText_MatchesWhatThePromptActuallyShows(t *testing.T) {
	// No chunks, no studio KB at all — nothing available.
	if got := availableKnowledgeText(nil, nil); got != "" {
		t.Errorf("nil studio, no chunks: got %q, want empty", got)
	}
	// Chunks present take priority over the raw studio text.
	chunks := []string{"chunk one", "chunk two"}
	if got := availableKnowledgeText(chunks, nil); got != "chunk one\n\nchunk two" {
		t.Errorf("got %q", got)
	}
}

func TestHistoryAlreadyAskedAboutGoals(t *testing.T) {
	cases := []struct {
		name    string
		history []Message
		want    bool
	}{
		{"empty history", nil, false},
		{
			"AI asked goals earlier — the exact real greeting from this bug report",
			[]Message{
				{Direction: DirectionOutbound, Body: "Hi there! Welcome to the studio. Are you looking to join one of our coach-led sessions or perhaps start with a free trial?\n\nWhat are your main fitness goals right now?"},
				{Direction: DirectionInbound, Body: "Do u have parking slot"},
			},
			true,
		},
		{
			"customer's own message mentioning goals doesn't count — only the AI's own asks matter",
			[]Message{
				{Direction: DirectionInbound, Body: "what are your fitness goals for this studio anyway"},
			},
			false,
		},
		{
			"AI reply with no goals question",
			[]Message{
				{Direction: DirectionOutbound, Body: "Hi! Yes, we have parking slots right next to our building."},
			},
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := historyAlreadyAskedAboutGoals(c.history); got != c.want {
				t.Errorf("historyAlreadyAskedAboutGoals(%+v) = %v, want %v", c.history, got, c.want)
			}
		})
	}
}

// Regression for the real bug: once the AI has already asked about fitness goals
// once in a conversation, it must not keep re-asking on every subsequent reply —
// even for intents other than booking_inquiry, which is the only intent the old
// code checked.
func TestStripMotivationQuestions_StillWorksForNonBookingReplies(t *testing.T) {
	resp := "Hi! Yes, we have parking slots right next to our building. What are your main fitness goals right now?"
	got := stripMotivationQuestions(resp)
	if strings.Contains(strings.ToLower(got), "fitness goals") {
		t.Errorf("stripMotivationQuestions did not remove the repeated goals question: %q", got)
	}
	if !strings.Contains(got, "parking slots") {
		t.Errorf("stripMotivationQuestions removed the actual answer, not just the goals question: %q", got)
	}
}

func TestParseMenuChoice(t *testing.T) {
	cases := []struct {
		body                  string
		wantTrial, wantMember bool
	}{
		{"1", true, false},
		{"2", false, true},
		{"I'd like the trial please", true, false},
		{"I want to become a member", false, true},
		{"trail", true, false}, // common WhatsApp typo
		{"something else entirely", false, false},
		{"", false, false},
		// Real bug, same class as detectOptionChoice: a question merely mentioning
		// "trial"/"member" must not be read as picking that menu option.
		{"What does the trial actually include?", false, false},
		{"How long is the trial session?", false, false},
		{"Do I need to become a member first?", false, false},
	}
	for _, c := range cases {
		gotTrial, gotMember := parseMenuChoice(c.body)
		if gotTrial != c.wantTrial || gotMember != c.wantMember {
			t.Errorf("parseMenuChoice(%q) = (%v, %v), want (%v, %v)", c.body, gotTrial, gotMember, c.wantTrial, c.wantMember)
		}
	}
}

func TestLastOutboundSentOurMenu(t *testing.T) {
	cases := []struct {
		name    string
		history []Message
		want    bool
	}{
		{"empty history", nil, false},
		{
			"the real menu from this bug report",
			[]Message{
				{Direction: DirectionOutbound, Body: "Great! Please select an option:\n1. Book a Trial\n2. Become a Member"},
			},
			true,
		},
		{
			"menu is 2 outbound messages back — still within the last-3 window",
			[]Message{
				{Direction: DirectionOutbound, Body: "Great! Please select an option:\n1. Book a Trial\n2. Become a Member"},
				{Direction: DirectionInbound, Body: "ok"},
				{Direction: DirectionOutbound, Body: "No worries, take your time!"},
			},
			true,
		},
		{"no menu sent", []Message{{Direction: DirectionOutbound, Body: "Hi there, how can I help?"}}, false},
		{"customer mentioning the menu text doesn't count", []Message{{Direction: DirectionInbound, Body: "1. Book a Trial sounds good"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := lastOutboundSentOurMenu(c.history); got != c.want {
				t.Errorf("lastOutboundSentOurMenu(%+v) = %v, want %v", c.history, got, c.want)
			}
		})
	}
}

// Real complaint: the AI opened two replies in a row, 4 minutes apart in an
// active conversation, each with "Hi Puneeth!" — reads as robotic/repetitive.
// It should only greet the lead by name again when genuinely resuming after
// a real pause (> 2 hours), not on every turn of a live back-and-forth.
func TestConversationResumedAfterGap(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name    string
		history []Message
		want    bool
	}{
		{"no history at all — nothing to compare, greeting is fine", nil, true},
		{"only one message ever — nothing to compare, greeting is fine", []Message{{SentAt: now}}, true},
		{
			"the real case: 4 minutes apart in an active exchange",
			[]Message{
				{Direction: DirectionOutbound, SentAt: now.Add(-4 * time.Minute)},
				{Direction: DirectionInbound, SentAt: now},
			},
			false,
		},
		{
			"exactly at the 2-hour boundary — not yet a real pause",
			[]Message{
				{Direction: DirectionOutbound, SentAt: now.Add(-2 * time.Hour)},
				{Direction: DirectionInbound, SentAt: now},
			},
			false,
		},
		{
			"genuinely resumed after 3 hours of silence",
			[]Message{
				{Direction: DirectionOutbound, SentAt: now.Add(-3 * time.Hour)},
				{Direction: DirectionInbound, SentAt: now},
			},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := conversationResumedAfterGap(c.history, 2*time.Hour); got != c.want {
				t.Errorf("conversationResumedAfterGap(...) = %v, want %v", got, c.want)
			}
		})
	}
}

// Real bug: a customer simply ASKING about the trial got misread as
// confirming it, because the bare "trial" keyword check had no guard for a
// genuine question — unlike the equivalent, already-correct logic in
// service.go's processInboundLeadAutomation. "How much is the trial? What's
// the price for the $68 package?" flipped a lead's status straight to
// trial_booked and scheduled a trial follow-up, from a pricing question.
func TestDetectOptionChoice(t *testing.T) {
	w := &AIWorker{}
	cases := []struct {
		name       string
		body       string
		status     leads.LeadStatus
		wantStatus leads.LeadStatus
		wantOK     bool
	}{
		{
			"the real bug: a pricing question mentioning 'trial' must not confirm a booking",
			"How much is the trial? What's the price for the $68 package?", leads.StatusNew,
			"", false,
		},
		{"bare '1' still selects trial", "1", leads.StatusNew, leads.StatusTrialBooked, true},
		{"short, non-question 'trial' still counts as a choice", "trial please", leads.StatusNew, leads.StatusTrialBooked, true},
		{"explicit phrase counts even as part of a longer message", "ok I'll book a trial then, thanks", leads.StatusNew, leads.StatusTrialBooked, true},
		// "membership" is an explicit-phrase match (unlike bare "member"), so
		// it still counts here even inside a question — only the bare "trial"
		// keyword is gated off by the question guard, leaving membership as
		// the sole match.
		{"a question containing the explicit phrase 'membership' still matches it", "what's the difference between a trial and membership anyway?", leads.StatusNew, leads.StatusMember, true},
		// trial_booked status: same class of bug for "ready"/"yes"/"no"/"later"
		{"a question containing 'ready' must not confirm membership", "is the trial payment link ready yet?", leads.StatusTrialBooked, "", false},
		{"short 'yes' still selects member", "yes", leads.StatusTrialBooked, leads.StatusMember, true},
		{"short 'no' still selects dropped", "no", leads.StatusTrialBooked, leads.StatusDropped, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotStatus, gotOK := w.detectOptionChoice(c.body, c.status)
			if gotStatus != c.wantStatus || gotOK != c.wantOK {
				t.Errorf("detectOptionChoice(%q, %q) = (%q, %v), want (%q, %v)", c.body, c.status, gotStatus, gotOK, c.wantStatus, c.wantOK)
			}
		})
	}
}

// Real bug, same class as detectOptionChoice/parseMenuChoice: several of the trigger
// phrases ("how to pay", "how do i pay", "payment link") are themselves question-shaped,
// so a customer just asking about payment got sent a real Stripe checkout link they never
// asked to receive.
func TestIsExplicitPurchaseIntent(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"i want to buy the trial", true},
		{"ok let's checkout", true},
		{"send me the payment link please", true},
		// the real bug: these are genuine questions, not confirmations
		{"how do i pay for this?", false},
		{"how do I pay, do you take Apple Pay?", false},
		{"what's the payment link once I sign up?", false},
		{"how to pay if I don't have a card?", false},
		{"not sure how to pay for this, help?", false},
		{"what are your prices?", false},
		{"", false},
	}
	for _, c := range cases {
		lower := strings.ToLower(strings.TrimSpace(c.msg))
		if got := isExplicitPurchaseIntent(lower); got != c.want {
			t.Errorf("isExplicitPurchaseIntent(%q) = %v, want %v", c.msg, got, c.want)
		}
	}
}
