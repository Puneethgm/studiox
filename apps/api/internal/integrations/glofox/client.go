package glofox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	body := map[string]any{
		"email":       in.Email,
		"first_name":  in.FirstName,
		"last_name":   in.LastName,
		"type":        "MEMBER",
		"lead_status": string(status),
		"birth":       "1990-01-01", // neutral default — DOB not collected at lead capture
	}
	if in.Phone != "" {
		body["phone"] = in.Phone
	}

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
