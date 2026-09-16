package crm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
)

// TestExecutor_AuthTypes_Integration exercises all three v1 static auth
// types end to end against a real local Postgres + a stubbed CRM API,
// confirming the Executor applies each auth type correctly and that
// crm_connections.credentials_enc is genuinely encrypted at rest — same
// shape as internal/messaging/connect_telegram_test.go.
func TestExecutor_AuthTypes_Integration(t *testing.T) {
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
	exec := NewExecutor(repo)

	// Stub CRM: reflects whatever auth header it saw back in the response
	// body, plus a fixed member payload for the response-mapping assertion.
	var sawAuthHeader string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuthHeader = r.Header.Get("Authorization")
		if sawAuthHeader == "" {
			// api_key case: auth rides on named headers, not Authorization.
			sawAuthHeader = "x-api-key=" + r.Header.Get("x-api-key")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"_id":"m123","email":"lead@example.com"}`))
	}))
	defer stub.Close()

	cases := []struct {
		name        string
		authType    AuthType
		fieldDefs   []AuthFieldDef
		creds       map[string]string
		plaintext   string // must NOT appear in the raw encrypted column
		wantAuthHas string
	}{
		{
			name:        "bearer",
			authType:    AuthBearer,
			fieldDefs:   []AuthFieldDef{{Key: "token", Label: "Token", Secret: true}},
			creds:       map[string]string{"token": "secret-bearer-token"},
			plaintext:   "secret-bearer-token",
			wantAuthHas: "Bearer secret-bearer-token",
		},
		{
			name:        "api_key",
			authType:    AuthAPIKey,
			fieldDefs:   []AuthFieldDef{{Key: "x-api-key", Label: "API Key", Secret: true}},
			creds:       map[string]string{"x-api-key": "secret-api-key"},
			plaintext:   "secret-api-key",
			wantAuthHas: "x-api-key=secret-api-key",
		},
		{
			name:        "basic",
			authType:    AuthBasic,
			fieldDefs:   []AuthFieldDef{{Key: "username", Label: "Username"}, {Key: "password", Label: "Password", Secret: true}},
			creds:       map[string]string{"username": "admin", "password": "secret-pass"},
			plaintext:   "secret-pass",
			wantAuthHas: "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret-pass")),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Provider{
				Name:          "Test CRM (" + tc.name + ")",
				BaseURL:       stub.URL,
				AuthType:      tc.authType,
				AuthFieldDefs: tc.fieldDefs,
				Status:        ProviderActive,
			}
			if err := repo.CreateProvider(ctx, p); err != nil {
				t.Fatalf("create provider: %v", err)
			}
			t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM crm_providers WHERE id = $1`, p.ID) })

			op := &Operation{
				CRMProviderID: p.ID,
				OperationKey:  OpGetMember,
				HTTPMethod:    "GET",
				PathTemplate:  "/members/{id}",
				RequestMapping: map[string]any{
					"path": map[string]any{"id": "userId"},
				},
				ResponseMapping: map[string]any{
					"memberId": "_id",
					"email":    "email",
				},
				Reviewed: true,
			}
			if err := repo.CreateOperation(ctx, op); err != nil {
				t.Fatalf("create operation: %v", err)
			}

			conn, err := repo.CreateConnection(ctx, studioID, p.ID, tc.creds)
			if err != nil {
				t.Fatalf("create connection: %v", err)
			}

			out, err := exec.Execute(ctx, studioID, OpGetMember, map[string]any{"userId": "m123"})
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if out["memberId"] != "m123" || out["email"] != "lead@example.com" {
				t.Errorf("response mapping = %+v, want memberId=m123 email=lead@example.com", out)
			}
			if !strings.Contains(sawAuthHeader, tc.wantAuthHas) {
				t.Errorf("auth header = %q, want it to contain %q", sawAuthHeader, tc.wantAuthHas)
			}

			// Credentials must be encrypted at rest, not plaintext in the DB.
			var rawEnc string
			if err := pool.QueryRow(ctx, `SELECT credentials_enc FROM crm_connections WHERE id = $1`, conn.ID).Scan(&rawEnc); err != nil {
				t.Fatalf("query credentials_enc: %v", err)
			}
			if rawEnc == "" {
				t.Fatal("credentials_enc is empty")
			}
			if strings.Contains(rawEnc, tc.plaintext) {
				t.Fatalf("credentials stored in plaintext — credentials_enc must be encrypted, got %q", rawEnc)
			}
			var leaked map[string]string
			if err := json.Unmarshal([]byte(rawEnc), &leaked); err == nil {
				t.Fatal("credentials_enc decoded as plain JSON — it must be ciphertext, not the raw JSON blob")
			}
		})
	}
}
