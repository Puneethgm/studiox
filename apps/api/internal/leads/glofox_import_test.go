package leads

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGlofoxStatusToLeadStatus(t *testing.T) {
	cases := []struct {
		raw    string
		want   LeadStatus
		wantOK bool
	}{
		{"Active", StatusMember, true},
		{" active ", StatusMember, true},
		{"Paid", StatusMember, true},
		{"Trial", StatusTrialBooked, true},
		{"Pending", StatusTrialBooked, true},
		{"Inactive", StatusDropped, true},
		{"Cancelled", StatusDropped, true},
		{"Expired", StatusDropped, true},
		{"Frozen", StatusDropped, true},
		// platform's own statuses are NOT re-translated here — the caller checks
		// LeadStatus.Valid() first and only falls back to this for the rest
		{"new", "", false},
		{"", "", false},
		{"something unrecognized", "", false},
	}
	for _, c := range cases {
		got, ok := glofoxStatusToLeadStatus(c.raw)
		if got != c.want || ok != c.wantOK {
			t.Errorf("glofoxStatusToLeadStatus(%q) = (%q, %v), want (%q, %v)", c.raw, got, ok, c.want, c.wantOK)
		}
	}
}

func TestMapHeaders_GlofoxExportFormat(t *testing.T) {
	// The exact header row from a real Glofox member export.
	header := []string{
		"BarcodeID", "Client Name", "Status", "Membership Tier", "Phone", "Email Address",
		"Joined On", "Location", "Next AutoPay Date", "Next AutoPay Amount", "Lifetime Sales", "Is New Member?",
	}
	mapping := mapHeaders(header)

	want := map[string]int{
		"glofoxMemberId": 0,
		"name":           1,
		"status":         2,
		"plan":           3,
		"phone":          4,
		"email":          5,
	}
	for field, idx := range want {
		got, ok := mapping[field]
		if !ok {
			t.Errorf("mapHeaders did not map %q at all, want column %d", field, idx)
			continue
		}
		if got != idx {
			t.Errorf("mapHeaders[%q] = %d, want %d", field, got, idx)
		}
	}
	// Joined On / Location / Next AutoPay Date / Next AutoPay Amount / Lifetime
	// Sales / Is New Member? have no platform field to land in — just must not
	// get misattributed to one of the fields above.
	for key, idx := range mapping {
		if idx > 5 {
			t.Errorf("unexpected mapping %q -> column %d (should only be the first 6 columns)", key, idx)
		}
	}
}

func TestMapHeaders_StillHandlesThePlatformsOwnExportFormat(t *testing.T) {
	header := []string{"First Name", "Last Name", "Email", "Phone", "Plan", "Goals", "Notes", "Status"}
	mapping := mapHeaders(header)
	want := map[string]int{"firstName": 0, "lastName": 1, "email": 2, "phone": 3, "plan": 4, "goals": 5, "notes": 6, "status": 7}
	for field, idx := range want {
		if got, ok := mapping[field]; !ok || got != idx {
			t.Errorf("mapHeaders[%q] = %d,%v, want %d,true", field, got, ok, idx)
		}
	}
	if _, ok := mapping["glofoxMemberId"]; ok {
		t.Error("platform's own export has no BarcodeID column; glofoxMemberId should not be mapped")
	}
}

