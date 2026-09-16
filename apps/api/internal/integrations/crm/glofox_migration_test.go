package crm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/integrations/glofox"
	"github.com/projectx/api/internal/platform/secrets"
)

// capturedRequest is everything about an inbound HTTP request this test
// cares about comparing between the old and new code paths.
type capturedRequest struct {
	method string
	path   string
	header http.Header
	body   map[string]any
}

func captureRequest(r *http.Request) capturedRequest {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	return capturedRequest{method: r.Method, path: r.URL.Path, header: r.Header.Clone(), body: body}
}

// TestGlofoxMigration_PurchaseMembership_Parity is Phase 4's regression
// test: it runs the identical scenario through the old, hand-written
// glofox.Client.PurchaseMembership and the new, data-driven
// crm.Executor.Execute(..., OpPurchaseMembership, ...) — seeded by
// migrations/20260912000004_seed_glofox_provider.sql — against the same
// httptest stub, and asserts they produce the same outbound request (method,
// path, the three Glofox auth headers, and body) and the same parsed
// invoice ID. This is what proves the migration is behavior-preserving
// before any real call site is ever pointed at the new executor.
func TestGlofoxMigration_PurchaseMembership_Parity(t *testing.T) {
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

	var glofoxProviderID uuid.UUID
	err = pool.QueryRow(ctx, `SELECT id FROM crm_providers WHERE name = 'Glofox'`).Scan(&glofoxProviderID)
	if err != nil {
		t.Skip("Skipping test; Glofox provider not seeded (run migrations/20260912000004_seed_glofox_provider.sql)")
	}

	const apiKey, apiToken, branchID = "test-api-key", "test-api-token", "branch123"
	const userID, membershipID, planCode = "user123", "membership456", "plan789"
	const startDate = int64(1700000000)

	var captured []capturedRequest
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = append(captured, captureRequest(r))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"ok","message_code":"OK","status":"PENDING-INTENT","invoice_id":"inv-abc-123"}`))
	}))
	defer stub.Close()

	// ---- old path: the hand-written glofox.Client ----
	oldClient := glofox.NewWithBaseURL(apiKey, apiToken, branchID, stub.URL)
	oldResp, err := oldClient.PurchaseMembership(ctx, glofox.PurchaseMembershipInput{
		UserID:        userID,
		MembershipID:  membershipID,
		PlanCode:      planCode,
		StartDateUnix: startDate,
	})
	if err != nil {
		t.Fatalf("old glofox.Client.PurchaseMembership: %v", err)
	}
	if len(captured) != 1 {
		t.Fatalf("old client made %d requests, want 1", len(captured))
	}
	oldReq := captured[0]
	captured = nil

	// ---- new path: the generic, data-driven executor ----
	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	repo := NewRepo(pool, cipher)

	// Point this test's connection at the stub by giving Glofox's seeded
	// provider row a temporary base_url override — restored on cleanup so
	// this test never leaves the real provider row pointed at a dead stub.
	var realBaseURL string
	if err := pool.QueryRow(ctx, `SELECT base_url FROM crm_providers WHERE id = $1`, glofoxProviderID).Scan(&realBaseURL); err != nil {
		t.Fatalf("read glofox provider base_url: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE crm_providers SET base_url = $2 WHERE id = $1`, glofoxProviderID, stub.URL); err != nil {
		t.Fatalf("point glofox provider at stub: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `UPDATE crm_providers SET base_url = $2 WHERE id = $1`, glofoxProviderID, realBaseURL)
	})

	conn, err := repo.CreateConnection(ctx, studioID, glofoxProviderID, map[string]string{
		"x-glofox-api-token": apiToken,
		"x-api-key":          apiKey,
		"x-glofox-branch-id": branchID,
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM crm_connections WHERE id = $1`, conn.ID) })

	executor := NewExecutor(repo)
	out, err := executor.Execute(ctx, studioID, OpPurchaseMembership, map[string]any{
		"userId":       userID,
		"membershipId": membershipID,
		"planCode":     planCode,
		"startDate":    startDate,
	})
	if err != nil {
		t.Fatalf("new crm.Executor.Execute: %v", err)
	}
	if len(captured) != 1 {
		t.Fatalf("new executor made %d requests, want 1", len(captured))
	}
	newReq := captured[0]

	// ---- compare ----
	if oldReq.method != newReq.method {
		t.Errorf("method: old=%q new=%q", oldReq.method, newReq.method)
	}
	if oldReq.path != newReq.path {
		t.Errorf("path: old=%q new=%q", oldReq.path, newReq.path)
	}
	for _, h := range []string{"X-Glofox-Api-Token", "X-Api-Key", "X-Glofox-Branch-Id"} {
		if oldReq.header.Get(h) != newReq.header.Get(h) {
			t.Errorf("header %s: old=%q new=%q", h, oldReq.header.Get(h), newReq.header.Get(h))
		}
	}
	if fmt.Sprint(oldReq.body["start_date"]) != fmt.Sprint(newReq.body["start_date"]) {
		t.Errorf("body.start_date: old=%v new=%v", oldReq.body["start_date"], newReq.body["start_date"])
	}

	if oldResp.InvoiceID != "inv-abc-123" {
		t.Errorf("old response invoice_id = %q, want inv-abc-123", oldResp.InvoiceID)
	}
	if out["invoiceId"] != "inv-abc-123" {
		t.Errorf("new response invoiceId = %v, want inv-abc-123", out["invoiceId"])
	}
	if oldResp.InvoiceID != out["invoiceId"] {
		t.Errorf("old and new parsed invoice IDs differ: old=%q new=%v", oldResp.InvoiceID, out["invoiceId"])
	}
}
