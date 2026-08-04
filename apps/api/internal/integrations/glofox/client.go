package glofox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

const baseURL = "https://gf-api.aws.glofox.com/prod"

// Client calls the Glofox REST API on behalf of the platform.
// Auth requires three values per request:
//   - x-glofox-api-token  — the API Token from the partner portal
//   - x-api-key           — the API Key from the partner portal
//   - x-glofox-branch-id  — the Glofox branch/location _id
type Client struct {
	apiKey   string
	apiToken string
	branchID string
	http     *http.Client
}

// New returns a configured client. Returns nil when any required credential is missing.
func New(apiKey, apiToken, branchID string) *Client {
	if apiKey == "" || apiToken == "" || branchID == "" {
		return nil
	}
	return &Client{
		apiKey:   apiKey,
		apiToken: apiToken,
		branchID: branchID,
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) addAuth(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-glofox-api-token", c.apiToken)
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("x-glofox-branch-id", c.branchID)
}

// RegisterUserInput holds the fields sent to POST /2.0/register.
type RegisterUserInput struct {
	Email     string
	FirstName string
	LastName  string
	Phone     string
	Password  string
}

// RegisterUserResponse is the relevant subset of Glofox's registration reply.
type RegisterUserResponse struct {
	Success bool `json:"success"`
	User    struct {
		ID    string `json:"_id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
	Message string `json:"message"`
}

// GlofoxLeadStatus maps our internal statuses to Glofox's lead_status values.
// Confirmed valid values: "LEAD", "TRIAL", "MEMBER".
type GlofoxLeadStatus string

const (
	GlofoxStatusTrial  GlofoxLeadStatus = "TRIAL"
	GlofoxStatusMember GlofoxLeadStatus = "MEMBER"
)

// CreateLeadInput holds the fields sent to POST /2.1/branches/{id}/leads.
type CreateLeadInput struct {
	Email      string
	FirstName  string
	LastName   string
	Phone      string
	LeadStatus GlofoxLeadStatus // "TRIAL" or "MEMBER"
	// Gender / BirthDate are optional — when empty, Glofox still gets a
	// neutral placeholder birth date (it requires one), but no gender field.
	Gender    string // our internal values: "male", "female", "other", or "" — translated to Glofox's M/F/O/P enum by glofoxGenderCode
	BirthDate string // "YYYY-MM-DD"; empty uses the neutral placeholder
	// ContactSource / MarketingSource attribute where the lead came from
	// (e.g. "whatsapp_web", "external_sheet") — shown in Glofox's lead
	// source reporting. Optional.
	ContactSource   string
	MarketingSource string
}

// CreateLeadResponse is the relevant subset of Glofox's lead-creation reply.
type CreateLeadResponse struct {
	Success bool `json:"success"`
	Entity  struct {
		ID    string `json:"_id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"entity"`
	Message string `json:"message"`
}

// glofoxGenderCode translates our internal lead.gender values ("male",
// "female", "other", or "" for prefer-not-to-say) into Glofox's fixed
// gender enum: "M", "F", "O", "P". Anything unrecognized falls back to "P"
// rather than sending a value Glofox would reject.
func glofoxGenderCode(gender string) string {
	switch strings.ToLower(strings.TrimSpace(gender)) {
	case "male", "m":
		return "M"
	case "female", "f":
		return "F"
	case "other", "o":
		return "O"
	case "":
		return ""
	default:
		return "P"
	}
}

// CreateLead calls POST /2.1/branches/{branchId}/leads.
// Triggered when a lead converts to trial_booked (TRIAL) or member (MEMBER).
// Glofox requires: type="MEMBER", valid lead_status, and a birth date (min age 16).
// Since we don't collect DOB, a neutral default is sent so Glofox accepts the record.
func (c *Client) CreateLead(ctx context.Context, in CreateLeadInput) (*CreateLeadResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("glofox client not configured")
	}

	status := in.LeadStatus
	if status == "" {
		status = GlofoxStatusTrial
	}

	birth := in.BirthDate
	if birth == "" {
		birth = "1990-01-01" // neutral default when not collected
	}
	body := map[string]any{
		"email":       in.Email,
		"first_name":  in.FirstName,
		"last_name":   in.LastName,
		"type":        "MEMBER",
		"lead_status": string(status),
		"birth":       birth,
	}
	if in.Phone != "" {
		body["phone"] = in.Phone
	}
	if code := glofoxGenderCode(in.Gender); code != "" {
		body["gender"] = code
	}
	if in.ContactSource != "" || in.MarketingSource != "" {
		leads := map[string]any{}
		if in.ContactSource != "" {
			leads["contact_source"] = in.ContactSource
		}
		if in.MarketingSource != "" {
			leads["marketing_source"] = in.MarketingSource
		}
		body["leads"] = leads
	}

	out, err := c.createLead(ctx, body)
	if err != nil && body["leads"] != nil && strings.Contains(err.Error(), "Contact source not allowed") {
		// Glofox only accepts a fixed, undocumented set of contact_source
		// values (e.g. "UNKNOWN", "MEMBER_APP") and rejects the whole lead
		// with a 400 for anything else — "whatsapp_web" among them. Rather
		// than lose the entire lead sync over an informational field, retry
		// once without it.
		delete(body, "leads")
		return c.createLead(ctx, body)
	}
	return out, err
}

// createLead does the actual POST for a prepared request body — split out
// from CreateLead so it can be retried once with the "leads" attribution
// object stripped when Glofox rejects contact_source.
func (c *Client) createLead(ctx context.Context, body map[string]any) (*CreateLeadResponse, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/2.1/branches/"+c.branchID+"/leads", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("glofox request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("glofox status %d: %s", resp.StatusCode, string(raw))
	}

	var out CreateLeadResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if !out.Success {
		return nil, fmt.Errorf("glofox create lead failed: %s", out.Message)
	}
	return &out, nil
}

