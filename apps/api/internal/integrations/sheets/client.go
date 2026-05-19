// Package sheets pushes lead exports to a Google Sheet via a service account.
//
// The platform tolerates Sheets being unconfigured or temporarily down: the
// outbox table holds every queued row, so submissions to the public form are
// never blocked on Sheets availability.
package sheets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const HeaderRow = "A1:M1"

// LeadExport is the JSON shape of the outbox payload for a "lead.created" event
// destined for Google Sheets. It deliberately mirrors the column order of the
// header row so the worker can flatten it without per-field decisions.
type LeadExport struct {
	ID           string    `json:"id"`
	StudioID     string    `json:"studioId"`
	StudioName   string    `json:"studioName,omitempty"`
	StudioSlug   string    `json:"studioSlug,omitempty"`
	CampaignID   string    `json:"campaignId"`
	CampaignName string    `json:"campaignName,omitempty"`
	CampaignSlug string    `json:"campaignSlug,omitempty"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	FitnessPlan  string    `json:"fitnessPlan"`
	Goals        string    `json:"goals"`
	Source       string    `json:"source"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Header columns must match LeadExport.Row() ordering exactly.
var Header = []any{
	"Submitted At",
	"Lead ID",
	"Studio",
	"Studio Slug",
	"Campaign",
	"Campaign Slug",
	"Name",
	"Email",
	"Phone",
	"Fitness Plan",
	"Goals",
	"Source",
	"Status",
}

func (l LeadExport) Row() []any {
	return []any{
		l.CreatedAt.Format(time.RFC3339),
		l.ID,
		l.StudioName,
		l.StudioSlug,
		l.CampaignName,
		l.CampaignSlug,
		l.Name,
		l.Email,
		l.Phone,
		l.FitnessPlan,
		l.Goals,
		l.Source,
		l.Status,
	}
}

type Client struct {
	svc           *sheets.Service
	spreadsheetID string
	tab           string
}

// NewClient loads the service-account credentials and verifies access. Returns
// (nil, nil) when sheets are not configured — callers should treat that as
// "skip the integration".
func NewClient(ctx context.Context, credentialsPath, spreadsheetID, tab string) (*Client, error) {
	if credentialsPath == "" || spreadsheetID == "" {
		return nil, nil
	}
	if _, err := os.Stat(credentialsPath); err != nil {
		return nil, fmt.Errorf("credentials file %q: %w", credentialsPath, err)
	}
	svc, err := sheets.NewService(ctx,
		option.WithCredentialsFile(credentialsPath),
		option.WithScopes(sheets.SpreadsheetsScope),
	)
	if err != nil {
		return nil, fmt.Errorf("init sheets service: %w", err)
	}
	c := &Client{svc: svc, spreadsheetID: spreadsheetID, tab: tab}
	if err := c.ensureHeader(ctx); err != nil {
		return nil, fmt.Errorf("ensure header: %w", err)
	}
	return c, nil
}

// ensureHeader writes the column header to row 1 if the sheet is empty.
func (c *Client) ensureHeader(ctx context.Context) error {
	rng := c.tab + "!" + HeaderRow
	resp, err := c.svc.Spreadsheets.Values.Get(c.spreadsheetID, rng).Context(ctx).Do()
	if err != nil {
		return err
	}
	if len(resp.Values) > 0 && len(resp.Values[0]) > 0 {
		return nil
	}
	_, err = c.svc.Spreadsheets.Values.Update(c.spreadsheetID, rng, &sheets.ValueRange{
		Values: [][]any{Header},
	}).ValueInputOption("RAW").Context(ctx).Do()
	return err
}

// AppendLead writes one row to the Leads tab.
func (c *Client) AppendLead(ctx context.Context, payload []byte) error {
	var l LeadExport
	if err := json.Unmarshal(payload, &l); err != nil {
		return fmt.Errorf("unmarshal lead: %w", err)
	}
	_, err := c.svc.Spreadsheets.Values.Append(c.spreadsheetID, c.tab+"!A:M", &sheets.ValueRange{
		Values: [][]any{l.Row()},
	}).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("append row: %w", err)
	}
	return nil
}
