package evals

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCase(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

func TestLoadCases_ParsesValidFixtures(t *testing.T) {
	dir := t.TempDir()
	writeCase(t, dir, "b.json", `{"id":"b","customer_message":"hi"}`)
	writeCase(t, dir, "a.json", `{"id":"a","customer_message":"hello"}`)

	cases, err := LoadCases(dir)
	if err != nil {
		t.Fatalf("LoadCases: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("len(cases) = %d, want 2", len(cases))
	}
	// Sorted by filename, so "a.json" before "b.json".
	if cases[0].ID != "a" || cases[1].ID != "b" {
		t.Errorf("cases not sorted by filename: got order %q, %q", cases[0].ID, cases[1].ID)
	}
}

func TestLoadCases_RejectsMalformedFixture(t *testing.T) {
	dir := t.TempDir()
	writeCase(t, dir, "good.json", `{"id":"good","customer_message":"hi"}`)
	writeCase(t, dir, "bad.json", `{not valid json`)

	_, err := LoadCases(dir)
	if err == nil {
		t.Fatal("LoadCases should fail loudly on a malformed fixture, not skip it silently")
	}
}

func TestLoadCases_RejectsMissingRequiredFields(t *testing.T) {
	dir := t.TempDir()
	writeCase(t, dir, "no-id.json", `{"customer_message":"hi"}`)

	_, err := LoadCases(dir)
	if err == nil {
		t.Fatal("LoadCases should reject a case missing \"id\"")
	}

	dir2 := t.TempDir()
	writeCase(t, dir2, "no-message.json", `{"id":"x"}`)
	_, err = LoadCases(dir2)
	if err == nil {
		t.Fatal("LoadCases should reject a case missing \"customer_message\"")
	}
}

func TestLoadCases_IgnoresNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	writeCase(t, dir, "case.json", `{"id":"x","customer_message":"hi"}`)
	writeCase(t, dir, "README.md", `not a case`)

	cases, err := LoadCases(dir)
	if err != nil {
		t.Fatalf("LoadCases: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("len(cases) = %d, want 1 (README.md should be ignored)", len(cases))
	}
}

// TestLoadCaseSetFromPath_RealCheckedInCases confirms the actual fixture
// files under internal/evals/cases/ (not synthetic temp-dir fixtures)
// parse cleanly — these are a starter set (not yet the doc's full
// 150-conversation target; see tasks.md task 1.2's completion note).
func TestLoadCaseSetFromPath_RealCheckedInCases(t *testing.T) {
	dev, heldout, err := LoadCaseSetFromPath("cases")
	if err != nil {
		t.Fatalf("LoadCaseSetFromPath(cases): %v", err)
	}
	if len(dev) == 0 {
		t.Error("expected at least one dev case in internal/evals/cases/dev")
	}
	if len(heldout) == 0 {
		t.Error("expected at least one heldout case in internal/evals/cases/heldout")
	}
}

func TestLoadCaseSetFromPath_LoadsBothSplits(t *testing.T) {
	root := t.TempDir()
	devDir := filepath.Join(root, "dev")
	heldoutDir := filepath.Join(root, "heldout")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(heldoutDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCase(t, devDir, "d1.json", `{"id":"d1","customer_message":"hi"}`)
	writeCase(t, heldoutDir, "h1.json", `{"id":"h1","customer_message":"hi"}`)

	dev, heldout, err := LoadCaseSetFromPath(root)
	if err != nil {
		t.Fatalf("LoadCaseSetFromPath: %v", err)
	}
	if len(dev) != 1 || dev[0].ID != "d1" {
		t.Errorf("dev = %+v, want one case with id=d1", dev)
	}
	if len(heldout) != 1 || heldout[0].ID != "h1" {
		t.Errorf("heldout = %+v, want one case with id=h1", heldout)
	}
}
