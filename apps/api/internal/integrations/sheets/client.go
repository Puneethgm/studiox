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
	"strings"
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
	Notes        string    `json:"notes"`
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
	"Notes",
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
		l.Notes,
	}
}

var BookedTrialsHeader = []any{
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
	"Trial Date/Time",
	"Goals",
	"Source",
	"Status",
	"Notes",
}

func (l LeadExport) BookedTrialsRow() []any {
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
		extractTrialSlot(l.Notes),
		l.Goals,
		l.Source,
		l.Status,
		l.Notes,
	}
}

func extractTrialSlot(notes string) string {
	idx := strings.Index(notes, "[Selected Trial Slot]: ")
	if idx == -1 {
		return ""
	}
	val := notes[idx+len("[Selected Trial Slot]: "):]
	if end := strings.Index(val, "\n"); end != -1 {
		val = val[:end]
	}
	return strings.TrimSpace(val)
}

type Client struct {
	svc *sheets.Service
}

// NewClient loads the service-account credentials and verifies access. Returns
// (nil, nil) when sheets are not configured — callers should treat that as
// "skip the integration".
func NewClient(ctx context.Context, credentialsPath string) (*Client, error) {
	if credentialsPath == "" {
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
	return &Client{svc: svc}, nil
}

// AppendLead writes one row to the Leads tab and optionally specialized tabs based on status.
func (c *Client) AppendLead(ctx context.Context, spreadsheetID, tab string, payload []byte) error {
	if spreadsheetID == "" {
		return fmt.Errorf("spreadsheet ID is empty")
	}
	if tab == "" {
		tab = "Leads"
	}

	var l LeadExport
	if err := json.Unmarshal(payload, &l); err != nil {
		return fmt.Errorf("unmarshal lead: %w", err)
	}

	// 1. Sync to default Leads tab
	resolvedTab, err := c.ensureTabExists(ctx, spreadsheetID, tab)
	if err != nil {
		return err
	}
	if err := c.ensureHeader(ctx, spreadsheetID, resolvedTab, Header, "A1:N1"); err != nil {
		return err
	}
	rowNum, err := c.findLeadRow(ctx, spreadsheetID, resolvedTab, l.ID)
	if err != nil {
		return err
	}
	if rowNum > 0 {
		rng := fmt.Sprintf("%s!A%d:N%d", resolvedTab, rowNum, rowNum)
		_, err = c.svc.Spreadsheets.Values.Update(spreadsheetID, rng, &sheets.ValueRange{
			Values: [][]any{l.Row()},
		}).ValueInputOption("RAW").Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("update row in %s: %w", resolvedTab, err)
		}
	} else {
		_, err = c.svc.Spreadsheets.Values.Append(spreadsheetID, resolvedTab+"!A:N", &sheets.ValueRange{
			Values: [][]any{l.Row()},
		}).
			ValueInputOption("RAW").
			InsertDataOption("INSERT_ROWS").
			Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("append to %s: %w", resolvedTab, err)
		}
	}

	// 2. Sync to "Booked Trials" tab if status is trial_booked
	if l.Status == "trial_booked" {
		bookedTab := "Booked Trials"
		resolvedBookedTab, err := c.ensureTabExists(ctx, spreadsheetID, bookedTab)
		if err != nil {
			return err
		}
		if err := c.ensureHeader(ctx, spreadsheetID, resolvedBookedTab, BookedTrialsHeader, "A1:O1"); err != nil {
			return err
		}
		rowNum, err := c.findLeadRow(ctx, spreadsheetID, resolvedBookedTab, l.ID)
		if err != nil {
			return err
		}
		if rowNum > 0 {
			rng := fmt.Sprintf("%s!A%d:O%d", resolvedBookedTab, rowNum, rowNum)
			_, err = c.svc.Spreadsheets.Values.Update(spreadsheetID, rng, &sheets.ValueRange{
				Values: [][]any{l.BookedTrialsRow()},
			}).ValueInputOption("RAW").Context(ctx).Do()
			if err != nil {
				return fmt.Errorf("update row in %s: %w", resolvedBookedTab, err)
			}
		} else {
			_, err = c.svc.Spreadsheets.Values.Append(spreadsheetID, resolvedBookedTab+"!A:O", &sheets.ValueRange{
				Values: [][]any{l.BookedTrialsRow()},
			}).
				ValueInputOption("RAW").
				InsertDataOption("INSERT_ROWS").
				Context(ctx).Do()
			if err != nil {
				return fmt.Errorf("append to %s: %w", resolvedBookedTab, err)
			}
		}
	}

	// 3. Sync to "Members" tab if status is member
	if l.Status == "member" {
		membersTab := "Members"
		resolvedMembersTab, err := c.ensureTabExists(ctx, spreadsheetID, membersTab)
		if err != nil {
			return err
		}
		if err := c.ensureHeader(ctx, spreadsheetID, resolvedMembersTab, Header, "A1:N1"); err != nil {
			return err
		}
		rowNum, err := c.findLeadRow(ctx, spreadsheetID, resolvedMembersTab, l.ID)
		if err != nil {
			return err
		}
		if rowNum > 0 {
			rng := fmt.Sprintf("%s!A%d:N%d", resolvedMembersTab, rowNum, rowNum)
			_, err = c.svc.Spreadsheets.Values.Update(spreadsheetID, rng, &sheets.ValueRange{
				Values: [][]any{l.Row()},
			}).ValueInputOption("RAW").Context(ctx).Do()
			if err != nil {
				return fmt.Errorf("update row in %s: %w", resolvedMembersTab, err)
			}
		} else {
			_, err = c.svc.Spreadsheets.Values.Append(spreadsheetID, resolvedMembersTab+"!A:N", &sheets.ValueRange{
				Values: [][]any{l.Row()},
			}).
				ValueInputOption("RAW").
				InsertDataOption("INSERT_ROWS").
				Context(ctx).Do()
			if err != nil {
				return fmt.Errorf("append to %s: %w", resolvedMembersTab, err)
			}
		}
	}

	return nil
}