func TestImportLeads_GlofoxExportFormat(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	repo := NewRepo(pool)
	svc := NewService(repo, nil)

	studioID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO studios (id, slug, name, brand_color) VALUES ($1, $2, 'Glofox Import Test', '#7c3aed')`,
		studioID, "glofox-import-test-"+studioID.String()[:8]); err != nil {
		t.Fatalf("create test studio: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM studios WHERE id = $1`, studioID) })

	userID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, studio_id, email, password_hash, role) VALUES ($1,$2,$3,'x','studio_admin')`,
		userID, studioID, "glofox-import-test-"+userID.String()[:8]+"@example.com"); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })

	campaignID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO campaigns (id, studio_id, slug, name, fitness_plans, active, created_by) VALUES ($1,$2,'default','Default',ARRAY['General'],true,$3)`,
		campaignID, studioID, userID); err != nil {
		t.Fatalf("create test campaign: %v", err)
	}

	rows := [][]string{
		{"BarcodeID", "Client Name", "Status", "Membership Tier", "Phone", "Email Address", "Joined On", "Location", "Next AutoPay Date", "Next AutoPay Amount", "Lifetime Sales", "Is New Member?"},
		{"100002058", "Alastair Curry", "Active", "Class Passes", "8110 6114", "alastair.curry21@gmail.com", "2026-09-26", "Pagar", "", "0", "344", "Yes"},
		{"100000589", "Aloysius Lee", "Trial", "Foundation Limited", "8182 9868", "aloy2612@gmail.com", "2026-08-02", "Pagar", "2026-10-11", "60", "379", ""},
		{"100000590", "Cancelled Member", "Inactive", "Foundation Limited", "8182 9870", "cancelled@gmail.com", "2026-07-02", "Pagar", "", "0", "0", ""},
	}

	imported, err := svc.ImportLeads(ctx, studioID, campaignID, rows)
	if err != nil {
		t.Fatalf("ImportLeads: %v", err)
	}
	if imported != 3 {
		t.Fatalf("imported = %d, want 3", imported)
	}

	leads, _, err := repo.ListLeads(ctx, studioID, ListLeadsFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list leads: %v", err)
	}
	if len(leads) != 3 {
		t.Fatalf("got %d leads, want 3", len(leads))
	}

	byBarcode := map[string]Lead{}
	for _, l := range leads {
		byBarcode[l.GlofoxMemberID] = l
	}

	active, ok := byBarcode["100002058"]
	if !ok {
		t.Fatal("Active member (100002058) not found by glofox_member_id — BarcodeID column was dropped")
	}
	if active.Name != "Alastair Curry" {
		t.Errorf("Active member name = %q, want %q (Client Name column should map to name)", active.Name, "Alastair Curry")
	}
	if active.FitnessPlan != "Class Passes" {
		t.Errorf("Active member plan = %q, want %q (Membership Tier should map to plan)", active.FitnessPlan, "Class Passes")
	}
	if active.Status != StatusMember {
		t.Errorf("Active member status = %q, want %q", active.Status, StatusMember)
	}
	if active.Phone == "" {
		t.Error("Active member phone is empty")
	}
	if active.Email != "alastair.curry21@gmail.com" {
		t.Errorf("Active member email = %q", active.Email)
	}

	trial, ok := byBarcode["100000589"]
	if !ok {
		t.Fatal("Trial member (100000589) not found")
	}
	if trial.Status != StatusTrialBooked {
		t.Errorf("Trial member status = %q, want %q", trial.Status, StatusTrialBooked)
	}

	cancelled, ok := byBarcode["100000590"]
	if !ok {
		t.Fatal("Cancelled member (100000590) not found")
	}
	if cancelled.Status != StatusDropped {
		t.Errorf("Cancelled member status = %q, want %q", cancelled.Status, StatusDropped)
	}
}

