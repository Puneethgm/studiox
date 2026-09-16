package crm

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/integrations/llm"
	"github.com/projectx/api/internal/platform/secrets"
)

// fakeLLM is a canned llm.Provider — ParseCRMDoc's real API call is never
// exercised here, only the request it constructs and the parsing/persisting
// of whatever comes back, which is exactly what makes this test fast and
// deterministic instead of depending on a real LLM.
type fakeLLM struct {
	reply llm.Reply
	err   error
}

func (f *fakeLLM) Analyze(ctx context.Context, systemPrompt, document string) (llm.Reply, error) {
	return f.reply, f.err
}

func TestParseCRMDoc_Integration(t *testing.T) {
	_ = godotenv.Load("../../../../../.env")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("POSTGRES_DB"),
	)
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("Skipping integration test; no DB env vars found")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to DB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	repo := NewRepo(pool, cipher)

	fake := &fakeLLM{reply: llm.Reply{Text: `{
		"baseUrl": "https://api.testcrm.example",
		"authType": "bearer",
		"authFieldDefs": [{"key":"token","label":"Token","secret":true}],
		"operations": {
			"get_member": {"httpMethod":"GET","pathTemplate":"/members/{id}","requestMapping":{"path":{"id":"userId"}},"responseMapping":{"userId":"_id"}},
			"create_lead": {"httpMethod":"POST","pathTemplate":"/leads","requestMapping":{"body":{"email":"email"}},"responseMapping":{}},
			"purchase_membership": null
		}
	}`}}

	p, ops, err := ParseCRMDoc(ctx, fake, repo, "Test CRM (parsed)", "irrelevant doc text for this fake")
	if err != nil {
		t.Fatalf("ParseCRMDoc: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM crm_providers WHERE id = $1`, p.ID) })

	if p.Status != ProviderDraft {
		t.Errorf("provider status = %q, want draft", p.Status)
	}
	if p.AuthType != AuthBearer {
		t.Errorf("provider auth type = %q, want bearer", p.AuthType)
	}
	if p.BaseURL != "https://api.testcrm.example" {
		t.Errorf("provider base url = %q", p.BaseURL)
	}

	// Only the two operations the fake reply actually described should be
	// saved — not all six, and definitely not the one explicitly null.
	if len(ops) != 2 {
		t.Fatalf("got %d operations, want 2 (create_lead, get_member)", len(ops))
	}
	seen := map[OperationKey]bool{}
	for _, op := range ops {
		seen[op.OperationKey] = true
		if op.Reviewed {
			t.Errorf("operation %q reviewed = true, want false until a super-admin approves it", op.OperationKey)
		}
	}
	if !seen[OpGetMember] || !seen[OpCreateLead] {
		t.Errorf("got operations %+v, want get_member and create_lead", seen)
	}
	if seen[OpPurchaseMembership] {
		t.Error("purchase_membership was explicitly null in the fake reply — it should not have been saved")
	}
}
