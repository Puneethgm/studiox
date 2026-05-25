package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/secrets"
)

func TestAutoContactWorker_Integration(t *testing.T) {
	// 1. Load config and connect to DB
	cwd, _ := os.Getwd()
	t.Logf("CWD: %s", cwd)
	err := godotenv.Load("../../../../.env")
	if err != nil {
		t.Logf("godotenv load error: %v", err)
	}
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
		t.Fatalf("Connect to DB: %v", err)
	}
	defer pool.Close()

	// 2. Fetch a campaign to submit lead to
	var campaignID, studioID uuid.UUID
	var campaignSlug, studioSlug, campaignName, studioName string
	var fitnessPlans []string
	err = pool.QueryRow(ctx, `
		SELECT c.id, c.studio_id, c.slug, s.slug, c.name, s.name, c.fitness_plans
		FROM campaigns c
		JOIN studios s ON s.id = c.studio_id
		JOIN channel_accounts ca ON ca.studio_id = s.id
		WHERE ca.kind = 'whatsapp_meta' AND ca.status = 'active'
		LIMIT 1
	`).Scan(&campaignID, &studioID, &campaignSlug, &studioSlug, &campaignName, &studioName, &fitnessPlans)
	if err != nil {
		t.Skip("Skipping test; no campaign with active WhatsApp channel found in DB")
	}
	if len(fitnessPlans) == 0 {
		t.Fatalf("Fetched campaign has no fitness plans configured")
	}

	// 3. Setup repositories & services
	leadsRepo := leads.NewRepo(pool)
	leadsSvc := leads.NewService(leadsRepo)

	// Token encryption key
	keyB64 := os.Getenv("TOKEN_ENCRYPTION_KEY")
	cipher, err := secrets.New(keyB64)
	if err != nil {
		t.Fatalf("Init cipher: %v", err)
	}

	msgRepo := NewRepo(pool, cipher)
	msgBus := NewInProcBus()
	msgSvc := NewService(msgRepo, msgBus, "")

	// 4. Submit a public lead
	leadName := "Test User"
	lead, errs, err := leadsSvc.SubmitPublicLead(ctx, leads.SubmitLeadInput{
		StudioSlug:   studioSlug,
		CampaignSlug: campaignSlug,
		Name:         leadName,
		Email:        "test-lead@example.com",
		Phone:        "9999999999",
		FitnessPlan:  fitnessPlans[0],
		Goals:        "Try it out",
	})
	if err != nil {
		t.Fatalf("SubmitPublicLead: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("SubmitPublicLead validation errs: %v", errs)
	}

	// 5. Query outbox to verify CampaignName is populated in the payload!
	var payload []byte
	err = pool.QueryRow(ctx, `
		SELECT payload FROM outbox 
		WHERE aggregate_type = 'lead' AND aggregate_id = $1 AND destination = 'lead_autocontact'
		LIMIT 1
	`, lead.ID).Scan(&payload)
	if err != nil {
		t.Fatalf("Query outbox payload: %v", err)
	}

	var parsedLead leads.Lead
	if err := json.Unmarshal(payload, &parsedLead); err != nil {
		t.Fatalf("Unmarshal outbox payload: %v", err)
	}

	if parsedLead.CampaignName != campaignName {
		t.Errorf("CampaignName in payload = %q; want %q", parsedLead.CampaignName, campaignName)
	}
	if parsedLead.StudioName != studioName {
		t.Errorf("StudioName in payload = %q; want %q", parsedLead.StudioName, studioName)
	}

	// 6. Run the autocontact worker tick manually
	logger := slog.Default()
	autoWorker := NewAutoContactWorker(leadsRepo, msgRepo, msgSvc, logger)
	autoWorker.tick(ctx)

	// 7. Verify that outbound job was created with correct message content
	var body string
	err = pool.QueryRow(ctx, `
		SELECT body FROM outbound_jobs 
		WHERE studio_id = $1 AND source_ref = $2
		LIMIT 1
	`, studioID, fmt.Sprintf("lead:%s:followup:0", lead.ID.String())).Scan(&body)
	if err != nil {
		t.Fatalf("Query outbound job body: %v", err)
	}

	expectedText := fmt.Sprintf("Hi {{contact.first_name}}, we saw your interest in {{campaign.name}} for %s. I’m from {{studio.name}} — would you like to get started? Please select an option:\n1. Interested\n2. Not Interested", fitnessPlans[0])
	if body != expectedText {
		t.Errorf("Outbound job body = %q; want %q", body, expectedText)
	}

	t.Logf("Integration test passed successfully! Message body: %s", body)
}

