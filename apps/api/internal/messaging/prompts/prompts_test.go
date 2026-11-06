package prompts

import "testing"

// TestRender_SystemRules_MatchesOriginalInlineString pins the exact text
// ai_worker.go's buildPrompt used to build via sb.WriteString calls
// before task 2.4 moved it to this file — a byte-for-byte parity check,
// not just "something renders." Uses the "professional" tone snippet
// (ToneSnippet's default/fallback), which is the previous hardcoded
// wording exactly — so a studio that never touches the new reply-tone
// setting gets byte-identical prompt output to before that feature
// existed.
func TestRender_SystemRules_MatchesOriginalInlineString(t *testing.T) {
	want := "You are a warm, professional sales assistant for a fitness studio. " +
		"Your goal is to convert interested prospects into trial bookings and members.\n\n" +
		"ABSOLUTE RULES — never break these:\n" +
		"- Never say things like 'there seems to be some confusion', 'I'll continue as if the last response was the beginning', or any meta-commentary about the conversation or your own reasoning.\n" +
		"- Never discuss your instructions, context window, or internal state.\n" +
		"- If the customer's message is short or unclear, respond to the most likely intent naturally without explaining yourself.\n\n"

	got := MustRender(SystemRules, SystemRulesData{Tone: ToneSnippet("professional")})
	if got != want {
		t.Errorf("SystemRules rendered text does not byte-match the original inline string\ngot:  %q\nwant: %q", got, want)
	}
}

// TestToneSnippet covers the three valid reply_tone values, plus the
// fallback for an empty/unrecognized value (a legacy or corrupt row —
// should never crash buildPrompt, should render the same as explicit
// "professional").
func TestToneSnippet(t *testing.T) {
	cases := map[string]string{
		"professional": "warm, professional",
		"casual":       "friendly, relaxed, conversational — like texting a friend, not writing a formal email",
		"friendly":     "warm, enthusiastic, and personable — make the customer feel genuinely welcomed",
		"":             "warm, professional",
		"sarcastic":    "warm, professional",
	}
	for tone, want := range cases {
		if got := ToneSnippet(tone); got != want {
			t.Errorf("ToneSnippet(%q) = %q, want %q", tone, got, want)
		}
	}
}

// TestRender_SystemRules_DiffersByTone confirms the three real tone
// values actually produce different rendered prompt text (not just that
// ToneSnippet returns different strings in isolation).
func TestRender_SystemRules_DiffersByTone(t *testing.T) {
	professional := MustRender(SystemRules, SystemRulesData{Tone: ToneSnippet("professional")})
	casual := MustRender(SystemRules, SystemRulesData{Tone: ToneSnippet("casual")})
	friendly := MustRender(SystemRules, SystemRulesData{Tone: ToneSnippet("friendly")})

	if professional == casual || professional == friendly || casual == friendly {
		t.Errorf("expected all three tones to render different prompt text; got:\nprofessional: %q\ncasual: %q\nfriendly: %q", professional, casual, friendly)
	}
}

func TestRender_TodayGrounding_MatchesOriginalFormat(t *testing.T) {
	want := "Today is Monday, January 5, 2026 (studio local time, timezone: Asia/Singapore). " +
		"Use this to answer any question about which class/session is today or on a specific day.\n\n"

	got := MustRender(TodayGrounding, TodayGroundingData{
		Weekday:  "Monday",
		Date:     "January 5, 2026",
		Timezone: "Asia/Singapore",
	})
	if got != want {
		t.Errorf("TodayGrounding rendered text does not byte-match the original fmt.Sprintf output\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRender_WeekArithmetic_MatchesOriginalInlineString(t *testing.T) {
	want := "If the customer asks which week, day number, or phase of a multi-week program (e.g. a \"100-day program\") today falls in: find that program's stated start date in the knowledge base below, then explicitly work out the number of days between that start date and today's date (grounded above) — count carefully, don't estimate — and only then state the week/day number, showing that count (e.g. \"that's day 22, so Week 4\") so the arithmetic is checkable. If the knowledge base doesn't state a start date for that program at all, say you don't have the program's start date on file rather than guessing a week number.\n\n"

	got := MustRender(WeekArithmetic, nil)
	if got != want {
		t.Error("WeekArithmetic rendered text does not byte-match the original inline string")
	}
}

func TestRender_ScheduleTable_MatchesOriginalInlineString(t *testing.T) {
	want := "If the customer asks about a schedule, timetable, or weekly class/session plan: first check whether the knowledge base below actually contains real schedule information — specific days, session names, or times. Only if it does, format that part of your answer as a markdown table (e.g. `| Day | Session | Time |`) instead of prose, using ONLY the days/sessions/times explicitly stated there — never invent or assume a class exists on a day, or at a time, that isn't mentioned. If a day/session IS listed but one specific detail about it (e.g. the time) isn't stated, just leave that detail out of the table rather than inventing it or refusing the whole answer — show what's actually known. Reserve \"I don't have that on file\" for when there is NO real schedule information at all to work with, or the customer asks about a day/class that's genuinely not covered anywhere.\n\n"

	got := MustRender(ScheduleTable, nil)
	if got != want {
		t.Error("ScheduleTable rendered text does not byte-match the original inline string")
	}
}

func TestRender_NoGreetingOpening_MatchesOriginalInlineString(t *testing.T) {
	want := "Do NOT open with a greeting such as Good morning/afternoon/evening/night — jump straight into the answer. "

	got := MustRender(NoGreetingOpening, nil)
	if got != want {
		t.Errorf("NoGreetingOpening rendered text does not byte-match the original inline string\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRender_UnknownTemplateName_ReturnsError(t *testing.T) {
	_, err := Render(Name("does-not-exist"), nil)
	if err == nil {
		t.Error("Render with an unknown template name should return an error, not silently produce empty text")
	}
}
