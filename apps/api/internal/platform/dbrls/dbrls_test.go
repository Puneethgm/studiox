package dbrls

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// testPool connects as the app's normal (superuser, in this local dev
// setup) role — used for seeding fixtures, where RLS shouldn't get in
// the way.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_DB"))
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("Skipping integration test; no DB env vars found")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to DB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// testNonSuperuserPool connects as a dedicated, deliberately-NOT-
// superuser/NOT-bypassrls Postgres role (rls_test_role, created once by
// hand against the local dev DB for this test suite) so the isolation
// tests below exercise RLS the way it actually behaves in production —
// a Postgres superuser bypasses RLS/FORCE ROW LEVEL SECURITY
// unconditionally, which this local dev DB's own app role (projectx)
// happens to be, so testing against THAT role would silently pass
// without RLS doing anything at all. See this package's own doc
// comment for why `FORCE ROW LEVEL SECURITY` alone isn't sufficient —
// it only closes the table-owner exemption, not the superuser one.
func testNonSuperuserPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	host, port, dbName := os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_DB")
	if port == "" {
		t.Skip("Skipping integration test; no DB env vars found")
	}
	dsn := fmt.Sprintf("postgres://rls_test_role:rls_test_pw@%s:%s/%s?sslmode=disable", host, port, dbName)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skip("Skipping: rls_test_role not available on this DB (create it with " +
			"`CREATE ROLE rls_test_role LOGIN PASSWORD 'rls_test_pw' NOSUPERUSER NOBYPASSRLS; " +
			"GRANT ALL ON ALL TABLES IN SCHEMA public TO rls_test_role;` to run this test for real)")
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// seedTwoStudiosWithLeads creates two real studios and one lead each
// (with a campaign, since leads.campaign_id is NOT NULL), returning
// their IDs. Cleanup removes everything it created.
func seedTwoStudiosWithLeads(t *testing.T, pool *pgxpool.Pool) (studioA, studioB, leadA, leadB uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	var adminA, adminB uuid.UUID
	for i, studioIDPtr := range []*uuid.UUID{&studioA, &studioB} {
		name := fmt.Sprintf("dbrls-test-studio-%d-%s", i, uuid.New().String()[:8])
		if err := pool.QueryRow(ctx, `
			INSERT INTO studios (slug, name, contact_email, active)
			VALUES ($1, $1, $2, true) RETURNING id
		`, name, name+"@example.com").Scan(studioIDPtr); err != nil {
			t.Fatalf("insert studio: %v", err)
		}
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (studio_id, email, password_hash, role) VALUES ($1,$2,'x','studio_admin') RETURNING id`,
		studioA, "dbrls-admin-a-"+uuid.New().String()[:8]+"@example.com").Scan(&adminA); err != nil {
		t.Fatalf("insert admin A: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (studio_id, email, password_hash, role) VALUES ($1,$2,'x','studio_admin') RETURNING id`,
		studioB, "dbrls-admin-b-"+uuid.New().String()[:8]+"@example.com").Scan(&adminB); err != nil {
		t.Fatalf("insert admin B: %v", err)
	}

	var campaignA, campaignB uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO campaigns (studio_id, slug, name, created_by) VALUES ($1,'dbrls-test','dbrls-test',$2) RETURNING id`,
		studioA, adminA).Scan(&campaignA); err != nil {
		t.Fatalf("insert campaign A: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO campaigns (studio_id, slug, name, created_by) VALUES ($1,'dbrls-test','dbrls-test',$2) RETURNING id`,
		studioB, adminB).Scan(&campaignB); err != nil {
		t.Fatalf("insert campaign B: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO leads (studio_id, campaign_id, name, email, phone, fitness_plan)
		VALUES ($1,$2,'A Lead','a@example.com','111','general') RETURNING id
	`, studioA, campaignA).Scan(&leadA); err != nil {
		t.Fatalf("insert lead A: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO leads (studio_id, campaign_id, name, email, phone, fitness_plan)
		VALUES ($1,$2,'B Lead','b@example.com','222','general') RETURNING id
	`, studioB, campaignB).Scan(&leadB); err != nil {
		t.Fatalf("insert lead B: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM leads WHERE id IN ($1,$2)`, leadA, leadB)
		_, _ = pool.Exec(ctx, `DELETE FROM campaigns WHERE id IN ($1,$2)`, campaignA, campaignB)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, adminA, adminB)
		_, _ = pool.Exec(ctx, `DELETE FROM studios WHERE id IN ($1,$2)`, studioA, studioB)
	})
	return studioA, studioB, leadA, leadB
}