func TestAutoContactWorker_TrialBooked_Integration(t *testing.T) {
	// 1. Load config and connect to DB
	_ = godotenv.Load("../../../../.env")
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
		t.Fatalf("Connect to DB: %v", err)
	}
	defer pool.Close()

	// 2. Fetch a campaign
	var campaignID, studioID uuid.UUID
	var campaignSlug, studioSlug, campaignName, studioName string
	var fitnessPlans []string
	err = pool.QueryRow(ctx, `
		SELECT c.id, c.studio_id, c.slug, s.slug, c.name, s.name, c.fitness_plans
		FROM campaigns c
		JOIN studios s ON s.id = c.studio_id
		JOIN channel_accounts ca ON ca.studio_id = s.id
		WHERE ca.kind = 'whatsapp_meta' AND ca.status = 'active'
		LIMIT 1
	`).Scan(&campaignID, &studioID, &campaignSlug, &studioSlug, &campaignName, &studioName, &fitnessPlans)
	if err != nil {
		t.Skip("Skipping test; no campaign with active WhatsApp channel found in DB")
	}

	// Update campaign to ensure it has a "Trial Class" fitness plan
	hasTrial := false
	for _, p := range fitnessPlans {
		if strings.Contains(strings.ToLower(p), "trial") {
			hasTrial = true
			break
		}
	}
	trialPlan := "Trial Class"
	if !hasTrial {
		_, err = pool.Exec(ctx, `
			UPDATE campaigns 
			SET fitness_plans = array_append(fitness_plans, $2)
			WHERE id = $1
		`, campaignID, trialPlan)
		if err != nil {
			t.Fatalf("Ensure campaign trial plan: %v", err)
		}
	} else {
		// Use the existing trial plan
		for _, p := range fitnessPlans {
			if strings.Contains(strings.ToLower(p), "trial") {
				trialPlan = p
				break
			}
		}
	}

	// 3. Setup repositories & services
	leadsRepo := leads.NewRepo(pool)
	leadsSvc := leads.NewService(leadsRepo)

	keyB64 := os.Getenv("TOKEN_ENCRYPTION_KEY")
	cipher, err := secrets.New(keyB64)
	if err != nil {
		t.Fatalf("Init cipher: %v", err)
	}

	msgRepo := NewRepo(pool, cipher)
	msgBus := NewInProcBus()
	msgSvc := NewService(msgRepo, msgBus, "")

	// 4. Submit a public lead with a "trial" fitness plan
	leadName := "Test User"
	lead, errs, err := leadsSvc.SubmitPublicLead(ctx, leads.SubmitLeadInput{
		StudioSlug:   studioSlug,
		CampaignSlug: campaignSlug,
		Name:         leadName,
		Email:        "trial-booked-test@example.com",
		Phone:        "9999999999",
		FitnessPlan:  trialPlan,
		Goals:        "Try class",
	})
	if err != nil {
		t.Fatalf("SubmitPublicLead: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("SubmitPublicLead validation errs: %v", errs)
	}

	if lead.Status != leads.StatusTrialBooked {
		t.Errorf("Lead status = %s; want %s", lead.Status, leads.StatusTrialBooked)
	}

	// 5. Run the autocontact worker tick manually
	logger := slog.Default()
	autoWorker := NewAutoContactWorker(leadsRepo, msgRepo, msgSvc, logger)
	autoWorker.tick(ctx)

	// 6. Verify that 1-day check-in follow-up was enqueued
	var followupBody string
	var scheduledFor time.Time
	err = pool.QueryRow(ctx, `
		SELECT body, scheduled_for FROM outbound_jobs 
		WHERE studio_id = $1 AND source_ref = $2
		LIMIT 1
	`, studioID, fmt.Sprintf("lead:%s:trial_followup:1day", lead.ID.String())).Scan(&followupBody, &scheduledFor)
	if err != nil {
		t.Fatalf("Query trial followup job: %v", err)
	}

	expectedFollowup := "Hi {{contact.first_name}}, we hope you're excited for your trial! Ready to take the next step and become a member? Please select an option:\n1. Book a Trial\n2. Become a Member"
	if followupBody != expectedFollowup {
		t.Errorf("Trial followup body = %q; want %q", followupBody, expectedFollowup)
	}

	timeDiff := scheduledFor.Sub(time.Now().UTC())
	if timeDiff < 23*time.Hour || timeDiff > 25*time.Hour {
		t.Errorf("ScheduledFor = %s; want ~24 hours from now (diff: %s)", scheduledFor, timeDiff)
	}

	t.Logf("TrialBooked integration test passed successfully!")
}

