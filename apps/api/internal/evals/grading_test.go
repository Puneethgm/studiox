package evals

import (
	"context"
	"errors"
	"testing"
)

func TestGradeCodeChecks(t *testing.T) {
	cases := []struct {
		name   string
		c      Case
		r      Reply
		passed bool
	}{
		{
			name:   "must mention price - present",
			c:      Case{Expected: Expected{MustMentionPrice: "$99"}},
			r:      Reply{Text: "The monthly plan is $99."},
			passed: true,
		},
		{
			name:   "must mention price - absent",
			c:      Case{Expected: Expected{MustMentionPrice: "$99"}},
			r:      Reply{Text: "The monthly plan is reasonably priced."},
			passed: false,
		},
		{
			name:   "must not mention price - violated",
			c:      Case{Expected: Expected{MustNotMentionPrices: []string{"$49"}}},
			r:      Reply{Text: "You can get the $49 deal!"},
			passed: false,
		},
		{
			name:   "must not mention price - clean",
			c:      Case{Expected: Expected{MustNotMentionPrices: []string{"$49"}}},
			r:      Reply{Text: "The monthly plan is $99."},
			passed: true,
		},
		{
			name:   "must offer handoff - satisfied",
			c:      Case{Expected: Expected{MustOfferHandoff: true}},
			r:      Reply{Handoff: true},
			passed: true,
		},
		{
			name:   "must offer handoff - violated (reply sent instead)",
			c:      Case{Expected: Expected{MustOfferHandoff: true}},
			r:      Reply{Text: "Sure, I can help with that!"},
			passed: false,
		},
		{
			name:   "unexpected handoff",
			c:      Case{Expected: Expected{}},
			r:      Reply{Handoff: true},
			passed: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := GradeCodeChecks(tc.c, tc.r)
			if got.Passed != tc.passed {
				t.Errorf("GradeCodeChecks() passed = %v, want %v (notes: %v)", got.Passed, tc.passed, got.Notes)
			}
		})
	}
}

// fakeJudge is a deterministic Judge for tests — never calls an LLM.
type fakeJudge struct {
	score float64
	err   error
}

func (f fakeJudge) Score(ctx context.Context, c Case, r Reply) (JudgeResult, error) {
	if f.err != nil {
		return JudgeResult{}, f.err
	}
	return JudgeResult{Score: f.score, Notes: "fake"}, nil
}

func TestGrade_Passed(t *testing.T) {
	cases := []struct {
		name  string
		grade Grade
		want  bool
	}{
		{
			name:  "code check fails -> overall fail",
			grade: Grade{CodeCheck: CodeCheckResult{Passed: false}},
			want:  false,
		},
		{
			name:  "code check passes, no judge ran -> overall pass",
			grade: Grade{CodeCheck: CodeCheckResult{Passed: true}, JudgeRan: false},
			want:  true,
		},
		{
			name:  "code check passes, judge score above bar -> pass",
			grade: Grade{CodeCheck: CodeCheckResult{Passed: true}, JudgeRan: true, Judge: JudgeResult{Score: 0.9}},
			want:  true,
		},
		{
			name:  "code check passes, judge score below bar -> fail",
			grade: Grade{CodeCheck: CodeCheckResult{Passed: true}, JudgeRan: true, Judge: JudgeResult{Score: 0.1}},
			want:  false,
		},
		{
			name:  "generation error -> always fail",
			grade: Grade{CodeCheck: CodeCheckResult{Passed: true}, Err: errors.New("boom")},
			want:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.grade.Passed(defaultJudgeMinScore); got != tc.want {
				t.Errorf("Passed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRun_GradesEveryCaseAndCountsCorrectly(t *testing.T) {
	cases := []Case{
		{ID: "good", Expected: Expected{MustMentionPrice: "$99"}},
		{ID: "bad", Expected: Expected{MustMentionPrice: "$99"}},
		{ID: "errors-out", Expected: Expected{}},
	}

	generate := func(ctx context.Context, c Case) (Reply, error) {
		switch c.ID {
		case "good":
			return Reply{Text: "It's $99/month."}, nil
		case "bad":
			return Reply{Text: "It's affordable!"}, nil
		default:
			return Reply{}, errors.New("generation failed")
		}
	}

	report := Run(context.Background(), cases, generate, fakeJudge{score: 1.0})

	if report.PassCount != 1 {
		t.Errorf("PassCount = %d, want 1", report.PassCount)
	}
	if report.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", report.FailCount)
	}
	if report.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", report.ErrorCount)
	}
	if len(report.Grades) != 3 {
		t.Errorf("len(Grades) = %d, want 3", len(report.Grades))
	}
}

func TestRun_NilJudgeSkipsJudgeGrading(t *testing.T) {
	cases := []Case{{ID: "x", Expected: Expected{}}}
	generate := func(ctx context.Context, c Case) (Reply, error) {
		return Reply{Text: "ok"}, nil
	}

	report := Run(context.Background(), cases, generate, nil)
	if report.PassCount != 1 {
		t.Errorf("PassCount = %d, want 1 (nil judge should not block a pass)", report.PassCount)
	}
	if report.Grades[0].JudgeRan {
		t.Error("JudgeRan should be false when judge is nil")
	}
}