func (c *Client) ensureTabExists(ctx context.Context, spreadsheetID, tabName string) (string, error) {
	spreadsheet, err := c.svc.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("get spreadsheet: %w", err)
	}

	for _, sheet := range spreadsheet.Sheets {
		if strings.EqualFold(sheet.Properties.Title, tabName) {
			return sheet.Properties.Title, nil
		}
	}

	// Create tab
	req := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				AddSheet: &sheets.AddSheetRequest{
					Properties: &sheets.SheetProperties{
						Title: tabName,
					},
				},
			},
		},
	}
	_, err = c.svc.Spreadsheets.BatchUpdate(spreadsheetID, req).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("add sheet %q: %w", tabName, err)
	}
	return tabName, nil
}

func (c *Client) findLeadRow(ctx context.Context, spreadsheetID, tabName, leadID string) (int, error) {
	rng := tabName + "!B:B"
	resp, err := c.svc.Spreadsheets.Values.Get(spreadsheetID, rng).Context(ctx).Do()
	if err != nil {
		return 0, nil
	}
	for idx, row := range resp.Values {
		if len(row) > 0 && fmt.Sprintf("%v", row[0]) == leadID {
			return idx + 1, nil
		}
	}
	return 0, nil
}

func (c *Client) ensureHeader(ctx context.Context, spreadsheetID, tabName string, headers []any, rngSuffix string) error {
	rng := tabName + "!" + rngSuffix
	resp, err := c.svc.Spreadsheets.Values.Get(spreadsheetID, rng).Context(ctx).Do()
	if err == nil {
		if len(resp.Values) == 0 || len(resp.Values[0]) == 0 {
			_, err = c.svc.Spreadsheets.Values.Update(spreadsheetID, rng, &sheets.ValueRange{
				Values: [][]any{headers},
			}).ValueInputOption("RAW").Context(ctx).Do()
			if err != nil {
				return fmt.Errorf("write headers to %s: %w", tabName, err)
			}
		}
	}
	return nil
}