// Real bug report: a studio uploaded an actual Glofox "member export" (a
// different, more detailed format than the Client-Name/BarcodeID one above —
// this one has separate First/Last Name columns, plus "Membership Name" and
// "Email Consent" columns whose headers also contain the words "name" and
// "email"). mapHeaders' old last-match-wins behaviour let those later,
// wrong columns silently overwrite the correct First Name/Last Name/Email
// mapping, so every imported lead showed its *membership plan* as its name
// ("Presale - Stage 2 Foundation", or even "Staff" for a staff membership
// plan) and had no phone/email at all.
func TestMapHeaders_GlofoxMemberExportFormat_DoesNotLetMembershipNameOrEmailConsentWin(t *testing.T) {
	header := []string{
		"Added", "First Name", "Last Name", "Email", "Phone", "Gender", "Date of Birth",
		"Street", "State", "City", "Country", "Zip Code", "Source", "Last Contacted",
		"Total Bookings", "Last Booking", "Total Attendances", "Membership Name", "Membership Plan",
		"Membership Expiry Date", "Credits Remaining", "Studio Waiver", "Email Consent", "SMS Consent",
	}
	mapping := mapHeaders(header)

	want := map[string]int{
		"firstName": 1,
		"lastName":  2,
		"email":     3,
		"phone":     4,
	}
	for field, idx := range want {
		got, ok := mapping[field]
		if !ok {
			t.Errorf("mapHeaders did not map %q at all, want column %d (%q)", field, idx, header[idx])
			continue
		}
		if got != idx {
			t.Errorf("mapHeaders[%q] = %d (%q), want %d (%q)", field, got, header[got], idx, header[idx])
		}
	}
	// "name" must not be mapped at all here — there's no actual full-name
	// column, only first/last — and it must never land on "Membership Name"
	// (17) or "Membership Plan" (18).
	if idx, ok := mapping["name"]; ok {
		t.Errorf(`mapHeaders["name"] = %d (%q), want unmapped (real columns are First/Last Name only)`, idx, header[idx])
	}
}

func TestImportLeads_GlofoxMemberExportFormat_NameAndContactSurviveMembershipColumns(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	repo := NewRepo(pool)
	svc := NewService(repo, nil)

	studioID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO studios (id, slug, name, brand_color) VALUES ($1, $2, 'Glofox Member Export Import Test', '#7c3aed')`,
		studioID, "glofox-member-export-test-"+studioID.String()[:8]); err != nil {
		t.Fatalf("create test studio: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM studios WHERE id = $1`, studioID) })

	userID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, studio_id, email, password_hash, role) VALUES ($1,$2,$3,'x','studio_admin')`,
		userID, studioID, "glofox-member-export-test-"+userID.String()[:8]+"@example.com"); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })

	campaignID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO campaigns (id, studio_id, slug, name, fitness_plans, active, created_by) VALUES ($1,$2,'default','Default',ARRAY['General'],true,$3)`,
		campaignID, studioID, userID); err != nil {
		t.Fatalf("create test campaign: %v", err)
	}

	// Real rows (anonymised) from the reported export, including one whose
	// Membership Name is literally "Staff" — the exact case that looked like
	// staff accounts were being imported as leads, when it was really this
	// same name-mapping bug.
	rows := [][]string{
		{"Added", "First Name", "Last Name", "Email", "Phone", "Gender", "Date of Birth",
			"Street", "State", "City", "Country", "Zip Code", "Source", "Last Contacted",
			"Total Bookings", "Last Booking", "Total Attendances", "Membership Name", "Membership Plan",
			"Membership Expiry Date", "Credits Remaining", "Studio Waiver", "Email Consent", "SMS Consent"},
		{"2026-08-01T12:09:00.000Z", "Abdu Salam", "Husin", "salamhusin78@gmail.com", "0177484749", "PREFER NOT TO SAY", "1978-02-21",
			"Jalan Jelatek", "Wilayah Persekutuan", "Kuala Lumpur", "Malaysia", "54200", "MEMBER_APP", "2026-08-25T09:18:35.000Z",
			"0", "2026-08-29T09:15:00.000Z", "0", "Presale - Stage 2 Foundation", "Presale - Stage 2 Foundation - Weekly",
			"02/10/2026", "0", "Not Accepted", "false", "true"},
		{"2026-07-01T06:14:47.310Z", "Ali", "Shah", "aliskywalker97@gmail.com", "0173105789", "PREFER NOT TO SAY", "1997-03-26",
			"", "", "", "", "", "MEMBER_APP", "2026-08-25T09:18:36.000Z",
			"5", "2026-08-17T09:30:00.000Z", "5", "Staff", "12 Month PIF",
			"01/07/2027", "0", "Not Accepted", "true", "true"},
	}

	imported, err := svc.ImportLeads(ctx, studioID, campaignID, rows)
	if err != nil {
		t.Fatalf("ImportLeads: %v", err)
	}
	if imported != 2 {
		t.Fatalf("imported = %d, want 2", imported)
	}

	leads, _, err := repo.ListLeads(ctx, studioID, ListLeadsFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list leads: %v", err)
	}
	byEmail := map[string]Lead{}
	for _, l := range leads {
		byEmail[l.Email] = l
	}

	abdu, ok := byEmail["salamhusin78@gmail.com"]
	if !ok {
		t.Fatal("lead with email salamhusin78@gmail.com not found — Email Consent must have overwritten the real Email column")
	}
	if abdu.Name != "Abdu Salam Husin" {
		t.Errorf("name = %q, want %q (Membership Name must not have overwritten First/Last Name)", abdu.Name, "Abdu Salam Husin")
	}
	if abdu.Phone != "0177484749" {
		t.Errorf("phone = %q, want %q (Last Contacted must not have overwritten the real Phone column)", abdu.Phone, "0177484749")
	}

	ali, ok := byEmail["aliskywalker97@gmail.com"]
	if !ok {
		t.Fatal("lead with email aliskywalker97@gmail.com not found")
	}
	if ali.Name != "Ali Shah" {
		t.Errorf(`name = %q, want "Ali Shah" (must come from First/Last Name, not the "Staff" membership plan name)`, ali.Name)
	}
}

