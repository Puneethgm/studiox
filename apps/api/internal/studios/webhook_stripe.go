package studios

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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
	// Fall back to global env secret
	if endpointSecret == "" {
		endpointSecret = h.webhookSecret
	}
	// Last resort: extract studio_id from unverified payload and look up the studio's secret.
	// Safe because we still verify the signature with whatever secret we find.
	if endpointSecret == "" {
		var raw struct {
			Data struct {
				Object struct {
					Metadata map[string]string `json:"metadata"`
				} `json:"object"`
			} `json:"data"`
		}
		if json.Unmarshal(payload, &raw) == nil {
			if sid := raw.Data.Object.Metadata["studio_id"]; sid != "" {
				if id, err := uuid.Parse(sid); err == nil {
					studio, err := h.svc.GetByID(r.Context(), id)
					if err == nil && studio != nil && studio.StripeWebhookSecret != "" {
						endpointSecret = studio.StripeWebhookSecret
					}
				}
			}
		}
	}

	if endpointSecret == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "missing_webhook_secret", "Stripe webhook secret is required")
		return
	}

	signatureHeader := r.Header.Get("Stripe-Signature")
	var event stripe.Event
	// The connected Stripe account's API version can be newer than the one
	// this vendored stripe-go release was built against — that alone made
	// ConstructEvent reject every event as a signature failure, even with
	// the correct secret. We only re-deserialize event.Data.Raw into our own
	// structs below, so a version mismatch here doesn't affect correctness.
	event, err = webhook.ConstructEventWithOptions(payload, signatureHeader, endpointSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_signature", "Error verifying webhook signature")
		return
	}

	// Stripe redelivers events at-least-once (retries, and occasionally even
	// after a 200). Dedup by event ID so a redelivery can't enqueue a second
	// WhatsApp message / re-apply the same lead update.
	tag, dedupErr := h.svc.repo.Pool().Exec(r.Context(), `
		INSERT INTO processed_stripe_events (event_id) VALUES ($1)
		ON CONFLICT (event_id) DO NOTHING
	`, event.ID)
	if dedupErr == nil && tag.RowsAffected() == 0 {
		slog.Info("stripe event already processed, skipping", "event_id", event.ID, "type", event.Type)
		httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if dedupErr != nil {
		slog.Warn("stripe event dedup check failed, processing anyway", "event_id", event.ID, "err", dedupErr)
	}

	// Handle the verified event
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
							slog.Info("stripe studio past_due", "studio_id", sub.Metadata["studio_id"])
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
						slog.Info("stripe studio canceled", "studio_id", sub.Metadata["studio_id"])
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
	slog.Debug("stripe checkout complete handler started")
	if session == nil {
		slog.Warn("stripe checkout complete: session is nil")
		return
	}

	slog.Debug("stripe checkout session", "session_id", session.ID)

	customerPhone := session.Metadata["customer_phone"]
	customerName := session.Metadata["customer_name"]
	studioIDStr := session.Metadata["studio_id"]

	if session.Metadata["is_upgrade"] == "true" {
		tier := session.Metadata["plan_tier"]
		id, err := uuid.Parse(studioIDStr)
		if err == nil {
			_ = h.svc.UpdatePayments(ctx, id, "", "", "", "", tier)
			slog.Info("stripe studio upgraded", "studio_id", studioIDStr, "tier", tier)

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
						slog.Info("stripe old subscription canceled", "sub_id", sub.ID, "studio_id", studioIDStr)
					}
				}
			}
		}
		return
	}

	if customerPhone == "" || studioIDStr == "" {
		slog.Warn("stripe checkout missing metadata")
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
	if studio.StripeSecretKey != "" {
		sc := &client.API{}
		sc.Init(studio.StripeSecretKey, nil)
		if session.Invoice != nil && session.Invoice.ID != "" {
			// Subscription/membership — fetch hosted invoice URL
			inv, errInv := sc.Invoices.Get(session.Invoice.ID, nil)
			if errInv == nil && inv != nil {
				if inv.HostedInvoiceURL != "" {
					receiptURL = inv.HostedInvoiceURL
				} else if inv.InvoicePDF != "" {
					receiptURL = inv.InvoicePDF
				}
			} else {
				slog.Warn("stripe invoice fetch failed", "invoice_id", session.Invoice.ID, "err", errInv)
			}
		} else if session.PaymentIntent != nil && session.PaymentIntent.ID != "" {
			// One-time payment (trial) — get receipt URL from latest charge
			pi, errPI := sc.PaymentIntents.Get(session.PaymentIntent.ID, &stripe.PaymentIntentParams{
				Params: stripe.Params{Expand: stripe.StringSlice([]string{"latest_charge"})},
			})
			if errPI == nil && pi != nil && pi.LatestCharge != nil {
				receiptURL = pi.LatestCharge.ReceiptURL
			}
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

	receiptLine := ""
	if receiptURL != "" {
		receiptLine = fmt.Sprintf("\n\n📄 *Your Receipt:* %s", receiptURL)
	}

	var message string
	if isMembership {
		message = fmt.Sprintf(
			"🎉 Hi %s! Welcome to *%s*!\n\n"+
				"Your membership subscription of *%s* was received successfully. We are excited to have you on board! 💪%s\n\n"+
				"See you soon! — The %s Team",
			name, studio.Name, amountStr, receiptLine, studio.Name,
		)
	} else {
		message = fmt.Sprintf(
			"🎉 Hi %s! Thank you for booking your Trial at *%s*!\n\n"+
				"Your payment of *%s* was received successfully. We can't wait to see you! 💪\n\n"+
				"Your session is confirmed. Please arrive 10 minutes early.%s\n\n"+
				"See you soon! — The %s Team",
			name, studio.Name, amountStr, receiptLine, studio.Name,
		)
	}

	cleanPhone := strings.ReplaceAll(strings.ReplaceAll(customerPhone, "+", ""), " ", "")

	// Instead of direct HTTP, enqueue it in the outbound_jobs table so the worker uses the studio's actual channel.
	// ci.value isn't always bare digits — Meta WhatsApp stores it that way, but
	// WhatsApp Web (QR) stores a full JID ("<digits>@c.us" / "...@lid"), so an
	// exact match against cleanPhone silently missed every WhatsApp Web
	// contact (this lookup would just return no rows — no error, no message,
	// no lead update, nothing). Normalize to digits-only on both sides,
	// matching the same pattern HandleInboundWAWeb already uses for its own
	// lead-by-phone lookup.
	var convID string
	var leadID *string
	err = h.svc.repo.Pool().QueryRow(ctx, `
		SELECT c.id, c.lead_id FROM conversations c
		JOIN contact_identities ci ON c.contact_identity_id = ci.id
		WHERE c.studio_id = $1 AND regexp_replace(ci.value, '\D', '', 'g') = $2
		ORDER BY c.created_at DESC LIMIT 1
	`, studio.ID, cleanPhone).Scan(&convID, &leadID)

	if errors.Is(err, pgx.ErrNoRows) {
		// First-ever contact for this phone (e.g. paid via a checkout link
		// before ever messaging the studio's WhatsApp) — no conversation
		// exists yet to enqueue the confirmation into. Create one against
		// the studio's active WhatsApp channel so the message isn't dropped.
		convID, leadID, err = h.createConversationForCheckout(ctx, studio.ID, cleanPhone, customerName)
		if err != nil {
			slog.Warn("stripe: could not create conversation for checkout", "phone", customerPhone, "err", err)
		}
	}

	if leadID != nil {
		if isMembership {
			monthlyFee := float64(session.AmountTotal) / 100.0

			var planName string
			_ = h.svc.repo.Pool().QueryRow(ctx, "SELECT plan_name FROM plans WHERE id = $1", planIDStr).Scan(&planName)

			var subID, custID string
			if session.Subscription != nil {
				subID = session.Subscription.ID
			}
			if session.Customer != nil {
				custID = session.Customer.ID
			}

			var err error
			if planName != "" {
				_, err = h.svc.repo.Pool().Exec(ctx, `
					UPDATE leads
					SET member_sold = true, status = 'member', monthly_fee = $1, fitness_plan = $2,
					    stripe_subscription_id = $3, stripe_customer_id = $4, updated_at = now()
					WHERE id = $5
				`, monthlyFee, planName, subID, custID, *leadID)
			} else {
				_, err = h.svc.repo.Pool().Exec(ctx, `
					UPDATE leads
					SET member_sold = true, status = 'member', monthly_fee = $1,
					    stripe_subscription_id = $2, stripe_customer_id = $3, updated_at = now()
					WHERE id = $4
				`, monthlyFee, subID, custID, *leadID)
			}

			if err != nil {
				slog.Warn("stripe lead status update failed", "err", err)
			} else {
				slog.Info("stripe lead status updated to member", "phone", customerPhone)

				// Phase 5: Cancel any pending automated follow-ups since the lead became a member
				_, _ = h.svc.repo.Pool().Exec(ctx, `
					DELETE FROM outbound_jobs
					WHERE studio_id = $1 AND conversation_id IN (
						SELECT id FROM conversations WHERE lead_id = $2
					) AND source_kind = 'automation' AND status = 'pending'
				`, studio.ID, *leadID)

				// Plan change: this checkout replaced an existing membership, so
				// cancel the old subscription now that the new one is confirmed —
				// otherwise the customer stays billed on both.
				if oldSubID := session.Metadata["old_subscription_id"]; oldSubID != "" && oldSubID != subID && studio.StripeSecretKey != "" {
					sc := &client.API{}
					sc.Init(studio.StripeSecretKey, nil)
					if _, cancelErr := sc.Subscriptions.Cancel(oldSubID, nil); cancelErr != nil {
						slog.Warn("stripe: failed to cancel old subscription after plan change", "old_sub_id", oldSubID, "err", cancelErr)
					} else {
						slog.Info("stripe: canceled old subscription after plan change", "old_sub_id", oldSubID, "new_sub_id", subID)
					}
				}
			}
		} else {
			_, updateErr := h.svc.repo.Pool().Exec(ctx, `
				UPDATE leads
				SET trial_purchased = true, status = 'trial_booked', updated_at = now()
				WHERE id = $1
			`, *leadID)
			if updateErr != nil {
				slog.Warn("stripe lead status update failed", "err", updateErr)
			} else {
				slog.Info("stripe lead status updated to trial_booked", "phone", customerPhone)

				// Schedule a 2-day post-trial follow-up to push membership.
				// This fires after the trial session and nudges the lead to join.
				if convID != "" {
					postTrialMsg := fmt.Sprintf(
						"Hi %s! 👋 How was your trial at *%s*? We hope you loved it!\n\n"+
							"Ready to make it official and become a member? Reply *2* to choose a membership plan and keep the momentum going! 💪",
						name, studio.Name,
					)
					_, _ = h.svc.repo.Pool().Exec(ctx, `
						INSERT INTO outbound_jobs (studio_id, conversation_id, source_kind, body, scheduled_for, next_attempt_at)
						VALUES ($1, $2, 'automation', $3, now() + interval '2 days', now() + interval '2 days')
					`, studio.ID, convID, postTrialMsg)
				}
			}
		}
	}

	if err == nil && convID != "" {
		_, err = h.svc.repo.Pool().Exec(ctx, `
			INSERT INTO outbound_jobs (studio_id, conversation_id, source_kind, body, scheduled_for, next_attempt_at)
			VALUES ($1, $2, 'automation', $3, now(), now())
		`, studio.ID, convID, message)
		if err != nil {
			slog.Warn("stripe whatsapp enqueue failed", "err", err)
		} else {
			slog.Info("stripe whatsapp enqueued", "phone", customerPhone)
		}
	} else {
		slog.Warn("stripe conversation not found", "phone", customerPhone, "err", err)
	}
}

// createConversationForCheckout creates a contact_identity + conversation for
// a phone number that has never messaged the studio before (e.g. paid via a
// checkout link before ever WhatsApp-ing in), so the post-payment
// confirmation isn't silently dropped for lack of somewhere to enqueue it.
// Returns the new conversation ID and, if an existing lead matches the
// phone, its ID.
func (h *StripeWebhookHandler) createConversationForCheckout(ctx context.Context, studioID uuid.UUID, cleanPhone, displayName string) (string, *string, error) {
	pool := h.svc.repo.Pool()

	var channelID, channelKind string
	if err := pool.QueryRow(ctx, `
		SELECT id, kind FROM channel_accounts
		WHERE studio_id = $1 AND status = 'active' AND kind IN ('whatsapp_web', 'whatsapp_meta')
		ORDER BY CASE kind WHEN 'whatsapp_web' THEN 0 ELSE 1 END, connected_at DESC
		LIMIT 1
	`, studioID).Scan(&channelID, &channelKind); err != nil {
		return "", nil, fmt.Errorf("no active whatsapp channel: %w", err)
	}

	if displayName == "" {
		displayName = cleanPhone
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var identityID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO contact_identities (studio_id, kind, value, display_name)
		VALUES ($1, 'phone', $2, $3)
		ON CONFLICT (studio_id, kind, value) DO UPDATE SET updated_at = now()
		RETURNING id
	`, studioID, cleanPhone, displayName).Scan(&identityID); err != nil {
		return "", nil, fmt.Errorf("upsert identity: %w", err)
	}

	var leadID *string
	_ = tx.QueryRow(ctx, `
		SELECT id FROM leads
		WHERE studio_id = $1 AND regexp_replace(phone, '\D', '', 'g') = $2
		ORDER BY created_at DESC LIMIT 1
	`, studioID, cleanPhone).Scan(&leadID)

	if leadID != nil {
		if _, err := tx.Exec(ctx, `UPDATE contact_identities SET lead_id = $2 WHERE id = $1`, identityID, *leadID); err != nil {
			return "", nil, fmt.Errorf("link identity to lead: %w", err)
		}
	}

	var convID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO conversations (studio_id, channel_account_id, contact_identity_id, external_thread_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (channel_account_id, external_thread_id) DO UPDATE SET updated_at = now()
		RETURNING id
	`, studioID, channelID, identityID, cleanPhone).Scan(&convID); err != nil {
		return "", nil, fmt.Errorf("upsert conversation: %w", err)
	}

	if leadID != nil {
		if _, err := tx.Exec(ctx, `UPDATE conversations SET lead_id = $2 WHERE id = $1`, convID, *leadID); err != nil {
			return "", nil, fmt.Errorf("link conversation to lead: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", nil, fmt.Errorf("commit: %w", err)
	}

	return convID, leadID, nil
}

// Removed direct sendWhatsAppMessage in favor of outbound_jobs queue
