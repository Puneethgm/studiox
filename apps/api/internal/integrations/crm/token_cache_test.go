package crm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
)

// TestExecutor_TokenExchange_Integration proves the Phase 3 login-then-bearer
// flow: a provider configured with auth_type=token_exchange logs in once,
// reuses the cached token for calls inside its lifetime, and logs in again
// once it expires — same DB-test shape as executor_test.go (real local
// Postgres, httptest stub, t.Skip if unconfigured).
func TestExecutor_TokenExchange_Integration(t *testing.T) {
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

	var studioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("Skipping test; no studio found in DB")
	}

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	repo := NewRepo(pool, cipher)

	loginCalls := 0
	var apiCalls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			loginCalls++
			if r.Header.Get("Api-Key") != "test-api-key" {
				t.Errorf("login request missing static Api-Key header, got %q", r.Header.Get("Api-Key"))
			}
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["Username"] != "const-user" || body["Password"] != "test-api-key" {
				t.Errorf("login body = %+v, want Username=const-user Password=test-api-key", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"AccessToken": fmt.Sprintf("token-%d", loginCalls),
				"ExpiresIn":   2, // seconds — short so the test can observe expiry without sleeping long
			})
		case "/members/u1":
			if r.Header.Get("Api-Key") != "test-api-key" {
				t.Errorf("api call missing static Api-Key header")
			}
			auth := r.Header.Get("Authorization")
			apiCalls = append(apiCalls, auth)
			_ = json.NewEncoder(w).Encode(map[string]any{"_id": "u1"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	provider := &Provider{
		Name:                  "Test Token CRM",
		BaseURL:               srv.URL,
		AuthType:              AuthTokenExchange,
		AuthFieldDefs:         []AuthFieldDef{{Key: "Api-Key", Label: "API Key", Secret: true}},
		TokenLoginPath:        "/login",
		TokenLoginMethod:      "POST",
		TokenLoginBodyMapping: map[string]string{"Username": "const:const-user", "Password": "Api-Key"},
		TokenResponsePath:     "AccessToken",
		TokenExpiryPath:       "ExpiresIn",
		Status:                ProviderDraft,
	}
	if err := repo.CreateProvider(ctx, provider); err != nil {
		t.Fatalf("create provider: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM crm_providers WHERE id = $1`, provider.ID) })

	op := &Operation{
		CRMProviderID:  provider.ID,
		OperationKey:   OpGetMember,
		HTTPMethod:     "GET",
		PathTemplate:   "/members/{userId}",
		RequestMapping: map[string]any{"path": map[string]any{"userId": "userId"}},
		Reviewed:       true,
	}
	if err := repo.CreateOperation(ctx, op); err != nil {
		t.Fatalf("create operation: %v", err)
	}

	// crm_connections cascades from crm_providers (ON DELETE CASCADE), so the
	// provider cleanup above already removes this connection too.
	if _, err := repo.CreateConnection(ctx, studioID, provider.ID, map[string]string{"Api-Key": "test-api-key"}); err != nil {
		t.Fatalf("create connection: %v", err)
	}

	executor := NewExecutor(repo)

	if _, err := executor.Execute(ctx, studioID, OpGetMember, map[string]any{"userId": "u1"}); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	if _, err := executor.Execute(ctx, studioID, OpGetMember, map[string]any{"userId": "u1"}); err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if loginCalls != 1 {
		t.Errorf("loginCalls = %d after two calls inside the token's lifetime, want 1 (cached)", loginCalls)
	}
	if len(apiCalls) != 2 || apiCalls[0] != apiCalls[1] {
		t.Errorf("expected both calls to reuse the same bearer token, got %+v", apiCalls)
	}

	time.Sleep(3 * time.Second) // past the 2-second ExpiresIn

	if _, err := executor.Execute(ctx, studioID, OpGetMember, map[string]any{"userId": "u1"}); err != nil {
		t.Fatalf("third Execute (after expiry): %v", err)
	}
	if loginCalls != 2 {
		t.Errorf("loginCalls = %d after the cached token expired, want 2 (refreshed)", loginCalls)
	}
	if apiCalls[2] == apiCalls[0] {
		t.Error("expected a fresh bearer token after expiry, got the same one")
	}
}