func TestParseGlofoxExpiryDate(t *testing.T) {
	got, ok := parseGlofoxExpiryDate("02/10/2026")
	if !ok {
		t.Fatal("parseGlofoxExpiryDate(\"02/10/2026\") ok = false, want true")
	}
	// DD/MM/YYYY, not MM/DD/YYYY — day 30 in the sample data only makes sense
	// read that way, and this confirms it: day=2, month=10 (October).
	if got.Day() != 2 || got.Month() != time.October || got.Year() != 2026 {
		t.Errorf("parseGlofoxExpiryDate(\"02/10/2026\") = %v, want 2026-10-02", got)
	}
	if _, ok := parseGlofoxExpiryDate(""); ok {
		t.Error("parseGlofoxExpiryDate(\"\") ok = true, want false")
	}
	if _, ok := parseGlofoxExpiryDate("not a date"); ok {
		t.Error("parseGlofoxExpiryDate(\"not a date\") ok = true, want false")
	}
}

// This export format (see the two tests above) has no Active/Trial/Inactive
// status column at all, so status has to be read off the membership itself:
// a "Presale" plan hasn't started yet (trial-stage), a membership expiring in
// the future is a current member, and one that's already expired is dropped.
func TestImportLeads_GlofoxMemberExportFormat_InfersStatusFromMembershipExpiry(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	repo := NewRepo(pool)
	svc := NewService(repo, nil)

	studioID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO studios (id, slug, name, brand_color) VALUES ($1, $2, 'Glofox Expiry Status Test', '#7c3aed')`,
		studioID, "glofox-expiry-status-test-"+studioID.String()[:8]); err != nil {
		t.Fatalf("create test studio: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM studios WHERE id = $1`, studioID) })

	userID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, studio_id, email, password_hash, role) VALUES ($1,$2,$3,'x','studio_admin')`,
		userID, studioID, "glofox-expiry-status-test-"+userID.String()[:8]+"@example.com"); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })

	campaignID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO campaigns (id, studio_id, slug, name, fitness_plans, active, created_by) VALUES ($1,$2,'default','Default',ARRAY['General'],true,$3)`,
		campaignID, studioID, userID); err != nil {
		t.Fatalf("create test campaign: %v", err)
	}

	future := time.Now().AddDate(1, 0, 0).Format("02/01/2006")
	past := time.Now().AddDate(-1, 0, 0).Format("02/01/2006")

	header := []string{
		"Added", "First Name", "Last Name", "Email", "Phone", "Gender", "Date of Birth",
		"Street", "State", "City", "Country", "Zip Code", "Source", "Last Contacted",
		"Total Bookings", "Last Booking", "Total Attendances", "Membership Name", "Membership Plan",
		"Membership Expiry Date", "Credits Remaining", "Studio Waiver", "Email Consent", "SMS Consent",
	}
	rows := [][]string{header,
		{"2026-08-01", "Presale", "Signup", "presale@example.com", "0177484001", "", "",
			"", "", "", "", "", "MEMBER_APP", "2026-08-25",
			"0", "", "0", "Presale - Stage 2 Foundation", "Presale - Stage 2 Foundation - Weekly",
			future, "0", "Not Accepted", "false", "true"},
		{"2026-08-01", "Current", "Member", "current@example.com", "0177484002", "", "",
			"", "", "", "", "", "MEMBER_APP", "2026-08-25",
			"0", "", "0", "Full Membership", "12 Month PIF",
			future, "0", "Not Accepted", "false", "true"},
		{"2026-08-01", "Expired", "Member", "expired@example.com", "0177484003", "", "",
			"", "", "", "", "", "MEMBER_APP", "2026-08-25",
			"0", "", "0", "Full Membership", "12 Month PIF",
			past, "0", "Not Accepted", "false", "true"},
	}

	imported, err := svc.ImportLeads(ctx, studioID, campaignID, rows)
	if err != nil {
		t.Fatalf("ImportLeads: %v", err)
	}
	if imported != 3 {
		t.Fatalf("imported = %d, want 3", imported)
	}

	leads, _, err := repo.ListLeads(ctx, studioID, ListLeadsFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list leads: %v", err)
	}
	byEmail := map[string]Lead{}
	for _, l := range leads {
		byEmail[l.Email] = l
	}

	if l, ok := byEmail["presale@example.com"]; !ok || l.Status != StatusTrialBooked {
		t.Errorf("presale lead status = %v (found=%v), want %q", l.Status, ok, StatusTrialBooked)
	}
	if l, ok := byEmail["current@example.com"]; !ok || l.Status != StatusMember {
		t.Errorf("future-expiry lead status = %v (found=%v), want %q", l.Status, ok, StatusMember)
	}
	if l, ok := byEmail["expired@example.com"]; !ok || l.Status != StatusDropped {
		t.Errorf("past-expiry lead status = %v (found=%v), want %q", l.Status, ok, StatusDropped)
	}
}

