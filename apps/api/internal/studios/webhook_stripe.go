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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/projectx/api/internal/integrations/glofox"
	"github.com/projectx/api/internal/platform/billing"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/client"
	"github.com/stripe/stripe-go/v78/webhook"
)

// invoiceSubscriptionID extracts the subscription ID an invoice was
// generated for. stripe-go v78's typed Invoice.Subscription field only
// works for older Stripe API versions — this account is on a newer one
// where Stripe moved that reference off the invoice entirely, into each
// line item's lines.data[].parent.subscription_item_details.subscription.
// The typed field comes back nil for every invoice on this account, so this
// falls back to parsing that path from the raw event bytes.
func invoiceSubscriptionID(invoice *stripe.Invoice, rawBytes []byte) string {
	if invoice.Subscription != nil && invoice.Subscription.ID != "" {
		return invoice.Subscription.ID
	}
	var raw struct {
		Lines struct {
			Data []struct {
				Parent struct {
					SubscriptionItemDetails struct {
						Subscription string `json:"subscription"`
					} `json:"subscription_item_details"`
				} `json:"parent"`
			} `json:"data"`
		} `json:"lines"`
	}
	if err := json.Unmarshal(rawBytes, &raw); err != nil {
		return ""
	}
	for _, line := range raw.Lines.Data {
		if line.Parent.SubscriptionItemDetails.Subscription != "" {
			return line.Parent.SubscriptionItemDetails.Subscription
		}
	}
	return ""
}

// renderConfirmationTemplate substitutes the placeholders a studio can use in
// its custom trial/membership payment confirmation message.
func renderConfirmationTemplate(tmpl, leadFirstName, studioName, amount, receiptURL string) string {
	replacer := strings.NewReplacer(
		"{{lead_first_name}}", leadFirstName,
		"{{studio_name}}", studioName,
		"{{amount}}", amount,
		"{{receipt_url}}", receiptURL,
	)
	return replacer.Replace(tmpl)
}

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