// TestLeadsRLS_StudioScopedTxOnlySeesOwnStudio is the real, live two-
// studio isolation test for task 7.1/7.2 — it does not mock RLS, it
// runs against the actual local Postgres with the actual migration
// applied.
func TestLeadsRLS_StudioScopedTxOnlySeesOwnStudio(t *testing.T) {
	pool := testPool(t)
	studioA, _, leadA, leadB := seedTwoStudiosWithLeads(t, pool)
	ctx := context.Background()

	rlsPool := testNonSuperuserPool(t)
	txA, err := BeginScopedToStudio(ctx, rlsPool, studioA)
	if err != nil {
		t.Fatalf("BeginScopedToStudio(A): %v", err)
	}
	defer txA.Rollback(ctx)

	var seenIDs []uuid.UUID
	rows, err := txA.Query(ctx, `SELECT id FROM leads WHERE id IN ($1, $2)`, leadA, leadB)
	if err != nil {
		t.Fatalf("query within studio A's scope: %v", err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seenIDs = append(seenIDs, id)
	}
	rows.Close()

	if len(seenIDs) != 1 || seenIDs[0] != leadA {
		t.Fatalf("studio A's scoped tx should see only leadA (%s), saw %v", leadA, seenIDs)
	}
}

// TestLeadsRLS_SuperAdminBypassSeesBothStudios proves the explicit
// super_admin escape hatch works — a cross-studio query that would
// otherwise be isolated per-studio instead sees both rows.
func TestLeadsRLS_SuperAdminBypassSeesBothStudios(t *testing.T) {
	pool := testPool(t)
	_, _, leadA, leadB := seedTwoStudiosWithLeads(t, pool)
	ctx := context.Background()

	rlsPool := testNonSuperuserPool(t)
	tx, err := BeginBypassingRLS(ctx, rlsPool)
	if err != nil {
		t.Fatalf("BeginBypassingRLS: %v", err)
	}
	defer tx.Rollback(ctx)

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM leads WHERE id IN ($1, $2)`, leadA, leadB).Scan(&count); err != nil {
		t.Fatalf("query with bypass: %v", err)
	}
	if count != 2 {
		t.Fatalf("super_admin bypass should see both studios' leads, saw count=%d", count)
	}
}

// TestLeadsRLS_UnscopedQueryStillSeesEverything pins this migration's
// deliberate fail-open default (see the migration's own doc comment):
// a query that never sets app.current_studio_id at all — i.e. every
// existing call site in this codebase today — is completely unaffected
// by RLS being enabled.
func TestLeadsRLS_UnscopedQueryStillSeesEverything(t *testing.T) {
	pool := testPool(t)
	_, _, leadA, leadB := seedTwoStudiosWithLeads(t, pool)
	ctx := context.Background()

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM leads WHERE id IN ($1, $2)`, leadA, leadB).Scan(&count); err != nil {
		t.Fatalf("unscoped query: %v", err)
	}
	if count != 2 {
		t.Fatalf("an unscoped query (no session variable set) must still see both rows — fail-open guarantee broken, saw count=%d", count)
	}
}

// TestMessagesRLS_StudioScopedTxOnlySeesOwnStudio covers the second
// table from task 7.1 the same way.
func TestMessagesRLS_StudioScopedTxOnlySeesOwnStudio(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	var studioA, studioB uuid.UUID
	for _, p := range []*uuid.UUID{&studioA, &studioB} {
		name := "dbrls-msg-test-" + uuid.New().String()[:8]
		if err := pool.QueryRow(ctx, `INSERT INTO studios (slug, name, contact_email, active) VALUES ($1,$1,$2,true) RETURNING id`,
			name, name+"@example.com").Scan(p); err != nil {
			t.Fatalf("insert studio: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM studios WHERE id IN ($1,$2)`, studioA, studioB)
	})

	var channelA, channelB, convA, convB, msgA, msgB uuid.UUID
	for i, pair := range []struct {
		studio *uuid.UUID
		ch, cv, msg *uuid.UUID
	}{
		{&studioA, &channelA, &convA, &msgA},
		{&studioB, &channelB, &convB, &msgB},
	} {
		if err := pool.QueryRow(ctx, `
			INSERT INTO channel_accounts (studio_id, kind, external_id, display_handle, access_token_enc)
			VALUES ($1,'whatsapp_meta',$2,$2,'enc') RETURNING id
		`, *pair.studio, fmt.Sprintf("dbrls-ext-%d-%s", i, uuid.New().String()[:8])).Scan(pair.ch); err != nil {
			t.Fatalf("insert channel_account: %v", err)
		}
		var identityID uuid.UUID
		if err := pool.QueryRow(ctx, `
			INSERT INTO contact_identities (studio_id, kind, value, display_name)
			VALUES ($1,'phone',$2,'dbrls test contact') RETURNING id
		`, *pair.studio, fmt.Sprintf("+1-dbrls-%d-%s", i, uuid.New().String()[:8])).Scan(&identityID); err != nil {
			t.Fatalf("insert contact_identity: %v", err)
		}
		if err := pool.QueryRow(ctx, `
			INSERT INTO conversations (studio_id, channel_account_id, contact_identity_id, external_thread_id, status)
			VALUES ($1,$2,$3,$4,'open') RETURNING id
		`, *pair.studio, *pair.ch, identityID, fmt.Sprintf("thread-%d-%s", i, uuid.New().String()[:8])).Scan(pair.cv); err != nil {
			t.Fatalf("insert conversation: %v", err)
		}
		if err := pool.QueryRow(ctx, `
			INSERT INTO messages (conversation_id, studio_id, direction, body)
			VALUES ($1,$2,'inbound','dbrls test message') RETURNING id
		`, *pair.cv, *pair.studio).Scan(pair.msg); err != nil {
			t.Fatalf("insert message: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE id IN ($1,$2)`, msgA, msgB)
		_, _ = pool.Exec(ctx, `DELETE FROM conversations WHERE id IN ($1,$2)`, convA, convB)
		_, _ = pool.Exec(ctx, `DELETE FROM contact_identities WHERE studio_id IN ($1,$2)`, studioA, studioB)
		_, _ = pool.Exec(ctx, `DELETE FROM channel_accounts WHERE id IN ($1,$2)`, channelA, channelB)
	})

	rlsPool := testNonSuperuserPool(t)
	txA, err := BeginScopedToStudio(ctx, rlsPool, studioA)
	if err != nil {
		t.Fatalf("BeginScopedToStudio: %v", err)
	}
	defer txA.Rollback(ctx)

	var seen []uuid.UUID
	rows, err := txA.Query(ctx, `SELECT id FROM messages WHERE id IN ($1,$2)`, msgA, msgB)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen = append(seen, id)
	}
	rows.Close()

	if len(seen) != 1 || seen[0] != msgA {
		t.Fatalf("studio A's scoped tx should see only msgA (%s), saw %v", msgA, seen)
	}
}
