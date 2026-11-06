package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// defaultJudgeMinScore is the quality bar a judge score must clear to
// count as a pass — doc §8.1 doesn't pin an exact number, so this is a
// conservative starting point meant to be tuned once real judge-score
// distributions exist.
const defaultJudgeMinScore = 0.6

// Report is the output of one eval run against a case set.
type Report struct {
	Grades     []Grade `json:"grades"`
	PassCount  int     `json:"passCount"`
	FailCount  int     `json:"failCount"`
	ErrorCount int     `json:"errorCount"`
}

// Run grades every case in cases using generate to produce a reply and
// judge (optional — nil skips judge grading) to score it, returning a
// Report. generate/judge errors are recorded per-case (Grade.Err), not
// returned — one bad case must not abort the whole run.
func Run(ctx context.Context, cases []Case, generate ReplyGenerator, judge Judge) Report {
	var report Report
	for _, c := range cases {
		grade := Grade{CaseID: c.ID}

		reply, err := generate(ctx, c)
		if err != nil {
			grade.Err = fmt.Errorf("generate reply: %w", err)
			report.Grades = append(report.Grades, grade)
			report.ErrorCount++
			continue
		}

		grade.CodeCheck = GradeCodeChecks(c, reply)

		if judge != nil {
			jr, err := judge.Score(ctx, c, reply)
			if err != nil {
				grade.Err = fmt.Errorf("judge score: %w", err)
				report.Grades = append(report.Grades, grade)
				report.ErrorCount++
				continue
			}
			grade.Judge = jr
			grade.JudgeRan = true
		}

		report.Grades = append(report.Grades, grade)
		if grade.Passed(defaultJudgeMinScore) {
			report.PassCount++
		} else {
			report.FailCount++
		}
	}
	return report
}

// WriteJSON writes report as indented JSON to path.
func WriteJSON(report Report, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write report file %s: %w", path, err)
	}
	return nil
}

// ReadJSON is WriteJSON's counterpart — used by the super-admin "last eval
// run" viewer (apps/web's /admin/eval-report page, backed by
// GET /api/v1/admin/eval-report) to read back whatever report
// `go run ./cmd/eval-replies` most recently wrote, without this package
// needing to know anything about HTTP. A missing file is a normal,
// expected case (no eval has been run yet / on this host) — callers
// should treat os.IsNotExist specially rather than surfacing it as a
// generic error.
func ReadJSON(path string) (Report, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is evalReportPath(): a fixed literal or operator-set EVAL_REPORT_PATH env var, never derived from any HTTP request
	if err != nil {
		return Report{}, err
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return Report{}, fmt.Errorf("unmarshal report file %s: %w", path, err)
	}
	return report, nil
}