// HandleInbound godoc
//
//	@Summary		Receive a Stripe webhook event
//	@Description	Verifies and processes an inbound Stripe webhook event. Not authenticated via cookies — Stripe signs the request body instead, verified against the Stripe-Signature header using the studio's or platform's webhook secret. Events are deduplicated by event ID before processing. Handles checkout.session.completed and payment_link.payment.completed (finalizes trial/membership checkout, sends a WhatsApp confirmation, syncs the lead to Glofox, and cancels superseded subscriptions on upgrades/plan changes), payment_intent.succeeded (the embedded Stripe Elements trial payment flow's equivalent of checkout completion), invoice.payment_failed (marks the studio past_due), and customer.subscription.updated/deleted (marks the studio canceled). Other event types are accepted but ignored.
//	@Tags			Stripe Webhooks
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string	false	"Studio ID, when the webhook is routed per-studio rather than to the shared platform endpoint"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid webhook signature"
//	@Failure		401			{object}	httpx.ErrorResponse	"no webhook secret configured to verify the signature"
//	@Failure		503			{object}	httpx.ErrorResponse	"failed to read request body"
//	@Router			/api/v1/webhooks/stripe [post]
//	@Router			/api/v1/webhooks/stripe/{studioId} [post]
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
	case "payment_intent.succeeded":
		// The embedded-Stripe-Elements trial payment flow (studios.
		// publicCreateTrialPaymentIntent) — unlike the hosted Checkout
		// Session flow above, side effects for this path only fire here.
		var pi stripe.PaymentIntent
		rawBytes := event.Data.Raw
		if len(rawBytes) == 0 {
			rawBytes, _ = json.Marshal(event.Data.Object)
		}
		if err := json.Unmarshal(rawBytes, &pi); err == nil {
			go h.handlePaymentIntentSucceeded(context.Background(), &pi)
		}
	case "invoice.paid":
		// Existing invoice handling (platform billing) — plus member-subscription
		// renewal tracking: bumps user_subscriptions.next_renewal_at, fires for
		// both the first billing period (right after checkout) and every
		// renewal after.
		var invoice stripe.Invoice
		rawBytes := event.Data.Raw
		if len(rawBytes) == 0 {
			rawBytes, _ = json.Marshal(event.Data.Object)
		}
		if err := json.Unmarshal(rawBytes, &invoice); err == nil {
			if subID := invoiceSubscriptionID(&invoice, rawBytes); subID != "" {
				go h.handleMemberInvoicePaid(context.Background(), studioIDStr, subID, &invoice)
			}
		}
	case "invoice.payment_failed":
		var invoice stripe.Invoice
		rawBytes := event.Data.Raw
		if len(rawBytes) == 0 {
			rawBytes, _ = json.Marshal(event.Data.Object)
		}
		if err := json.Unmarshal(rawBytes, &invoice); err == nil {
			if subID := invoiceSubscriptionID(&invoice, rawBytes); subID != "" {
				secretKey, _ := h.svc.GetPlatformSetting(context.Background(), "stripe_secret_key")
				if secretKey != "" {
					sc := &client.API{}
					sc.Init(secretKey, nil)
					sub, err := sc.Subscriptions.Get(subID, nil)
					if err == nil && sub.Metadata["studio_id"] != "" {
						id, err := uuid.Parse(sub.Metadata["studio_id"])
						if err == nil {
							// Set the studio tier to 'past_due'
							_ = h.svc.UpdatePayments(context.Background(), id, "", "", "", "", "past_due")
							slog.Info("stripe studio past_due", "studio_id", sub.Metadata["studio_id"])
						}
					}
				}
				// Member subscriptions (not studio-tier billing) also flag past_due here.
				go h.handleMemberInvoiceFailed(context.Background(), subID, &invoice)
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
			// Member subscriptions: only a real cancellation reverts the lead —
			// a superseded old subscription from a plan-change is expected to
			// cancel and must not revert a member who's now on a new plan.
			if sub.Status == "canceled" || sub.CancelAtPeriodEnd {
				go h.handleMemberSubscriptionCanceled(context.Background(), &sub)
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

	// Stripe Checkout collects the customer's real email at payment time —
	// capture it so it overwrites the synthetic "wa-<phone>@example.com"
	// placeholder set when the lead was first created via WhatsApp, and so
	// Glofox sync (further down) gets a real email instead of the fake one.
	customerEmail := ""
	if session.CustomerDetails != nil {
		customerEmail = strings.TrimSpace(session.CustomerDetails.Email)
	}

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
			// Subscription/membership — fetch hosted invoice URL. Note: this
			// invoice's own period_start/period_end are NOT a usable "next
			// renewal" signal — Stripe sets both equal to the invoice's
			// creation instant on a brand-new subscription's first invoice,
			// not creation+interval. The real next-charge date is fetched
			// from the Subscription object separately, below.
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
		if studio.MembershipConfirmationMessage != "" {
			message = renderConfirmationTemplate(studio.MembershipConfirmationMessage, name, studio.Name, amountStr, receiptURL)
		} else {
			message = fmt.Sprintf(
				"🎉 Hi %s! Welcome to *%s*!\n\n"+
					"Your membership subscription of *%s* was received successfully. We are excited to have you on board! 💪%s\n\n"+
					"See you soon! — The %s Team",
				name, studio.Name, amountStr, receiptLine, studio.Name,
			)
		}
	} else {
		if studio.TrialConfirmationMessage != "" {
			message = renderConfirmationTemplate(studio.TrialConfirmationMessage, name, studio.Name, amountStr, receiptURL)
		} else {
			message = fmt.Sprintf(
				"🎉 Hi %s! Thank you for booking your Trial at *%s*!\n\n"+
					"Your payment of *%s* was received successfully. We can't wait to see you! 💪\n\n"+
					"Your session is confirmed. Please arrive 10 minutes early.%s\n\n"+
					"See you soon! — The %s Team",
				name, studio.Name, amountStr, receiptLine, studio.Name,
			)
		}
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
		// Save the real email Stripe collected at checkout, overwriting the
		// synthetic WhatsApp placeholder — do this before the Glofox sync
		// calls below so they pick up the real address.
		if customerEmail != "" {
			if _, err := h.svc.repo.Pool().Exec(ctx, `
				UPDATE leads SET email = $1, updated_at = now() WHERE id = $2
			`, customerEmail, *leadID); err != nil {
				slog.Warn("stripe: failed to save customer email from checkout", "err", err, "lead_id", *leadID)
			}
		}

		if isMembership {
			var subID, custID string
			if session.Subscription != nil {
				subID = session.Subscription.ID
			}
			if session.Customer != nil {
				custID = session.Customer.ID
			}
			if _, err := h.applyMembershipConfirmed(ctx, studio, *leadID, planIDStr, session.AmountTotal, session.ID, subID, custID, session.Metadata["old_subscription_id"]); err != nil {
				slog.Warn("stripe lead status update failed", "err", err)
			} else {
				slog.Info("stripe lead status updated to member", "phone", customerPhone)
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
				h.svc.SyncLeadToGlofoxByID(ctx, *leadID, glofox.GlofoxStatusTrial, session.AmountTotal)
				h.svc.SyncLeadToMindbodyByID(ctx, *leadID, true, session.AmountTotal)

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

// handlePaymentIntentSucceeded fires for the embedded-Stripe-Elements trial
// payment flow (studios.publicCreateTrialPaymentIntent + the customer-facing
// trial payment page's confirmCardPayment) — the counterpart to
// handleCheckoutComplete's trial branch above, but for a PaymentIntent
// rather than a CheckoutSession, since that flow never creates a Checkout
// Session at all. Firing side effects here (not client-side after
// confirmCardPayment resolves) means a customer closing the tab right after
// a successful charge still gets their status update / Glofox sync /
// confirmation message — the client can't be trusted to always call back.
// handleMembershipOneTimePaymentSucceeded confirms a one_time plan bought
// through the embedded-Elements flow (publicCreatePlanPaymentIntent) — the
// non-recurring counterpart to handleFirstMembershipInvoice, triggered
// directly since a one-time PaymentIntent carries our metadata ourselves
// (no Subscription object involved to fetch it from).
func (h *StripeWebhookHandler) handleMembershipOneTimePaymentSucceeded(ctx context.Context, pi *stripe.PaymentIntent) {
	studioIDStr := pi.Metadata["studio_id"]
	leadID := pi.Metadata["lead_id"]
	planID := pi.Metadata["plan_id"]
	if studioIDStr == "" || leadID == "" || planID == "" {
		slog.Warn("stripe membership_onetime payment_intent.succeeded missing metadata", "pi_id", pi.ID)
		return
	}
	id, err := uuid.Parse(studioIDStr)
	if err != nil {
		return
	}
	studio, err := h.svc.GetByID(ctx, id)
	if err != nil || studio == nil {
		slog.Warn("stripe membership_onetime: studio not found", "studio_id", studioIDStr)
		return
	}

	result, err := h.applyMembershipConfirmed(ctx, studio, leadID, planID, pi.Amount, pi.ID, "", "", "")
	if err != nil {
		slog.Warn("stripe: failed to apply one-time membership", "err", err, "lead_id", leadID)
		return
	}
	slog.Info("stripe: one-time membership confirmed", "lead_id", leadID, "plan", result.PlanName)

	var convID string
	_ = h.svc.repo.Pool().QueryRow(ctx, `
		SELECT id FROM conversations WHERE lead_id = $1 ORDER BY created_at DESC LIMIT 1
	`, leadID).Scan(&convID)
	if convID == "" {
		return
	}
	var leadName string
	_ = h.svc.repo.Pool().QueryRow(ctx, "SELECT name FROM leads WHERE id = $1", leadID).Scan(&leadName)
	name := leadName
	if name == "" {
		name = "there"
	}
	receiptURL := ""
	if studio.StripeSecretKey != "" {
		sc := &client.API{}
		sc.Init(studio.StripeSecretKey, nil)
		if full, errPI := sc.PaymentIntents.Get(pi.ID, &stripe.PaymentIntentParams{
			Params: stripe.Params{Expand: stripe.StringSlice([]string{"latest_charge"})},
		}); errPI == nil && full != nil && full.LatestCharge != nil {
			receiptURL = full.LatestCharge.ReceiptURL
		}
	}
	receiptLine := ""
	if receiptURL != "" {
		receiptLine = fmt.Sprintf("\n\n📄 *Your Receipt:* %s", receiptURL)
	}

	amountStr := fmt.Sprintf("%.2f %s", float64(pi.Amount)/100.0, strings.ToUpper(string(pi.Currency)))
	var message string
	if studio.MembershipConfirmationMessage != "" {
		message = renderConfirmationTemplate(studio.MembershipConfirmationMessage, name, studio.Name, amountStr, receiptURL)
	} else {
		message = fmt.Sprintf(
			"🎉 Hi %s! Welcome to *%s*!\n\nYour payment of *%s* for the *%s* plan was received successfully. We are excited to have you on board! 💪%s\n\nSee you soon! — The %s Team",
			name, studio.Name, amountStr, result.PlanName, receiptLine, studio.Name,
		)
	}
	_, _ = h.svc.repo.Pool().Exec(ctx, `
		INSERT INTO outbound_jobs (studio_id, conversation_id, source_kind, body, scheduled_for, next_attempt_at)
		VALUES ($1, $2, 'automation', $3, now(), now())
	`, studio.ID, convID, message)
}

func (h *StripeWebhookHandler) handlePaymentIntentSucceeded(ctx context.Context, pi *stripe.PaymentIntent) {
	if pi == nil {
		return
	}
	if pi.Metadata["kind"] == "membership_onetime" {
		h.handleMembershipOneTimePaymentSucceeded(ctx, pi)
		return
	}
	if pi.Metadata["kind"] != "trial" {
		return // not ours — e.g. platform billing
	}
	studioIDStr := pi.Metadata["studio_id"]
	leadIDStr := pi.Metadata["lead_id"]
	if studioIDStr == "" || leadIDStr == "" {
		slog.Warn("stripe payment_intent.succeeded missing metadata", "pi_id", pi.ID)
		return
	}
	studioID, err := uuid.Parse(studioIDStr)
	if err != nil {
		return
	}
	studio, err := h.svc.repo.GetByID(ctx, studioID)
	if err != nil || studio == nil {
		slog.Warn("stripe payment_intent.succeeded: studio not found", "studio_id", studioIDStr)
		return
	}

	// Re-fetch with the charge expanded — the webhook payload itself doesn't
	// include billing details/receipt URL, same as handleCheckoutComplete's
	// approach for the trial branch there.
	customerEmail, receiptURL := "", ""
	if studio.StripeSecretKey != "" {
		sc := &client.API{}
		sc.Init(studio.StripeSecretKey, nil)
		full, errPI := sc.PaymentIntents.Get(pi.ID, &stripe.PaymentIntentParams{
			Params: stripe.Params{Expand: stripe.StringSlice([]string{"latest_charge"})},
		})
		if errPI == nil && full != nil && full.LatestCharge != nil {
			receiptURL = full.LatestCharge.ReceiptURL
			if full.LatestCharge.BillingDetails != nil {
				customerEmail = strings.TrimSpace(full.LatestCharge.BillingDetails.Email)
			}
		}
	}

	var leadName string
	_ = h.svc.repo.Pool().QueryRow(ctx, "SELECT name FROM leads WHERE id = $1", leadIDStr).Scan(&leadName)
	name := leadName
	if name == "" {
		name = "there"
	}

	if customerEmail != "" {
		if _, err := h.svc.repo.Pool().Exec(ctx, `
			UPDATE leads SET email = $1, updated_at = now() WHERE id = $2
		`, customerEmail, leadIDStr); err != nil {
			slog.Warn("stripe: failed to save customer email from trial payment", "err", err, "lead_id", leadIDStr)
		}
	}

	_, updateErr := h.svc.repo.Pool().Exec(ctx, `
		UPDATE leads SET trial_purchased = true, status = 'trial_booked', updated_at = now()
		WHERE id = $1
	`, leadIDStr)
	if updateErr != nil {
		slog.Warn("stripe trial payment: lead status update failed", "err", updateErr)
		return
	}
	slog.Info("stripe trial payment: lead status updated to trial_booked", "lead_id", leadIDStr)
	h.svc.SyncLeadToGlofoxByID(ctx, leadIDStr, glofox.GlofoxStatusTrial, pi.Amount)
	h.svc.SyncLeadToMindbodyByID(ctx, leadIDStr, true, pi.Amount)

	var convID string
	_ = h.svc.repo.Pool().QueryRow(ctx, `
		SELECT id FROM conversations WHERE lead_id = $1 ORDER BY created_at DESC LIMIT 1
	`, leadIDStr).Scan(&convID)
	if convID == "" {
		slog.Warn("stripe trial payment: no conversation found for lead — confirmation message dropped", "lead_id", leadIDStr)
		return
	}

	// Schedule a 2-day post-trial follow-up to push membership — same as
	// handleCheckoutComplete's trial branch.
	postTrialMsg := fmt.Sprintf(
		"Hi %s! 👋 How was your trial at *%s*? We hope you loved it!\n\n"+
			"Ready to make it official and become a member? Reply *2* to choose a membership plan and keep the momentum going! 💪",
		name, studio.Name,
	)
	_, _ = h.svc.repo.Pool().Exec(ctx, `
		INSERT INTO outbound_jobs (studio_id, conversation_id, source_kind, body, scheduled_for, next_attempt_at)
		VALUES ($1, $2, 'automation', $3, now() + interval '2 days', now() + interval '2 days')
	`, studio.ID, convID, postTrialMsg)

	amountStr := fmt.Sprintf("%.2f %s", float64(pi.Amount)/100.0, strings.ToUpper(string(pi.Currency)))
	receiptLine := ""
	if receiptURL != "" {
		receiptLine = fmt.Sprintf("\n\n📄 *Your Receipt:* %s", receiptURL)
	}
	var message string
	if studio.TrialConfirmationMessage != "" {
		message = renderConfirmationTemplate(studio.TrialConfirmationMessage, name, studio.Name, amountStr, receiptURL)
	} else {
		message = fmt.Sprintf(
			"🎉 Hi %s! Thank you for booking your Trial at *%s*!\n\n"+
				"Your payment of *%s* was received successfully. We can't wait to see you! 💪\n\n"+
				"Your session is confirmed. Please arrive 10 minutes early.%s\n\n"+
				"See you soon! — The %s Team",
			name, studio.Name, amountStr, receiptLine, studio.Name,
		)
	}
	if _, err := h.svc.repo.Pool().Exec(ctx, `
		INSERT INTO outbound_jobs (studio_id, conversation_id, source_kind, body, scheduled_for, next_attempt_at)
		VALUES ($1, $2, 'automation', $3, now(), now())
	`, studio.ID, convID, message); err != nil {
		slog.Warn("stripe trial payment: whatsapp enqueue failed", "err", err)
	} else {
		slog.Info("stripe trial payment: whatsapp confirmation enqueued", "lead_id", leadIDStr)
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

// handleMemberInvoicePaid bumps a member subscription's next renewal date
// when Stripe successfully charges a recurring invoice for it — fires both
// for the first billing period (right after checkout) and every renewal
// after. If no user_subscriptions row exists yet for this subscription, it's
// either not one of ours, or it's a brand-new subscription from the
// embedded-Elements plan-purchase flow (publicCreatePlanPaymentIntent) whose
// first confirmation arrives here rather than via handleCheckoutComplete —
// see handleFirstMembershipInvoice.
func (h *StripeWebhookHandler) handleMemberInvoicePaid(ctx context.Context, studioIDStr, subID string, invoice *stripe.Invoice) {
	var studioSecretKey string
	err := h.svc.repo.Pool().QueryRow(ctx, `
		SELECT s.stripe_secret_key FROM user_subscriptions us
		JOIN studios s ON s.id = us.studio_id
		WHERE us.stripe_subscription_id = $1
	`, subID).Scan(&studioSecretKey)
	if errors.Is(err, pgx.ErrNoRows) {
		h.handleFirstMembershipInvoice(ctx, studioIDStr, subID, invoice)
		return
	}
	if err != nil {
		slog.Warn("stripe: failed to look up studio for member invoice", "err", err, "subscription_id", subID)
		return
	}

	// The invoice's own period_end isn't a reliable "next renewal" signal —
	// Stripe sets it equal to the invoice's creation instant on a brand-new
	// subscription's first invoice. Fetch the Subscription object itself,
	// whose current_period_end is the actual next-charge date.
	var nextRenewalAt *time.Time
	if studioSecretKey != "" {
		sc := &client.API{}
		sc.Init(studioSecretKey, nil)
		if sub, errSub := sc.Subscriptions.Get(subID, nil); errSub == nil && sub != nil && sub.CurrentPeriodEnd > 0 {
			t := time.Unix(sub.CurrentPeriodEnd, 0).UTC()
			nextRenewalAt = &t
		} else if errSub != nil {
			slog.Warn("stripe: failed to fetch subscription for renewal date", "err", errSub, "subscription_id", subID)
		}
	}
	if nextRenewalAt == nil {
		if invoice.PeriodEnd == 0 {
			return
		}
		t := time.Unix(invoice.PeriodEnd, 0).UTC()
		nextRenewalAt = &t
	}

	tag, err := h.svc.repo.Pool().Exec(ctx, `
		UPDATE user_subscriptions
		SET next_renewal_at = $1, payment_status = 'paid', updated_at = now()
		WHERE stripe_subscription_id = $2
	`, nextRenewalAt, subID)
	if err != nil {
		slog.Warn("stripe: failed to update member subscription renewal", "err", err, "subscription_id", subID)
		return
	}
	if tag.RowsAffected() > 0 {
		slog.Info("stripe: member subscription renewed", "subscription_id", subID)
	}
}

// handleFirstMembershipInvoice checks whether a subscription this webhook
// hasn't recorded yet is a brand-new membership purchase from the embedded-
// Elements flow (publicCreatePlanPaymentIntent — the studio's own branded
// page, not a hosted Checkout Session) and, if so, applies the same
// bookkeeping handleCheckoutComplete does for that flow. studioIDStr comes
// from the webhook URL itself (see HandleInbound) — the invoice payload
// doesn't carry our metadata directly, only the Subscription object does,
// and fetching that needs a studio's secret key to call Stripe with.
func (h *StripeWebhookHandler) handleFirstMembershipInvoice(ctx context.Context, studioIDStr, subID string, invoice *stripe.Invoice) {
	if studioIDStr == "" {
		return
	}
	id, err := uuid.Parse(studioIDStr)
	if err != nil {
		return
	}
	studio, err := h.svc.GetByID(ctx, id)
	if err != nil || studio == nil || studio.StripeSecretKey == "" {
		return
	}

	sc := &client.API{}
	sc.Init(studio.StripeSecretKey, nil)
	sub, errSub := sc.Subscriptions.Get(subID, nil)
	if errSub != nil || sub == nil || sub.Metadata["kind"] != "membership" {
		return // not one of ours — e.g. studio-tier platform billing
	}
	leadID := sub.Metadata["lead_id"]
	planID := sub.Metadata["plan_id"]
	if leadID == "" || planID == "" {
		slog.Warn("stripe: membership subscription missing lead/plan metadata", "subscription_id", subID)
		return
	}
	var custID string
	if sub.Customer != nil {
		custID = sub.Customer.ID
	}

	result, err := h.applyMembershipConfirmed(ctx, studio, leadID, planID, invoice.AmountPaid, invoice.ID, subID, custID, "")
	if err != nil {
		slog.Warn("stripe: failed to apply new embedded-flow membership", "err", err, "lead_id", leadID)
		return
	}
	slog.Info("stripe: embedded-flow membership confirmed", "lead_id", leadID, "plan", result.PlanName)

	var convID string
	_ = h.svc.repo.Pool().QueryRow(ctx, `
		SELECT id FROM conversations WHERE lead_id = $1 ORDER BY created_at DESC LIMIT 1
	`, leadID).Scan(&convID)
	if convID == "" {
		return
	}
	var leadName string
	_ = h.svc.repo.Pool().QueryRow(ctx, "SELECT name FROM leads WHERE id = $1", leadID).Scan(&leadName)
	name := leadName
	if name == "" {
		name = "there"
	}
	receiptURL := ""
	if inv, errInv := sc.Invoices.Get(invoice.ID, nil); errInv == nil && inv != nil {
		if inv.HostedInvoiceURL != "" {
			receiptURL = inv.HostedInvoiceURL
		} else if inv.InvoicePDF != "" {
			receiptURL = inv.InvoicePDF
		}
	}
	receiptLine := ""
	if receiptURL != "" {
		receiptLine = fmt.Sprintf("\n\n📄 *Your Receipt:* %s", receiptURL)
	}

	amountStr := fmt.Sprintf("%.2f %s", float64(invoice.AmountPaid)/100.0, strings.ToUpper(string(invoice.Currency)))
	var message string
	if studio.MembershipConfirmationMessage != "" {
		message = renderConfirmationTemplate(studio.MembershipConfirmationMessage, name, studio.Name, amountStr, receiptURL)
	} else {
		message = fmt.Sprintf(
			"🎉 Hi %s! Welcome to *%s*!\n\nYour membership subscription of *%s* to the *%s* plan was received successfully. We are excited to have you on board! 💪%s\n\nSee you soon! — The %s Team",
			name, studio.Name, amountStr, result.PlanName, receiptLine, studio.Name,
		)
	}
	_, _ = h.svc.repo.Pool().Exec(ctx, `
		INSERT INTO outbound_jobs (studio_id, conversation_id, source_kind, body, scheduled_for, next_attempt_at)
		VALUES ($1, $2, 'automation', $3, now(), now())
	`, studio.ID, convID, message)
}

// handleMemberInvoiceFailed flags a member subscription past_due when a
// recurring charge fails. Doesn't touch the lead — Stripe keeps retrying per
// the account's dunning settings before eventually canceling the
// subscription, which handleMemberSubscriptionCanceled reacts to.
func (h *StripeWebhookHandler) handleMemberInvoiceFailed(ctx context.Context, subID string, invoice *stripe.Invoice) {
	tag, err := h.svc.repo.Pool().Exec(ctx, `
		UPDATE user_subscriptions
		SET payment_status = 'failed', subscription_status = 'past_due', updated_at = now()
		WHERE stripe_subscription_id = $1
	`, subID)
	if err != nil {
		slog.Warn("stripe: failed to flag member subscription past_due", "err", err, "subscription_id", subID)
		return
	}
	if tag.RowsAffected() > 0 {
		slog.Info("stripe: member subscription past_due", "subscription_id", subID)
	}
}

// handleMemberSubscriptionCanceled marks a member subscription canceled and,
// only if it's still the lead's CURRENT subscription, reverts the lead off
// membership. The guard on leads.stripe_subscription_id matters: when a
// member changes plans, the OLD subscription gets canceled deliberately
// (see handleCheckoutComplete) and is already marked 'superseded' there, not
// 'active' — so this update's WHERE clause won't touch it, and even if it
// did, the leads UPDATE below is scoped to still require a stripe_subscription_id
// match, which no longer holds once the lead has moved to the new subscription.
func (h *StripeWebhookHandler) handleMemberSubscriptionCanceled(ctx context.Context, sub *stripe.Subscription) {
	var leadID string
	err := h.svc.repo.Pool().QueryRow(ctx, `
		UPDATE user_subscriptions
		SET subscription_status = 'canceled', canceled_at = now(), updated_at = now()
		WHERE stripe_subscription_id = $1 AND subscription_status NOT IN ('canceled', 'superseded')
		RETURNING lead_id
	`, sub.ID).Scan(&leadID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("stripe: failed to mark member subscription canceled", "err", err, "subscription_id", sub.ID)
		}
		return
	}
	slog.Info("stripe: member subscription canceled", "subscription_id", sub.ID, "lead_id", leadID)

	if _, err := h.svc.repo.Pool().Exec(ctx, `
		UPDATE leads
		SET member_sold = false, status = 'dropped', updated_at = now()
		WHERE id = $1 AND stripe_subscription_id = $2
	`, leadID, sub.ID); err != nil {
		slog.Warn("stripe: failed to revert lead after subscription cancellation", "err", err, "lead_id", leadID)
	}
}

// membershipConfirmed carries what a caller needs to compose its own
// confirmation message after applyMembershipConfirmed's bookkeeping.
type membershipConfirmed struct {
	PlanName string
}

// applyMembershipConfirmed marks a lead a member and records the real
// subscription (cadence/renewal/lifecycle) — the single place every path
// that confirms "this lead just paid for this plan" converges, whether
// that's the hosted-Checkout-Session flow (handleCheckoutComplete) or the
// embedded-Elements flow on the studio's own branded page (a Subscription's
// invoice.paid for a recurring plan, or payment_intent.succeeded for a
// one-time plan — see publicCreatePlanPaymentIntent).
func (h *StripeWebhookHandler) applyMembershipConfirmed(ctx context.Context, studio *Studio, leadID, planIDStr string, amountPaid int64, paymentID, subID, custID, oldSubscriptionID string) (membershipConfirmed, error) {
	var planName, planBillingCycle, planBillingInterval string
	var planBillingIntervalCount int
	_ = h.svc.repo.Pool().QueryRow(ctx, "SELECT plan_name, billing_cycle, billing_interval, billing_interval_count FROM plans WHERE id = $1", planIDStr).
		Scan(&planName, &planBillingCycle, &planBillingInterval, &planBillingIntervalCount)

	monthlyFee := float64(amountPaid) / 100.0
	var err error
	if planName != "" {
		_, err = h.svc.repo.Pool().Exec(ctx, `
			UPDATE leads
			SET member_sold = true, status = 'member', monthly_fee = $1, fitness_plan = $2,
			    stripe_subscription_id = $3, stripe_customer_id = $4, updated_at = now()
			WHERE id = $5
		`, monthlyFee, planName, subID, custID, leadID)
	} else {
		_, err = h.svc.repo.Pool().Exec(ctx, `
			UPDATE leads
			SET member_sold = true, status = 'member', monthly_fee = $1,
			    stripe_subscription_id = $2, stripe_customer_id = $3, updated_at = now()
			WHERE id = $4
		`, monthlyFee, subID, custID, leadID)
	}
	if err != nil {
		return membershipConfirmed{}, fmt.Errorf("update lead: %w", err)
	}

	h.svc.SyncLeadToGlofoxByID(ctx, leadID, glofox.GlofoxStatusMember, amountPaid)
	h.svc.SyncLeadToMindbodyByID(ctx, leadID, false, amountPaid)

	// Record the real subscription (cadence, renewal, lifecycle) — the leads
	// columns above are just the display summary the pipeline/lead-detail UI
	// already reads.
	interval, intervalCount, recurring := billing.Resolve(planBillingCycle, planBillingInterval, int64(planBillingIntervalCount))
	subStatus := "completed"
	if recurring {
		subStatus = "active"
	}
	// Fetched fresh (not parsed from the webhook body) so the first
	// user_subscriptions row can carry a correct next_renewal_at
	// immediately — waiting for a later invoice.paid event to set it
	// instead is a race: Stripe can (and does) deliver invoice.paid before
	// or concurrently with this handler finishing its own INSERT, so that
	// event's UPDATE can silently match zero rows.
	var nextRenewalAt *time.Time
	if recurring && subID != "" && studio.StripeSecretKey != "" {
		sc := &client.API{}
		sc.Init(studio.StripeSecretKey, nil)
		if sub, errSub := sc.Subscriptions.Get(subID, nil); errSub == nil && sub != nil && sub.CurrentPeriodEnd > 0 {
			t := time.Unix(sub.CurrentPeriodEnd, 0).UTC()
			nextRenewalAt = &t
		} else if errSub != nil {
			slog.Warn("stripe: failed to fetch subscription for renewal date", "err", errSub, "subscription_id", subID)
		}
	}
	if _, subErr := h.svc.repo.Pool().Exec(ctx, `
		INSERT INTO user_subscriptions
			(studio_id, lead_id, plan_id, plan_name, amount_paid, currency, payment_id,
			 payment_status, subscription_status, stripe_subscription_id, stripe_customer_id,
			 billing_interval, billing_interval_count, next_renewal_at)
		VALUES ($1, $2, $3, $4, $5, 'SGD', $6, 'paid', $7, $8, $9, $10, $11, $12)
	`, studio.ID, leadID, planIDStr, planName, amountPaid, paymentID, subStatus, subID, custID, interval, intervalCount, nextRenewalAt); subErr != nil {
		slog.Warn("stripe: failed to record member subscription", "err", subErr, "lead_id", leadID)
	}

	// Cancel any pending automated follow-ups since the lead became a member.
	_, _ = h.svc.repo.Pool().Exec(ctx, `
		DELETE FROM outbound_jobs
		WHERE studio_id = $1 AND conversation_id IN (
			SELECT id FROM conversations WHERE lead_id = $2
		) AND source_kind = 'automation' AND status = 'pending'
	`, studio.ID, leadID)

	// Plan change: this purchase replaced an existing membership, so cancel
	// the old subscription now that the new one is confirmed — otherwise
	// the customer stays billed on both.
	if oldSubscriptionID != "" && oldSubscriptionID != subID && studio.StripeSecretKey != "" {
		sc := &client.API{}
		sc.Init(studio.StripeSecretKey, nil)
		if _, cancelErr := sc.Subscriptions.Cancel(oldSubscriptionID, nil); cancelErr != nil {
			slog.Warn("stripe: failed to cancel old subscription after plan change", "old_sub_id", oldSubscriptionID, "err", cancelErr)
		} else {
			slog.Info("stripe: canceled old subscription after plan change", "old_sub_id", oldSubscriptionID, "new_sub_id", subID)
			if _, err := h.svc.repo.Pool().Exec(ctx, `
				UPDATE user_subscriptions
				SET subscription_status = 'superseded', canceled_at = now(), updated_at = now()
				WHERE stripe_subscription_id = $1
			`, oldSubscriptionID); err != nil {
				slog.Warn("stripe: failed to mark old member subscription superseded", "err", err, "old_sub_id", oldSubscriptionID)
			}
		}
	}

	return membershipConfirmed{PlanName: planName}, nil
}

// Removed direct sendWhatsAppMessage in favor of outbound_jobs queue
