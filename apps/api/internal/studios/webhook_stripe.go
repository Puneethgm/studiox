package studios

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/httpx"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/client"
	"github.com/stripe/stripe-go/v78/webhook"
)

type StripeWebhookHandler struct {
	svc           *Service
	webhookSecret string
}

func NewStripeWebhookHandler(svc *Service, webhookSecret string) *StripeWebhookHandler {
	return &StripeWebhookHandler{
		svc:           svc,
		webhookSecret: webhookSecret,
	}
}

func (h *StripeWebhookHandler) HandleInbound(w http.ResponseWriter, r *http.Request) {
	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "read_error", "Error reading request body")
		return
	}

	// Try to resolve the specific studio's webhook secret first if isolated routing is used
	var endpointSecret string
	studioIDStr := chi.URLParam(r, "studioId")
	if studioIDStr != "" {
		if id, err := uuid.Parse(studioIDStr); err == nil {
			studio, err := h.svc.GetByID(r.Context(), id)
			if err == nil && studio != nil && studio.StripeWebhookSecret != "" {
				endpointSecret = studio.StripeWebhookSecret
			}
		}
	}
	if endpointSecret == "" {
		endpointSecret = h.webhookSecret
	}

	var event stripe.Event

	if endpointSecret != "" {
		signatureHeader := r.Header.Get("Stripe-Signature")
		event, err = webhook.ConstructEvent(payload, signatureHeader, endpointSecret)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad_signature", "Error verifying webhook signature")
			return
		}
	} else {
		var raw map[string]any
		if err := json.Unmarshal(payload, &raw); err != nil {
			fmt.Printf("[Stripe Webhook] JSON unmarshal error: %v\n", err)
			httpx.WriteError(w, http.StatusBadRequest, "invalid_payload", "Error parsing webhook JSON")
			return
		}
		
		typ, _ := raw["type"].(string)
		fmt.Printf("[Stripe Webhook Tracker] Received event type: %s\n", typ)
		
		if typ == "checkout.session.completed" || typ == "payment_link.payment.completed" {
			data, _ := raw["data"].(map[string]any)
			obj, _ := data["object"].(map[string]any)
			
			objBytes, errMarshal := json.Marshal(obj)
			if errMarshal != nil {
				fmt.Printf("[Stripe Webhook Tracker] Marshal object error: %v\n", errMarshal)
			}
			
			var session stripe.CheckoutSession
			if err := json.Unmarshal(objBytes, &session); err == nil {
				go h.handleCheckoutComplete(context.Background(), &session)
			} else {
				fmt.Printf("[Stripe Webhook Tracker] Session Unmarshal error: %v\n", err)
			}
		}
		httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	// Handle the event (for when secret is used)
	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		rawBytes := event.Data.Raw
		if len(rawBytes) == 0 {
			rawBytes, _ = json.Marshal(event.Data.Object)
		}
		if err := json.Unmarshal(rawBytes, &session); err == nil {
			go h.handleCheckoutComplete(context.Background(), &session)
		}
	case "payment_link.payment.completed":
		var session stripe.CheckoutSession
		rawBytes := event.Data.Raw
		if len(rawBytes) == 0 {
			rawBytes, _ = json.Marshal(event.Data.Object)
		}
		if err := json.Unmarshal(rawBytes, &session); err == nil {
			go h.handleCheckoutComplete(context.Background(), &session)
		}
	case "invoice.paid":
		// Existing invoice handling (platform billing)
	case "invoice.payment_failed":
		var invoice stripe.Invoice
		rawBytes := event.Data.Raw
		if len(rawBytes) == 0 {
			rawBytes, _ = json.Marshal(event.Data.Object)
		}
		if err := json.Unmarshal(rawBytes, &invoice); err == nil {
			if invoice.Subscription != nil {
				secretKey, _ := h.svc.GetPlatformSetting(context.Background(), "stripe_secret_key")
				if secretKey != "" {
					sc := &client.API{}
					sc.Init(secretKey, nil)
					sub, err := sc.Subscriptions.Get(invoice.Subscription.ID, nil)
					if err == nil && sub.Metadata["studio_id"] != "" {
						id, err := uuid.Parse(sub.Metadata["studio_id"])
						if err == nil {
							// Set the studio tier to 'past_due'
							_ = h.svc.UpdatePayments(context.Background(), id, "", "", "", "", "past_due")
							fmt.Printf("[Stripe Webhook] Marked studio %s as past_due\n", sub.Metadata["studio_id"])
						}
					}
				}
			}
		}
	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		rawBytes := event.Data.Raw
		if len(rawBytes) == 0 {
			rawBytes, _ = json.Marshal(event.Data.Object)
		}
		if err := json.Unmarshal(rawBytes, &sub); err == nil {
			if sub.Metadata["studio_id"] != "" {
				id, err := uuid.Parse(sub.Metadata["studio_id"])
				if err == nil {
					if sub.Status == "canceled" || sub.CancelAtPeriodEnd {
						_ = h.svc.UpdatePayments(context.Background(), id, "", "", "", "", "canceled")
						fmt.Printf("[Stripe Webhook] Marked studio %s as canceled\n", sub.Metadata["studio_id"])
					} else {
						// If they un-cancel, or upgrade
						// Wait, if it's updated and NOT canceled, we shouldn't necessarily override unless we know the tier.
						// The tier is stored in sub.Metadata["plan_tier"] usually, but we set it on checkout.
						// We can ignore updates that aren't cancellations to avoid overwriting state unnecessarily.
					}
				}
			}
		}
	default:
		// Unhandled event type
	}

	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleCheckoutComplete fires after a successful Stripe payment.
