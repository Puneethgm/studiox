// Package prompts holds the AI reply prompt's text as versioned .tmpl
// files (ai-reply-flow-llm-design doc's explicit ask — a prompt-wording
// change should be a reviewable diff to a template file, not buried in a
// Go string literal).
//
// Scope note: this extracts the STATIC instructional segments of
// ai_worker.go's buildPrompt — the ones a human actually reviews when
// "the prompt wording" changes — into template files. buildPrompt's
// surrounding control flow (date arithmetic, conditional branches,
// mid-build database lookups like GetProgramSession) stays Go code and
// feeds computed values into these templates where needed (e.g.
// today_grounding.tmpl's {{.Weekday}}/{{.Date}}/{{.Timezone}}); that
// control flow isn't prose someone reviews as a "prompt change," so
// templating it wouldn't serve this task's actual goal, and would be a
// much larger, far riskier rewrite of a live production prompt for very
// little of the doc's stated benefit. See
// openspec/changes/ai-reply-flow-llm-design/tasks.md task 2.4 for the
// explicit scope note on what was and wasn't converted.
package prompts

import (
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed *.tmpl
var files embed.FS

var templates = template.Must(template.ParseFS(files, "*.tmpl"))

// Name identifies one prompt template file (without its .tmpl extension).
type Name string

const (
	SystemRules       Name = "system_rules"
	TodayGrounding    Name = "today_grounding"
	WeekArithmetic    Name = "week_arithmetic"
	ScheduleTable     Name = "schedule_table"
	NoGreetingOpening Name = "no_greeting_opening"
	// Repair is the one-shot repair-call prompt (guards' bounded
	// repair-then-handoff, ai-reply-flow-llm-design task 4.4) — appended
	// to the original prompt, not used alone.
	Repair Name = "repair"
)

// Render executes the named template with data (nil for the templates
// that take no variables) and returns the rendered text.
func Render(name Name, data any) (string, error) {
	var sb strings.Builder
	if err := templates.ExecuteTemplate(&sb, string(name)+".tmpl", data); err != nil {
		return "", fmt.Errorf("render prompt template %q: %w", name, err)
	}
	return sb.String(), nil
}

// MustRender is Render, panicking on error — for the call sites in
// buildPrompt where a template execution failure means a template file
// itself is broken (a programmer error caught by the unit tests in this
// package, not a runtime condition the AI worker should try to recover
// from mid-reply).
func MustRender(name Name, data any) string {
	s, err := Render(name, data)
	if err != nil {
		panic(err)
	}
	return s
}

// replyToneSnippets maps a studio's reply_tone column value to the
// adjective phrase fed into system_rules.tmpl's "You are a {{.Tone}}
// sales assistant..." opening line. "professional" is today's exact
// previous hardcoded wording, so a studio that never touches this
// setting (the DEFAULT 'professional' from migrations/
// 20261102010001_reply_tone.sql) gets byte-identical prompt behavior to
// before this feature existed.
var replyToneSnippets = map[string]string{
	"professional": "warm, professional",
	"casual":       "friendly, relaxed, conversational — like texting a friend, not writing a formal email",
	"friendly":     "warm, enthusiastic, and personable — make the customer feel genuinely welcomed",
}

// SystemRulesData is SystemRules' template data.
type SystemRulesData struct {
	Tone string
}

// ToneSnippet resolves a studio's reply_tone value to its prompt
// snippet, falling back to the "professional" wording for an empty or
// unrecognized value (defense in depth alongside the DB CHECK
// constraint and the HTTP handler's validation — buildPrompt should
// never panic or render a blank tone just because of a bad/legacy row).
func ToneSnippet(tone string) string {
	if s, ok := replyToneSnippets[tone]; ok {
		return s
	}
	return replyToneSnippets["professional"]
}

// TodayGroundingData is TodayGrounding's template data.
type TodayGroundingData struct {
	Weekday  string
	Date     string
	Timezone string
}

// RepairData is Repair's template data.
type RepairData struct {
	PreviousReply string
	Violations    []string
}
