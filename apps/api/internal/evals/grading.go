package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Reply is what a reply-generating function (the pluggable "generate a
// reply" function design.md's cmd/eval-replies CLI calls per case)
// produces for one case.
type Reply struct {
	Text    string `json:"text"`
	Handoff bool   `json:"handoff"` // true when the pipeline decided to hand off to staff instead of sending a reply
}

// ReplyGenerator produces a Reply for one case — the pluggable function
// the eval CLI runs against. Swappable so a unit test can inject a fixed
// fake instead of driving the real reply pipeline.
type ReplyGenerator func(ctx context.Context, c Case) (Reply, error)

// CodeCheckResult is the deterministic grading pass's verdict.
type CodeCheckResult struct {
	Passed bool     `json:"passed"`
	Notes  []string `json:"notes"`
}

// GradeCodeChecks deterministically checks reply against case's Expected
// fields — doc §8.1's "automated checks" half of grading. Never calls an
// LLM; every check here is a plain string/boolean comparison.
func GradeCodeChecks(c Case, r Reply) CodeCheckResult {
	var notes []string
	passed := true

	if c.Expected.MustOfferHandoff {
		if !r.Handoff {
			passed = false
			notes = append(notes, "expected a handoff (no AI reply sent), but a reply was generated")
		}
		// Nothing further to check once handoff is correctly satisfied —
		// a handoff case has no reply text to grade.
		return CodeCheckResult{Passed: passed, Notes: notes}
	}
	if r.Handoff {
		passed = false
		notes = append(notes, "unexpected handoff — case expected a normal reply")
		return CodeCheckResult{Passed: passed, Notes: notes}
	}

	if c.Expected.MustMentionPrice != "" && !strings.Contains(r.Text, c.Expected.MustMentionPrice) {
		passed = false
		notes = append(notes, fmt.Sprintf("expected reply to mention price %q, it did not", c.Expected.MustMentionPrice))
	}
	for _, bad := range c.Expected.MustNotMentionPrices {
		if strings.Contains(r.Text, bad) {
			passed = false
			notes = append(notes, fmt.Sprintf("reply incorrectly mentions price %q", bad))
		}
	}

	return CodeCheckResult{Passed: passed, Notes: notes}
}

// JudgeResult is the model-judge grading pass's verdict — doc §8.1's
// "model-judge grading" half, a quality score a code check can't produce
// (tone, helpfulness, naturalness).
type JudgeResult struct {
	Score float64 `json:"score"` // 0.0-1.0
	Notes string  `json:"notes"`
}

// Judge scores a reply's quality for a case. The real implementation
// (see cmd/eval-replies) calls an LLM via llmgateway; tests use a fake.
type Judge interface {
	Score(ctx context.Context, c Case, r Reply) (JudgeResult, error)
}

// Grade is one case's full grading result (both passes).
type Grade struct {
	CaseID    string          `json:"caseId"`
	CodeCheck CodeCheckResult `json:"codeCheck"`
	Judge     JudgeResult     `json:"judge"`
	JudgeRan  bool            `json:"judgeRan"` // false if no Judge was configured for this run
	Err       error           `json:"-"`        // set if reply generation or judging itself failed; see MarshalJSON
}

// gradeJSON is Grade's JSON shape — Err (a Go error interface with no
// exported fields of its own) can't marshal to anything useful via the
// default encoder, so this renders it as a plain string instead,
// specifically for the super-admin eval-report viewer (apps/web's
// /admin/eval-report) to actually show the admin why a case errored,
// rather than silently losing the message.
type gradeJSON struct {
	CaseID    string          `json:"caseId"`
	CodeCheck CodeCheckResult `json:"codeCheck"`
	Judge     JudgeResult     `json:"judge"`
	JudgeRan  bool            `json:"judgeRan"`
	Err       string          `json:"err,omitempty"`
}

func (g Grade) MarshalJSON() ([]byte, error) {
	out := gradeJSON{CaseID: g.CaseID, CodeCheck: g.CodeCheck, Judge: g.Judge, JudgeRan: g.JudgeRan}
	if g.Err != nil {
		out.Err = g.Err.Error()
	}
	return json.Marshal(out)
}

// Passed reports whether this case's grade counts as an overall pass:
// the code check passed and, if a judge ran, its score clears a minimum
// bar. A case with Err set never passes.
func (g Grade) Passed(judgeMinScore float64) bool {
	if g.Err != nil {
		return false
	}
	if !g.CodeCheck.Passed {
		return false
	}
	if g.JudgeRan && g.Judge.Score < judgeMinScore {
		return false
	}
	return true
}