func TestService_HandleInboundWhatsAppMessage_StatusTransitions(t *testing.T) {
	_ = godotenv.Load("../../../../.env")
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
		t.Fatalf("Connect to DB: %v", err)
	}
	defer pool.Close()

	// 1. Fetch/Ensure studio and active channel
	var studioID uuid.UUID
	var channelID, externalID string
	err = pool.QueryRow(ctx, `
		SELECT s.id, c.id, c.external_id
		FROM channel_accounts c
		JOIN studios s ON s.id = c.studio_id
		WHERE c.kind = 'whatsapp_meta' AND c.status = 'active'
		LIMIT 1
	`).Scan(&studioID, &channelID, &externalID)
	if err != nil {
		t.Skip("Skipping integration test; no active WhatsApp channel found in DB")
	}

	// 2. Fetch/Create a campaign slug and studio slug
	var campaignID uuid.UUID
	var campaignSlug, studioSlug, campaignName string
	var fitnessPlans []string
	err = pool.QueryRow(ctx, `
		SELECT c.id, c.slug, s.slug, c.name, c.fitness_plans
		FROM campaigns c
		JOIN studios s ON s.id = c.studio_id
		WHERE c.studio_id = $1 AND c.active = true
		LIMIT 1
	`, studioID).Scan(&campaignID, &campaignSlug, &studioSlug, &campaignName, &fitnessPlans)
	if err != nil {
		t.Fatalf("Query campaign: %v", err)
	}

	// Update campaign to ensure it has a fitness plan
	trialPlan := "Standard Class"
	if len(fitnessPlans) > 0 {
		trialPlan = fitnessPlans[0]
	} else {
		_, err = pool.Exec(ctx, `
			UPDATE campaigns 
			SET fitness_plans = array_append(fitness_plans, $2)
			WHERE id = $1
		`, campaignID, trialPlan)
		if err != nil {
			t.Fatalf("Ensure campaign trial plan: %v", err)
		}
	}

	leadsRepo := leads.NewRepo(pool)
	leadsSvc := leads.NewService(leadsRepo)

	keyB64 := os.Getenv("TOKEN_ENCRYPTION_KEY")
	cipher, err := secrets.New(keyB64)
	if err != nil {
		t.Fatalf("Init cipher: %v", err)
	}

	msgRepo := NewRepo(pool, cipher)
	msgBus := NewInProcBus()
	msgSvc := NewService(msgRepo, msgBus, "")

	phone := fmt.Sprintf("15555%06d", time.Now().UnixNano()%1000000)

	// 3. Submit a public lead
	lead, errs, err := leadsSvc.SubmitPublicLead(ctx, leads.SubmitLeadInput{
		StudioSlug:   studioSlug,
		CampaignSlug: campaignSlug,
		Name:         "Webhook Inbound User",
		Email:        "webhook-inbound-test@example.com",
		Phone:        phone,
		FitnessPlan:  trialPlan,
		Goals:        "Get fit",
	})
	if err != nil || len(errs) > 0 {
		t.Fatalf("SubmitPublicLead: %v, validation: %v", err, errs)
	}

	// 4. Create the conversation and link the lead explicitly using the new CreateConversation
	_, err = msgSvc.CreateConversation(ctx, studioID, CreateConversationInput{
		ChannelKind:  KindWhatsAppMeta,
		ContactValue: phone,
		DisplayName:  "Webhook Inbound User",
		LeadID:       &lead.ID,
	})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	err = leadsRepo.UpdateAutoContactStage(ctx, studioID, lead.ID, "awaiting_interest")
	if err != nil {
		t.Fatalf("UpdateAutoContactStage: %v", err)
	}

	// 5a. Simulate "Interested" message click
	inboundInterested := channels.WhatsAppWebhookMessage{
		ID:        "wamid.test_interested_" + uuid.New().String(),
		From:      phone,
		Timestamp: fmt.Sprintf("%d", time.Now().Unix()),
		Interactive: &channels.WhatsAppWebhookInteractive{
			Type: "button_reply",
			ButtonReply: &channels.WhatsAppWebhookInteractiveReply{
				ID:    "choice_interested",
				Title: "Interested",
			},
		},
	}

	err = msgSvc.HandleInboundWhatsAppMessage(ctx, "waba_id_dummy", channels.WhatsAppWebhookMetadata{
		PhoneNumberID:      externalID,
		DisplayPhoneNumber: "+" + phone,
	}, nil, inboundInterested)
	if err != nil {
		t.Fatalf("HandleInboundWhatsAppMessage Interested: %v", err)
	}

	// Verify lead stage and status after Interested response
	updatedLead, err := leadsRepo.GetLead(ctx, studioID, lead.ID)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if updatedLead.Status != leads.StatusContacted {
		t.Errorf("After Interested, Lead status = %s; want %s", updatedLead.Status, leads.StatusContacted)
	}
	if updatedLead.AutoContactStage != "awaiting_options" {
		t.Errorf("After Interested, Lead stage = %s; want %s", updatedLead.AutoContactStage, "awaiting_options")
	}

	// 5b. Simulate "Book a Trial" message click
	inboundTrial := channels.WhatsAppWebhookMessage{
		ID:        "wamid.test_trial_" + uuid.New().String(),
		From:      phone,
		Timestamp: fmt.Sprintf("%d", time.Now().Unix()),
		Interactive: &channels.WhatsAppWebhookInteractive{
			Type: "button_reply",
			ButtonReply: &channels.WhatsAppWebhookInteractiveReply{
				ID:    "choice_trial",
				Title: "Book a Trial",
			},
		},
	}

	err = msgSvc.HandleInboundWhatsAppMessage(ctx, "waba_id_dummy", channels.WhatsAppWebhookMetadata{
		PhoneNumberID:      externalID,
		DisplayPhoneNumber: "+" + phone,
	}, nil, inboundTrial)
	if err != nil {
		t.Fatalf("HandleInboundWhatsAppMessage Trial: %v", err)
	}

	// Verify lead status was updated to trial_booked and stage to awaiting_trial_date
	updatedLead, err = leadsRepo.GetLead(ctx, studioID, lead.ID)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if updatedLead.Status != leads.StatusTrialBooked {
		t.Errorf("After Trial Choice, Lead status = %s; want %s", updatedLead.Status, leads.StatusTrialBooked)
	}
	if updatedLead.AutoContactStage != "awaiting_trial_date" {
		t.Errorf("After Trial Choice, Lead stage = %s; want %s", updatedLead.AutoContactStage, "awaiting_trial_date")
	}

	// 5c. Simulate selecting a date option (1)
	inboundDate := channels.WhatsAppWebhookMessage{
		ID:        "wamid.test_date_" + uuid.New().String(),
		From:      phone,
		Timestamp: fmt.Sprintf("%d", time.Now().Unix()),
		Text: &struct {
			Body string `json:"body"`
		}{
			Body: "1",
		},
	}

	err = msgSvc.HandleInboundWhatsAppMessage(ctx, "waba_id_dummy", channels.WhatsAppWebhookMetadata{
		PhoneNumberID:      externalID,
		DisplayPhoneNumber: "+" + phone,
	}, nil, inboundDate)
	if err != nil {
		t.Fatalf("HandleInboundWhatsAppMessage Date: %v", err)
	}

	// Verify lead stage transitions to awaiting_trial_time and notes updated with date
	updatedLead, err = leadsRepo.GetLead(ctx, studioID, lead.ID)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if updatedLead.AutoContactStage != "awaiting_trial_time" {
		t.Errorf("After Date, Lead stage = %s; want %s", updatedLead.AutoContactStage, "awaiting_trial_time")
	}
	if !strings.Contains(updatedLead.Notes, "[Selected Trial Date]:") {
		t.Errorf("Lead notes = %q; want it to contain %q", updatedLead.Notes, "[Selected Trial Date]:")
	}

	// 5d. Simulate selecting a time slot option (1)
	inboundTime := channels.WhatsAppWebhookMessage{
		ID:        "wamid.test_time_" + uuid.New().String(),
		From:      phone,
		Timestamp: fmt.Sprintf("%d", time.Now().Unix()),
		Text: &struct {
			Body string `json:"body"`
		}{
			Body: "1",
		},
	}

	err = msgSvc.HandleInboundWhatsAppMessage(ctx, "waba_id_dummy", channels.WhatsAppWebhookMetadata{
		PhoneNumberID:      externalID,
		DisplayPhoneNumber: "+" + phone,
	}, nil, inboundTime)
	if err != nil {
		t.Fatalf("HandleInboundWhatsAppMessage Time: %v", err)
	}

	// Verify lead stage completed and notes appended with full slot
	updatedLead, err = leadsRepo.GetLead(ctx, studioID, lead.ID)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if updatedLead.AutoContactStage != "completed" {
		t.Errorf("After Time, Lead stage = %s; want %s", updatedLead.AutoContactStage, "completed")
	}
	if !strings.Contains(updatedLead.Notes, "[Selected Trial Slot]:") || !strings.Contains(updatedLead.Notes, "09:00 AM") {
		t.Errorf("Lead notes = %q; want it to contain %q and %q", updatedLead.Notes, "[Selected Trial Slot]:", "09:00 AM")
	}

	t.Logf("Successfully verified multi-step conversation flow status transitions!")
}