// It reads the phone number from session metadata and sends a WhatsApp
// thank-you message with the receipt / invoice link.
func (h *StripeWebhookHandler) handleCheckoutComplete(ctx context.Context, session *stripe.CheckoutSession) {
	fmt.Println("[Stripe Webhook] inside handleCheckoutComplete!")
	if session == nil {
		fmt.Println("[Stripe Webhook] session is nil!")
		return
	}

	fmt.Printf("[Stripe Webhook] session ID: %s, Metadata: %+v\n", session.ID, session.Metadata)

	customerPhone := session.Metadata["customer_phone"]
	customerName := session.Metadata["customer_name"]
	studioIDStr := session.Metadata["studio_id"]

	if session.Metadata["is_upgrade"] == "true" {
		tier := session.Metadata["plan_tier"]
		id, err := uuid.Parse(studioIDStr)
		if err == nil {
			_ = h.svc.UpdatePayments(ctx, id, "", "", "", "", tier)
			fmt.Printf("[Stripe Webhook] Successfully upgraded studio %s to %s\n", studioIDStr, tier)
			
			// Cancel old subscriptions
			secretKey, _ := h.svc.GetPlatformSetting(ctx, "stripe_secret_key")
			if secretKey != "" {
				sc := &client.API{}
				sc.Init(secretKey, nil)
				if session.Customer != nil {
					params := &stripe.SubscriptionListParams{
						Customer: stripe.String(session.Customer.ID),
						Status:   stripe.String("active"),
					}
					iter := sc.Subscriptions.List(params)
					for iter.Next() {
						sub := iter.Subscription()
						// Don't cancel the newly created subscription!
						if session.Subscription != nil && sub.ID == session.Subscription.ID {
							continue
						}
						_, _ = sc.Subscriptions.Cancel(sub.ID, nil)
						fmt.Printf("[Stripe Webhook] Canceled old subscription %s for studio %s\n", sub.ID, studioIDStr)
					}
				}
			}
		}
		return
	}

	if customerPhone == "" || studioIDStr == "" {
		fmt.Printf("[Stripe Webhook] customerPhone or studioIDStr is empty! Phone: '%s', Studio: '%s'\n", customerPhone, studioIDStr)
		// No phone embedded — nothing to do for WhatsApp
		return
	}

	// Retrieve studio to get WhatsApp credentials and name
	studio, err := h.svc.repo.GetBySlug(ctx, studioIDStr)
	if err != nil || studio == nil {
		id, err2 := uuid.Parse(studioIDStr)
		if err2 == nil {
			studio, _ = h.svc.repo.GetByID(ctx, id)
		}
	}
	if studio == nil {
		return
	}

	receiptURL := ""
	if session.Invoice != nil && session.Invoice.ID != "" {
		// We only have the ID, fetch the full invoice to get the PDF URL using the Studio's key
		if studio.StripeSecretKey != "" {
			sc := &client.API{}
			sc.Init(studio.StripeSecretKey, nil)
			inv, errInv := sc.Invoices.Get(session.Invoice.ID, nil)
			if errInv == nil && inv != nil {
				if inv.HostedInvoiceURL != "" {
					receiptURL = inv.HostedInvoiceURL
				} else if inv.InvoicePDF != "" {
					receiptURL = inv.InvoicePDF
				}
			} else {
				fmt.Printf("[Stripe Webhook] Failed to fetch invoice %s with studio key: %v\n", session.Invoice.ID, errInv)
			}
		} else {
			fmt.Printf("[Stripe Webhook] Studio has no StripeSecretKey to fetch invoice %s\n", session.Invoice.ID)
		}
	}
	
	amountStr := ""
	if session.AmountTotal > 0 {
		amountStr = fmt.Sprintf("%.2f %s", float64(session.AmountTotal)/100.0, strings.ToUpper(string(session.Currency)))
	}

	name := customerName
	if name == "" {
		name = "there"
	}

	planIDStr := session.Metadata["plan_id"]
	isMembership := planIDStr != ""

	var message string
	if isMembership {
		message = fmt.Sprintf(
			"🎉 Hi %s! Welcome to *%s*!\n\n"+
				"Your membership subscription of *%s* was received successfully. We are excited to have you on board! 💪\n\n"+
				"📄 *Your Receipt:* %s\n\n"+
				"See you soon! — The %s Team",
			name, studio.Name, amountStr, receiptURL, studio.Name,
		)
	} else {
		message = fmt.Sprintf(
			"🎉 Hi %s! Thank you for booking your Trial at *%s*!\n\n"+
				"Your payment of *%s* was received successfully. We can't wait to see you! 💪\n\n"+
				"Your session is confirmed. Please arrive 10 minutes early.\n\n"+
				"📄 *Your Receipt:* %s\n\n"+
				"See you soon! — The %s Team",
			name, studio.Name, amountStr, receiptURL, studio.Name,
		)
	}

	cleanPhone := strings.ReplaceAll(strings.ReplaceAll(customerPhone, "+", ""), " ", "")

	// Instead of direct HTTP, enqueue it in the outbound_jobs table so the worker uses the studio's actual channel
	var convID string
	var leadID *string
	err = h.svc.repo.Pool().QueryRow(ctx, `
		SELECT c.id, c.lead_id FROM conversations c
		JOIN contact_identities ci ON c.contact_identity_id = ci.id
		WHERE c.studio_id = $1 AND ci.value = $2
		ORDER BY c.created_at DESC LIMIT 1
	`, studio.ID, cleanPhone).Scan(&convID, &leadID)

	if leadID != nil {
		if isMembership {
			monthlyFee := float64(session.AmountTotal) / 100.0

			var planName string
			_ = h.svc.repo.Pool().QueryRow(ctx, "SELECT plan_name FROM plans WHERE id = $1", planIDStr).Scan(&planName)

			var err error
			if planName != "" {
				_, err = h.svc.repo.Pool().Exec(ctx, `
					UPDATE leads 
					SET member_sold = true, status = 'member', monthly_fee = $1, fitness_plan = $2, updated_at = now() 
					WHERE id = $3
				`, monthlyFee, planName, *leadID)
			} else {
				_, err = h.svc.repo.Pool().Exec(ctx, `
					UPDATE leads 
					SET member_sold = true, status = 'member', monthly_fee = $1, updated_at = now() 
					WHERE id = $2
				`, monthlyFee, *leadID)
			}

			if err != nil {
				fmt.Printf("[Stripe Webhook] Failed to update lead status: %v\n", err)
			} else {
				fmt.Printf("[Stripe Webhook] Successfully updated lead status to member for %s\n", customerPhone)

				// Phase 5: Cancel any pending automated follow-ups since the lead became a member
				_, _ = h.svc.repo.Pool().Exec(ctx, `
					DELETE FROM outbound_jobs
					WHERE studio_id = $1 AND conversation_id IN (
						SELECT id FROM conversations WHERE lead_id = $2
					) AND source_kind = 'automation' AND status = 'pending'
				`, studio.ID, *leadID)
			}
		} else {
			_, updateErr := h.svc.repo.Pool().Exec(ctx, `
				UPDATE leads 
				SET trial_purchased = true, status = 'trial_booked', updated_at = now() 
				WHERE id = $1
			`, *leadID)
			if updateErr != nil {
				fmt.Printf("[Stripe Webhook] Failed to update lead status: %v\n", updateErr)
			} else {
				fmt.Printf("[Stripe Webhook] Successfully updated lead status to trial_booked for %s\n", customerPhone)
			}
		}
	}

	if err == nil && convID != "" {
		_, err = h.svc.repo.Pool().Exec(ctx, `
			INSERT INTO outbound_jobs (studio_id, conversation_id, source_kind, body, scheduled_for, next_attempt_at)
			VALUES ($1, $2, 'automation', $3, now(), now())
		`, studio.ID, convID, message)
		if err != nil {
			fmt.Printf("[Stripe Webhook] Failed to enqueue WhatsApp message: %v\n", err)
		} else {
			fmt.Printf("[Stripe Webhook] ✅ WhatsApp enqueued successfully for %s\n", customerPhone)
		}
	} else {
		fmt.Printf("[Stripe Webhook] Failed to find conversation for phone %s: %v\n", customerPhone, err)
	}
}

// Removed direct sendWhatsAppMessage in favor of outbound_jobs queue
