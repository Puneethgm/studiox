package dbrls

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// TestCampaignsRLS_StudioScopedTxOnlySeesOwnStudio exercises task 9.1's
// extension of the RLS pattern to the remaining studio-scoped tables,
// against one representative table (campaigns — already has real
// fixture-creation logic in seedTwoStudiosWithLeads) rather than
// duplicating near-identical setup for all ~39 tables the migration
// touches. See migrations/20261030140001_rls_remaining_tables.sql for
// the full table list; this is the "doesn't regress 7.2/7.3 at the
// extended scale" check task 9.4 asks for, applied to a second real
// table beyond leads/messages.
func TestCampaignsRLS_StudioScopedTxOnlySeesOwnStudio(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	var studioA, studioB, adminA, adminB, campaignA, campaignB uuid.UUID
	for i, p := range []*uuid.UUID{&studioA, &studioB} {
		name := fmt.Sprintf("dbrls-campaign-test-%d-%s", i, uuid.New().String()[:8])
		if err := pool.QueryRow(ctx, `INSERT INTO studios (slug, name, contact_email, active) VALUES ($1,$1,$2,true) RETURNING id`,
			name, name+"@example.com").Scan(p); err != nil {
			t.Fatalf("insert studio: %v", err)
		}
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (studio_id, email, password_hash, role) VALUES ($1,$2,'x','studio_admin') RETURNING id`,
		studioA, "dbrls-camp-admin-a-"+uuid.New().String()[:8]+"@example.com").Scan(&adminA); err != nil {
		t.Fatalf("insert admin A: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (studio_id, email, password_hash, role) VALUES ($1,$2,'x','studio_admin') RETURNING id`,
		studioB, "dbrls-camp-admin-b-"+uuid.New().String()[:8]+"@example.com").Scan(&adminB); err != nil {
		t.Fatalf("insert admin B: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO campaigns (studio_id, slug, name, created_by) VALUES ($1,'dbrls-test-9-1','dbrls-test-9-1',$2) RETURNING id`,
		studioA, adminA).Scan(&campaignA); err != nil {
		t.Fatalf("insert campaign A: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO campaigns (studio_id, slug, name, created_by) VALUES ($1,'dbrls-test-9-1','dbrls-test-9-1',$2) RETURNING id`,
		studioB, adminB).Scan(&campaignB); err != nil {
		t.Fatalf("insert campaign B: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM campaigns WHERE id IN ($1,$2)`, campaignA, campaignB)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, adminA, adminB)
		_, _ = pool.Exec(ctx, `DELETE FROM studios WHERE id IN ($1,$2)`, studioA, studioB)
	})

	rlsPool := testNonSuperuserPool(t)
	txA, err := BeginScopedToStudio(ctx, rlsPool, studioA)
	if err != nil {
		t.Fatalf("BeginScopedToStudio: %v", err)
	}
	defer txA.Rollback(ctx)

	var seen []uuid.UUID
	rows, err := txA.Query(ctx, `SELECT id FROM campaigns WHERE id IN ($1,$2)`, campaignA, campaignB)
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

	if len(seen) != 1 || seen[0] != campaignA {
		t.Fatalf("studio A's scoped tx should see only campaignA (%s), saw %v — RLS on 'campaigns' (part of task 9.1's extended table set) is not isolating correctly", campaignA, seen)
	}
}