// PurchaseMembershipInput holds the fields sent to
// POST /2.2/branches/{branchId}/users/{userId}/memberships/{membershipId}/plans/{planCode}/purchase.
type PurchaseMembershipInput struct {
	UserID        string // Glofox user id — the CreateLeadResponse.Entity.ID from CreateLead
	MembershipID  string // studio-specific, looked up via GET /2.0/memberships
	PlanCode      string // studio-specific plan code within that membership
	StartDateUnix int64  // epoch seconds, in the branch's local time
	// PaymentMethod is optional and its accepted values aren't fully
	// documented — since payment was already collected externally via
	// Stripe, this is intentionally left empty by default so Glofox
	// doesn't attempt to charge the customer again through its own
	// payment processor. Only set this if/when confirmed safe to do so.
	PaymentMethod string
}

// PurchaseMembershipResponse is Glofox's reply to a purchase call. Status is
// commonly "PENDING-INTENT" even on success — this reflects Glofox's own
// invoice/payment reconciliation state, not necessarily a failed or
// incomplete purchase from our side.
type PurchaseMembershipResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	MessageCode string `json:"message_code"`
	Status      string `json:"status"`
	InvoiceID   string `json:"invoice_id"`
}

// PurchaseMembership calls POST /2.2/branches/{branchId}/users/{userId}/memberships/{membershipId}/plans/{planCode}/purchase.
// Triggered after a real Stripe trial/membership payment confirms, using the
// studio's own Glofox membership/plan-code mapping — this is what actually
// creates the credit-pack/membership purchase (and Transactions entry) in
// Glofox, not just a bare lead record.
func (c *Client) PurchaseMembership(ctx context.Context, in PurchaseMembershipInput) (*PurchaseMembershipResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("glofox client not configured")
	}
	if in.UserID == "" || in.MembershipID == "" || in.PlanCode == "" {
		return nil, fmt.Errorf("userID, membershipID, and planCode are all required")
	}

	startDate := in.StartDateUnix
	if startDate == 0 {
		startDate = time.Now().Unix()
	}
	body := map[string]any{
		"start_date": startDate,
	}
	if in.PaymentMethod != "" {
		body["payment_method"] = in.PaymentMethod
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/2.2/branches/%s/users/%s/memberships/%s/plans/%s/purchase",
		baseURL, c.branchID, in.UserID, in.MembershipID, in.PlanCode)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("glofox request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("glofox status %d: %s", resp.StatusCode, string(raw))
	}

	var out PurchaseMembershipResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if !out.Success {
		return nil, fmt.Errorf("glofox purchase membership failed: %s", out.Message)
	}
	return &out, nil
}

// MembershipMatch is a Glofox membership/plan pairing found by price.
type MembershipMatch struct {
	MembershipID string
	PlanCode     string
}

