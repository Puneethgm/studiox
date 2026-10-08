package studios

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// setupKnowledgeGapsTestEnv connects to the real dev DB (same pattern as the other
// integration tests in this codebase) and creates a throwaway studio row to attach
// knowledge_gaps fixtures to; the whole fixture is cleaned up via t.Cleanup.
func setupKnowledgeGapsTestEnv(t *testing.T) (*Repo, uuid.UUID) {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("skipping integration test; no DB env vars found")
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_DB"))
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	t.Cleanup(pool.Close)

	studioID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO studios (id, slug, name, brand_color) VALUES ($1, $2, 'Knowledge Gap Test', '#7c3aed')`,
		studioID, "kg-test-"+studioID.String()[:8])
	if err != nil {
		t.Fatalf("create test studio: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM studios WHERE id = $1`, studioID)
	})

	return NewRepo(pool, nil), studioID
}

// Two customers asking the same question (different case/whitespace) while it's
// still unanswered must collapse into one open row with times_asked incremented —
// the "Needs Answers" tab would otherwise fill up with duplicates of the same miss.
func TestCreateKnowledgeGap_DedupesRepeatAsksWhileOpen(t *testing.T) {
	repo, studioID := setupKnowledgeGapsTestEnv(t)
	ctx := context.Background()

	if err := repo.CreateKnowledgeGap(ctx, studioID, nil, nil, "What time is the Friday class?"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := repo.CreateKnowledgeGap(ctx, studioID, nil, nil, "  WHAT time is the friday class?  "); err != nil {
		t.Fatalf("second create: %v", err)
	}

	gaps, err := repo.ListKnowledgeGaps(ctx, studioID, "open")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(gaps) != 1 {
		t.Fatalf("got %d open gaps, want 1 (deduped)", len(gaps))
	}
	if gaps[0].TimesAsked != 2 {
		t.Errorf("times_asked = %d, want 2", gaps[0].TimesAsked)
	}
}

// Resolving a gap and then having the same question asked again must create a FRESH
// open gap — the dedupe index is scoped to status='open', so a resolved row must
// never silently swallow a brand-new instance of the same miss.
func TestCreateKnowledgeGap_ReopensAfterResolve(t *testing.T) {
	repo, studioID := setupKnowledgeGapsTestEnv(t)
	ctx := context.Background()

	if err := repo.CreateKnowledgeGap(ctx, studioID, nil, nil, "Do you offer Pilates?"); err != nil {
		t.Fatalf("create: %v", err)
	}
	gaps, _ := repo.ListKnowledgeGaps(ctx, studioID, "open")
	if len(gaps) != 1 {
		t.Fatalf("setup: got %d open gaps, want 1", len(gaps))
	}
	if err := repo.MarkKnowledgeGapResolved(ctx, studioID, gaps[0].ID, "Yes, every Tuesday at 6pm.", nil); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if err := repo.CreateKnowledgeGap(ctx, studioID, nil, nil, "Do you offer Pilates?"); err != nil {
		t.Fatalf("re-create: %v", err)
	}

	all, err := repo.ListKnowledgeGaps(ctx, studioID, "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d total gaps, want 2 (1 resolved + 1 fresh open)", len(all))
	}
	openCount, resolvedCount := 0, 0
	for _, g := range all {
		switch g.Status {
		case "open":
			openCount++
			if g.TimesAsked != 1 {
				t.Errorf("fresh gap times_asked = %d, want 1", g.TimesAsked)
			}
		case "resolved":
			resolvedCount++
			if g.Answer != "Yes, every Tuesday at 6pm." {
				t.Errorf("resolved gap answer = %q, want the recorded answer", g.Answer)
			}
		}
	}
	if openCount != 1 || resolvedCount != 1 {
		t.Errorf("open=%d resolved=%d, want 1 and 1", openCount, resolvedCount)
	}
}

// Dismissing or resolving a gap that's already handled (or doesn't exist) must fail
// with ErrNotFound rather than silently succeeding or double-processing it.
func TestKnowledgeGap_CannotResolveOrDismissTwice(t *testing.T) {
	repo, studioID := setupKnowledgeGapsTestEnv(t)
	ctx := context.Background()

	if err := repo.CreateKnowledgeGap(ctx, studioID, nil, nil, "What's the cancellation policy?"); err != nil {
		t.Fatalf("create: %v", err)
	}
	gaps, _ := repo.ListKnowledgeGaps(ctx, studioID, "open")
	id := gaps[0].ID

	if err := repo.MarkKnowledgeGapDismissed(ctx, studioID, id); err != nil {
		t.Fatalf("first dismiss: %v", err)
	}
	if err := repo.MarkKnowledgeGapDismissed(ctx, studioID, id); err != ErrNotFound {
		t.Errorf("second dismiss error = %v, want ErrNotFound", err)
	}
	if err := repo.MarkKnowledgeGapResolved(ctx, studioID, id, "answer", nil); err != ErrNotFound {
		t.Errorf("resolving an already-dismissed gap error = %v, want ErrNotFound", err)
	}
	if err := repo.MarkKnowledgeGapResolved(ctx, studioID, uuid.New(), "answer", nil); err != ErrNotFound {
		t.Errorf("resolving a nonexistent gap error = %v, want ErrNotFound", err)
	}
}

