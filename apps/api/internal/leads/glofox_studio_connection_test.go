package leads

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/integrations/crm"
	"github.com/projectx/api/internal/integrations/glofox"
	"github.com/projectx/api/internal/platform/secrets"
)

func TestGlofoxClientForStudio_NilCRMRepoUsesFallback(t *testing.T) {
	fallback := glofox.New("key", "token", "branch")
	got := glofoxClientForStudio(context.Background(), nil, uuid.New(), fallback)
	if got != fallback {
		t.Fatalf("expected fallback client when crmRepo is nil, got a different client")
	}
}

func TestGlofoxClientForStudio_NoConnectionFallsBack(t *testing.T) {
	pool := testDB(t)
	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	crmRepo := crm.NewRepo(pool, cipher)

	fallback := glofox.New("key", "token", "branch")
	// A studio that has never connected anything.
	got := glofoxClientForStudio(context.Background(), crmRepo, uuid.New(), fallback)
	if got != fallback {
		t.Fatalf("expected fallback client when studio has no CRM connection, got a different client")
	}
}

func TestGlofoxClientForStudio_UsesStudiosOwnConnection(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	crmRepo := crm.NewRepo(pool, cipher)

	provider, err := crmRepo.GetProviderByName(ctx, "Glofox")
	if err != nil {
		t.Skipf("Glofox provider not seeded in test DB: %v", err)
	}

	studioID, _ := seedTestLead(t, ctx, pool, "glofox-conn-test@example.com")

	creds := map[string]string{
		"x-api-key":          "test-api-key",
		"x-glofox-api-token": "test-api-token",
		"x-glofox-branch-id": "test-branch-id",
	}
	if _, err := crmRepo.CreateConnection(ctx, studioID, provider.ID, creds, nil); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM crm_connections WHERE studio_id = $1 AND crm_provider_id = $2`, studioID, provider.ID)
	})

	fallback := glofox.New("fallback-key", "fallback-token", "fallback-branch")
	got := glofoxClientForStudio(ctx, crmRepo, studioID, fallback)
	if got == nil {
		t.Fatal("expected a non-nil client built from the studio's own connection")
	}
	if got == fallback {
		t.Fatal("expected the studio's own connection to be used instead of the fallback client")
	}
}