// FindMembershipPlanByPrice calls GET /2.0/branches/{branchId}/memberships
// and picks the active plan whose price matches amountCents (Stripe's
// smallest-unit convention, same as everywhere else in this codebase) —
// auto-detecting which Glofox membership/plan a payment corresponds to
// instead of requiring it to be mapped by hand per studio. When more than one
// plan matches the price, one whose membership name mentions "trial" is
// preferred when isTrial is true (and avoided otherwise), since Glofox
// branches commonly have both a trial and a full-price plan at the same
// price point.
func (c *Client) FindMembershipPlanByPrice(ctx context.Context, amountCents int64, isTrial bool) (MembershipMatch, error) {
	if c == nil {
		return MembershipMatch{}, fmt.Errorf("glofox client not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		baseURL+"/2.0/branches/"+c.branchID+"/memberships", nil)
	if err != nil {
		return MembershipMatch{}, fmt.Errorf("build request: %w", err)
	}
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return MembershipMatch{}, fmt.Errorf("glofox request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return MembershipMatch{}, fmt.Errorf("glofox status %d: %s", resp.StatusCode, string(raw))
	}

	var out struct {
		Data []struct {
			ID     string `json:"_id"`
			Name   string `json:"name"`
			Active bool   `json:"active"`
			Plans  []struct {
				Code  json.Number `json:"code"`
				Price float64     `json:"price"`
			} `json:"plans"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return MembershipMatch{}, fmt.Errorf("decode memberships: %w", err)
	}

	target := math.Round(float64(amountCents) / 100.0)
	var fallback MembershipMatch
	for _, m := range out.Data {
		if !m.Active {
			continue
		}
		nameIsTrial := strings.Contains(strings.ToLower(m.Name), "trial")
		for _, p := range m.Plans {
			if math.Round(p.Price) != target {
				continue
			}
			match := MembershipMatch{MembershipID: m.ID, PlanCode: p.Code.String()}
			if nameIsTrial == isTrial {
				return match, nil
			}
			if fallback.MembershipID == "" {
				fallback = match
			}
		}
	}
	if fallback.MembershipID != "" {
		return fallback, nil
	}
	return MembershipMatch{}, fmt.Errorf("no glofox membership plan found matching price %.0f", target)
}

// Booking represents a single booking record from Glofox.
type Booking struct {
	ID        string `json:"_id"`
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	Attended  bool   `json:"attended"`
	ModelName string `json:"model_name"` // class or course name
	Status    string `json:"status"`
	TimeStart string `json:"time_start"`
	Paid      bool   `json:"paid"`
}

type listBookingsResponse struct {
	Data []Booking `json:"data"`
}

// ListBookings fetches all bookings for the branch from GET /2.2/branches/{branchId}/bookings.
// Glofox does not support server-side attended filtering, so all records are returned
// and callers should filter by Booking.Attended == true.
func (c *Client) ListBookings(ctx context.Context) ([]Booking, error) {
	if c == nil {
		return nil, fmt.Errorf("glofox client not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		baseURL+"/2.2/branches/"+c.branchID+"/bookings", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("glofox request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("glofox status %d: %s", resp.StatusCode, string(raw))
	}

	var out listBookingsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode bookings: %w", err)
	}
	return out.Data, nil
}

// GlofoxMember is the member profile returned by GET /2.0/members/{userId}.
type GlofoxMember struct {
	ID           string `json:"_id"`
	Email        string `json:"email"`
	AccountEmail string `json:"account_email"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Phone        string `json:"phone"`
	Membership   struct {
		Status string `json:"status"` // "ACTIVE", "INACTIVE", etc.
		Type   string `json:"type"`   // "payg", "membership", etc.
	} `json:"membership"`
	MemberPurchase bool `json:"MEMBERPURCHASE"` // true if they bought a recurring membership
	PAYGPayment    bool `json:"PAYGPAYMENT"`    // true if they paid for PAYG sessions
	Active         bool `json:"active"`
}

// HasActivePlan returns true when the member has purchased any plan (membership or PAYG).
func (m *GlofoxMember) HasActivePlan() bool {
	return m.Membership.Status == "ACTIVE" || m.MemberPurchase || m.PAYGPayment
}

// ResolveEmail returns the best available email, preferring account_email over email.
func (m *GlofoxMember) ResolveEmail() string {
	if m.Email != "" {
		return m.Email
	}
	return m.AccountEmail
}

// GetMember fetches a single member profile by their Glofox user ID.
func (c *Client) GetMember(ctx context.Context, userID string) (*GlofoxMember, error) {
	if c == nil {
		return nil, fmt.Errorf("glofox client not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		baseURL+"/2.0/members/"+userID, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("glofox request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("glofox status %d: %s", resp.StatusCode, string(raw))
	}

	var out GlofoxMember
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode member: %w", err)
	}
	return &out, nil
}

// RegisterUser calls POST /2.0/register to create the member in Glofox.
func (c *Client) RegisterUser(ctx context.Context, in RegisterUserInput) (*RegisterUserResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("glofox client not configured")
	}

	body := map[string]any{
		"email":      in.Email,
		"first_name": in.FirstName,
		"last_name":  in.LastName,
		"password":   in.Password,
	}
	if in.Phone != "" {
		body["phone"] = in.Phone
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/2.0/register", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("glofox request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("glofox status %d: %s", resp.StatusCode, string(raw))
	}

	var out RegisterUserResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if !out.Success {
		return nil, fmt.Errorf("glofox register failed: %s", out.Message)
	}
	return &out, nil
}