// A different studio's gap must never be visible, resolvable or dismissable through
// another studio's id — this is the tenant-isolation guarantee every studio-scoped
// table needs.
func TestKnowledgeGap_ScopedToStudio(t *testing.T) {
	repoA, studioA := setupKnowledgeGapsTestEnv(t)
	_, studioB := setupKnowledgeGapsTestEnv(t)
	ctx := context.Background()

	if err := repoA.CreateKnowledgeGap(ctx, studioA, nil, nil, "Is there parking?"); err != nil {
		t.Fatalf("create: %v", err)
	}
	gapsA, _ := repoA.ListKnowledgeGaps(ctx, studioA, "open")
	id := gapsA[0].ID

	gapsB, err := repoA.ListKnowledgeGaps(ctx, studioB, "open")
	if err != nil {
		t.Fatalf("list for studio B: %v", err)
	}
	if len(gapsB) != 0 {
		t.Errorf("studio B sees %d gaps belonging to studio A, want 0", len(gapsB))
	}
	if _, err := repoA.GetKnowledgeGap(ctx, studioB, id); err != ErrNotFound {
		t.Errorf("get studio A's gap scoped to studio B: err = %v, want ErrNotFound", err)
	}
	if err := repoA.MarkKnowledgeGapDismissed(ctx, studioB, id); err != ErrNotFound {
		t.Errorf("dismiss studio A's gap scoped to studio B: err = %v, want ErrNotFound", err)
	}
}

// Resolving a gap must append a "Q: ... / A: ..." block to the studio's single
// "Learned Answers" document rather than spawning a new file per answer, and the
// second resolve must append alongside the first, not overwrite it.
func TestResolveKnowledgeGap_AppendsToLearnedAnswersFile(t *testing.T) {
	repo, studioID := setupKnowledgeGapsTestEnv(t)
	svc := NewService(repo, nil, nil, nil, nil, nil, "")
	ctx := context.Background()

	if err := repo.CreateKnowledgeGap(ctx, studioID, nil, nil, "What's your cancellation policy?"); err != nil {
		t.Fatalf("create gap 1: %v", err)
	}
	if err := repo.CreateKnowledgeGap(ctx, studioID, nil, nil, "Do you have parking?"); err != nil {
		t.Fatalf("create gap 2: %v", err)
	}
	gaps, _ := repo.ListKnowledgeGaps(ctx, studioID, "open")
	if len(gaps) != 2 {
		t.Fatalf("setup: got %d open gaps, want 2", len(gaps))
	}
	var policyGap, parkingGap KnowledgeGap
	for _, g := range gaps {
		if g.Question == "What's your cancellation policy?" {
			policyGap = g
		} else {
			parkingGap = g
		}
	}

	if _, err := svc.ResolveKnowledgeGap(ctx, studioID, policyGap.ID, "24 hours notice, no fee.", nil); err != nil {
		t.Fatalf("resolve 1: %v", err)
	}
	if _, err := svc.ResolveKnowledgeGap(ctx, studioID, parkingGap.ID, "Yes, free parking at the back.", nil); err != nil {
		t.Fatalf("resolve 2: %v", err)
	}

	studio, err := repo.GetByID(ctx, studioID)
	if err != nil {
		t.Fatalf("get studio: %v", err)
	}
	var learnedFiles int
	var text string
	for _, f := range studio.KnowledgeBaseFiles {
		if f.Name == learnedAnswersFileName {
			learnedFiles++
			text = f.Text
		}
	}
	if learnedFiles != 1 {
		t.Fatalf("got %d 'Learned Answers' files, want exactly 1 (answers should accumulate in one doc)", learnedFiles)
	}
	if !strings.Contains(text, "Q: What's your cancellation policy?") || !strings.Contains(text, "A: 24 hours notice, no fee.") {
		t.Errorf("learned-answers text missing the cancellation Q/A:\n%s", text)
	}
	if !strings.Contains(text, "Q: Do you have parking?") || !strings.Contains(text, "A: Yes, free parking at the back.") {
		t.Errorf("learned-answers text missing the parking Q/A (second resolve overwrote the first?):\n%s", text)
	}

	// Both gaps must now read resolved, with their answer recorded.
	resolved, err := repo.ListKnowledgeGaps(ctx, studioID, "resolved")
	if err != nil {
		t.Fatalf("list resolved: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("got %d resolved gaps, want 2", len(resolved))
	}

	// Resolving the same gap again (already resolved) must fail cleanly, not
	// silently re-append a third block or panic on a nil answer lookup.
	if _, err := svc.ResolveKnowledgeGap(ctx, studioID, policyGap.ID, "something else", nil); err != ErrNotFound {
		t.Errorf("re-resolving an already-resolved gap: err = %v, want ErrNotFound", err)
	}

	// An empty answer must be rejected before anything is touched.
	if err := repo.CreateKnowledgeGap(ctx, studioID, nil, nil, "Is there a trial?"); err != nil {
		t.Fatalf("create gap 3: %v", err)
	}
	open, _ := repo.ListKnowledgeGaps(ctx, studioID, "open")
	if len(open) != 1 {
		t.Fatalf("setup: got %d open gaps, want 1", len(open))
	}
	if _, err := svc.ResolveKnowledgeGap(ctx, studioID, open[0].ID, "   ", nil); err != ErrValidation {
		t.Errorf("resolving with a blank answer: err = %v, want ErrValidation", err)
	}
	stillOpen, _ := repo.ListKnowledgeGaps(ctx, studioID, "open")
	if len(stillOpen) != 1 {
		t.Errorf("blank-answer resolve attempt changed the gap's status; got %d still open, want 1", len(stillOpen))
	}
}