// Real bug: every imported lead used to be handed straight to the
// auto-contact worker (which sends the "want to book a trial / become a
// member" opener) regardless of status — so importing a Glofox export full
// of already-active members would immediately WhatsApp all of them with a
// new-lead sales pitch. Fixed: import never auto-contacts anyone, full stop.
// Connecting AI is now a separate, explicit action (EnableAutoContactForLeadsByStatus).
func TestImportLeads_NeverAutoContacts(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	repo := NewRepo(pool)
	svc := NewService(repo, nil)

	studioID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO studios (id, slug, name, brand_color) VALUES ($1, $2, 'Import Never AutoContacts Test', '#7c3aed')`,
		studioID, "import-never-autocontacts-test-"+studioID.String()[:8]); err != nil {
		t.Fatalf("create test studio: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM studios WHERE id = $1`, studioID) })

	userID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, studio_id, email, password_hash, role) VALUES ($1,$2,$3,'x','studio_admin')`,
		userID, studioID, "import-never-autocontacts-test-"+userID.String()[:8]+"@example.com"); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })

	campaignID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO campaigns (id, studio_id, slug, name, fitness_plans, active, created_by) VALUES ($1,$2,'default','Default',ARRAY['General'],true,$3)`,
		campaignID, studioID, userID); err != nil {
		t.Fatalf("create test campaign: %v", err)
	}

	header := []string{"Client Name", "Status", "Phone", "Email Address"}
	rows := [][]string{
		header,
		{"New Lead", "", "0177400001", "never-ac-new@example.com"},
		{"Active Member", "Active", "0177400002", "never-ac-member@example.com"},
	}

	if _, err := svc.ImportLeads(ctx, studioID, campaignID, rows); err != nil {
		t.Fatalf("ImportLeads: %v", err)
	}

	var n int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM outbox o JOIN leads l ON l.id = o.aggregate_id
		WHERE l.studio_id = $1 AND o.destination = 'lead_autocontact'
	`, studioID).Scan(&n); err != nil {
		t.Fatalf("count autocontact outbox: %v", err)
	}
	if n != 0 {
		t.Errorf("import queued %d autocontact outbox rows, want 0 — import must never auto-contact anyone", n)
	}
}

// The "Enable AI" control on the Leads page: pick a status, connect every
// current lead at that status. Must only touch leads actually AT the chosen
// status (a member must not get connected when enabling "new"), must skip a
// lead that's already been contacted (no double-messaging), and must be
// safe to call twice in a row (the second call enqueues nothing new).
func TestEnableAutoContactForLeadsByStatus(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	repo := NewRepo(pool)
	svc := NewService(repo, nil)

	studioID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO studios (id, slug, name, brand_color) VALUES ($1, $2, 'Enable AutoContact By Status Test', '#7c3aed')`,
		studioID, "enable-autocontact-status-test-"+studioID.String()[:8]); err != nil {
		t.Fatalf("create test studio: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM studios WHERE id = $1`, studioID) })

	userID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, studio_id, email, password_hash, role) VALUES ($1,$2,$3,'x','studio_admin')`,
		userID, studioID, "enable-autocontact-status-test-"+userID.String()[:8]+"@example.com"); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })

	campaignID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO campaigns (id, studio_id, slug, name, fitness_plans, active, created_by) VALUES ($1,$2,'default','Default',ARRAY['General'],true,$3)`,
		campaignID, studioID, userID); err != nil {
		t.Fatalf("create test campaign: %v", err)
	}

	rows := [][]string{
		{"Client Name", "Status", "Phone", "Email Address"},
		{"New One", "", "0177500001", "enable-new1@example.com"},
		{"New Two", "", "0177500002", "enable-new2@example.com"},
		{"Active Member", "Active", "0177500003", "enable-member@example.com"},
	}
	if _, err := svc.ImportLeads(ctx, studioID, campaignID, rows); err != nil {
		t.Fatalf("ImportLeads: %v", err)
	}

	// Simulate "New Two" already having been contacted (e.g. via a prior
	// manual message) — it must be skipped, not double-messaged.
	if _, err := pool.Exec(ctx, `UPDATE leads SET contact_attempts = 1, last_contacted_at = now() WHERE studio_id = $1 AND email = 'enable-new2@example.com'`, studioID); err != nil {
		t.Fatalf("mark lead contacted: %v", err)
	}

	autocontactCount := func(email string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM outbox o JOIN leads l ON l.id = o.aggregate_id
			WHERE l.studio_id = $1 AND l.email = $2 AND o.destination = 'lead_autocontact'
		`, studioID, email).Scan(&n); err != nil {
			t.Fatalf("count autocontact outbox for %s: %v", email, err)
		}
		return n
	}

	enqueued, err := svc.EnableAutoContactForLeadsByStatuses(ctx, studioID, []LeadStatus{StatusNew})
	if err != nil {
		t.Fatalf("EnableAutoContactForLeadsByStatus: %v", err)
	}
	if enqueued != 1 {
		t.Fatalf("enqueued = %d, want 1 (only the never-contacted new lead)", enqueued)
	}
	if n := autocontactCount("enable-new1@example.com"); n != 1 {
		t.Errorf("never-contacted new lead has %d autocontact outbox rows, want 1", n)
	}
	if n := autocontactCount("enable-new2@example.com"); n != 0 {
		t.Errorf("already-contacted new lead has %d autocontact outbox rows, want 0 — must not double-message", n)
	}
	if n := autocontactCount("enable-member@example.com"); n != 0 {
		t.Errorf("member lead has %d autocontact outbox rows, want 0 — enabling \"new\" must not touch other statuses", n)
	}

	// Calling it again must be a no-op — nothing new queued for the lead
	// already connected the first time.
	enqueued2, err := svc.EnableAutoContactForLeadsByStatuses(ctx, studioID, []LeadStatus{StatusNew})
	if err != nil {
		t.Fatalf("EnableAutoContactForLeadsByStatus (2nd call): %v", err)
	}
	if enqueued2 != 0 {
		t.Errorf("2nd call enqueued = %d, want 0 (already connected)", enqueued2)
	}
	if n := autocontactCount("enable-new1@example.com"); n != 1 {
		t.Errorf("after 2nd call, new lead has %d autocontact outbox rows, want still 1 (not doubled)", n)
	}
}

// The "Enable AI" control lets the studio pick more than one status at once
// (e.g. New + Contacted) — both should connect, a third status left
// unselected must not.
func TestEnableAutoContactForLeadsByStatuses_Multiple(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	repo := NewRepo(pool)
	svc := NewService(repo, nil)

	studioID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO studios (id, slug, name, brand_color) VALUES ($1, $2, 'Enable AutoContact Multi Status Test', '#7c3aed')`,
		studioID, "enable-autocontact-multi-test-"+studioID.String()[:8]); err != nil {
		t.Fatalf("create test studio: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM studios WHERE id = $1`, studioID) })

	userID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, studio_id, email, password_hash, role) VALUES ($1,$2,$3,'x','studio_admin')`,
		userID, studioID, "enable-autocontact-multi-test-"+userID.String()[:8]+"@example.com"); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })

	campaignID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO campaigns (id, studio_id, slug, name, fitness_plans, active, created_by) VALUES ($1,$2,'default','Default',ARRAY['General'],true,$3)`,
		campaignID, studioID, userID); err != nil {
		t.Fatalf("create test campaign: %v", err)
	}

	newID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO leads (id, studio_id, campaign_id, name, fitness_plan, email, phone, source, status) VALUES ($1,$2,$3,'New Lead','','multi-new@example.com','0177600001','test','new')`,
		newID, studioID, campaignID); err != nil {
		t.Fatalf("seed new lead: %v", err)
	}
	contactedID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO leads (id, studio_id, campaign_id, name, fitness_plan, email, phone, source, status) VALUES ($1,$2,$3,'Contacted Lead','','multi-contacted@example.com','0177600002','test','contacted')`,
		contactedID, studioID, campaignID); err != nil {
		t.Fatalf("seed contacted lead: %v", err)
	}
	droppedID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO leads (id, studio_id, campaign_id, name, fitness_plan, email, phone, source, status) VALUES ($1,$2,$3,'Dropped Lead','','multi-dropped@example.com','0177600003','test','dropped')`,
		droppedID, studioID, campaignID); err != nil {
		t.Fatalf("seed dropped lead: %v", err)
	}

	enqueued, err := svc.EnableAutoContactForLeadsByStatuses(ctx, studioID, []LeadStatus{StatusNew, StatusContacted})
	if err != nil {
		t.Fatalf("EnableAutoContactForLeadsByStatuses: %v", err)
	}
	if enqueued != 2 {
		t.Fatalf("enqueued = %d, want 2 (new + contacted)", enqueued)
	}

	count := func(leadID uuid.UUID) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE aggregate_id = $1 AND destination = 'lead_autocontact'`, leadID).Scan(&n); err != nil {
			t.Fatalf("count autocontact outbox: %v", err)
		}
		return n
	}
	if n := count(newID); n != 1 {
		t.Errorf("new lead autocontact rows = %d, want 1", n)
	}
	if n := count(contactedID); n != 1 {
		t.Errorf("contacted lead autocontact rows = %d, want 1", n)
	}
	if n := count(droppedID); n != 0 {
		t.Errorf("dropped lead autocontact rows = %d, want 0 — status wasn't selected", n)
	}
}
