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

const HeaderRow = "A1:U1"

// LeadExport is the JSON shape of the outbox payload for a "lead.created" or "lead.updated" event
// destined for Google Sheets.
type LeadExport struct {
	ID              string     `json:"id"`
	StudioID        string     `json:"studioId"`
	StudioName      string     `json:"studioName,omitempty"`
	StudioSlug      string     `json:"studioSlug,omitempty"`
	CampaignID      string     `json:"campaignId"`
	CampaignName    string     `json:"campaignName,omitempty"`
	CampaignSlug    string     `json:"campaignSlug,omitempty"`
	Name            string     `json:"name"`
	FirstName       string     `json:"firstName"`
	LastName        string     `json:"lastName"`
	Email           string     `json:"email"`
	Phone           string     `json:"phone"`
	FitnessPlan     string     `json:"fitnessPlan"`
	Goals           string     `json:"goals"`
	Source          string     `json:"source"`
	Status          string     `json:"status"`
	Notes           string     `json:"notes"`
	ContactAttempts int        `json:"contactAttempts"`
	LastContactedAt *time.Time `json:"lastContactedAt,omitempty"`
	ContactMade     bool       `json:"contactMade"`
	HotLead         bool       `json:"hotLead"`
	TrialPurchased  bool       `json:"trialPurchased"`
	AssignedTo      string     `json:"assignedTo"`
	TrialAttended   bool       `json:"trialAttended"`
	MemberSold      bool       `json:"memberSold"`
	MonthlyFee      float64    `json:"monthlyFee"`
	Offer           string     `json:"offer"`
	FurtherNotes    string     `json:"furtherNotes"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// Header columns must match LeadExport.Row() ordering exactly.
var Header = []any{
	"Lead ID",
	"First Name",
	"Last Name",
	"Email Address",
	"Phone Number",
	"Date Of Lead",
	"Lead Source",
	"Offer?",
	"Assigned to",
	"# of Attempts",
	"Last Followed Up?",
	"Contact Made?",
	"HOT LEAD?",
	"Trial Purchased?",
	"Notes on Lead",
	"Trial Attended?",
	"Member Sold?",
	"Monthly Fee",
	"Predicted Revenue Won\n(Based on Monthly Fee x 9 Month Average Lifetime)",
	"Further Notes on Contact",
	"Status",
}

func (l LeadExport) Row() []any {
	lastFollowedUp := ""
	if l.LastContactedAt != nil {
		lastFollowedUp = l.LastContactedAt.Format("2006-01-02 15:04:05")
	}

	contactMadeStr := "No"
	if l.ContactMade {
		contactMadeStr = "Yes"
	}

	hotLeadStr := "No"
	if l.HotLead {
		hotLeadStr = "Yes"
	}

	trialPurchasedStr := "No"
	if l.TrialPurchased {
		trialPurchasedStr = "Yes"
	}

	trialAttendedStr := "No"
	if l.TrialAttended {
		trialAttendedStr = "Yes"
	}

	memberSoldStr := "No"
	if l.MemberSold {
		memberSoldStr = "Yes"
	}

	predictedRevenue := l.MonthlyFee * 9.0

	return []any{
		l.ID,
		l.FirstName,
		l.LastName,
		l.Email,
		l.Phone,
		l.CreatedAt.Format("2006-01-02 15:04:05"),
		l.Source,
		l.Offer,
		l.AssignedTo,
		l.ContactAttempts,
		lastFollowedUp,
		contactMadeStr,
		hotLeadStr,
		trialPurchasedStr,
		l.Notes,
		trialAttendedStr,
		memberSoldStr,
		l.MonthlyFee,
		predictedRevenue,
		l.FurtherNotes,
		l.Status,
	}
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
	if err := c.ensureHeader(ctx, spreadsheetID, resolvedTab, Header, "A1:U1"); err != nil {
		return err
	}
	rowNum, err := c.findLeadRow(ctx, spreadsheetID, resolvedTab, l.ID)
	if err != nil {
		return err
	}
	if rowNum > 0 {
		rng := fmt.Sprintf("%s!A%d:U%d", resolvedTab, rowNum, rowNum)
		_, err = c.svc.Spreadsheets.Values.Update(spreadsheetID, rng, &sheets.ValueRange{
			Values: [][]any{l.Row()},
		}).ValueInputOption("RAW").Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("update row in %s: %w", resolvedTab, err)
		}
	} else {
		_, err = c.svc.Spreadsheets.Values.Append(spreadsheetID, resolvedTab+"!A:U", &sheets.ValueRange{
			Values: [][]any{l.Row()},
		}).
			ValueInputOption("RAW").
			InsertDataOption("INSERT_ROWS").
			Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("append to %s: %w", resolvedTab, err)
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
	rng := tabName + "!A:B"
	resp, err := c.svc.Spreadsheets.Values.Get(spreadsheetID, rng).Context(ctx).Do()
	if err != nil {
		return 0, nil
	}
	for idx, row := range resp.Values {
		if len(row) > 0 && fmt.Sprintf("%v", row[0]) == leadID {
			return idx + 1, nil
		}
		if len(row) > 1 && fmt.Sprintf("%v", row[1]) == leadID {
			return idx + 1, nil
		}
	}
	return 0, nil
}

func (c *Client) ensureHeader(ctx context.Context, spreadsheetID, tabName string, headers []any, rngSuffix string) error {
	rng := tabName + "!" + rngSuffix
	resp, err := c.svc.Spreadsheets.Values.Get(spreadsheetID, rng).Context(ctx).Do()
	if err == nil {
		if len(resp.Values) == 0 || len(resp.Values[0]) == 0 || len(resp.Values[0]) < len(headers) {
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
