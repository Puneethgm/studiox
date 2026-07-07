// Package inbound polls each studio's configured, read-only third-party
// company Google Sheet for new lead rows and imports them into StudioX,
// which automatically triggers the WhatsApp autocontact automation via the
// existing outbox mechanism.
//
// This is intentionally separate from the outbound sheets.Worker: that one
// exports StudioX's own leads into a studio-owned tracker sheet; this one
// reads from an external company's sheet whose layout and ownership we don't
// control. We never write anything back to it — progress is tracked purely
// via a row-number watermark in our own database.
package inbound

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/projectx/api/internal/integrations/sheets"
	"github.com/projectx/api/internal/leads"
)

const pollInterval = 2 * time.Minute

// Worker polls every studio's active external-sheet config on an interval,
// importing any row beyond the last-seen row number as a new lead.
type Worker struct {
	client *sheets.Client
	repo   *leads.Repo
	log    *slog.Logger
}

func New(client *sheets.Client, repo *leads.Repo, log *slog.Logger) *Worker {
	return &Worker{client: client, repo: repo, log: log}
}

func (w *Worker) Run(ctx context.Context) {
	if w.client == nil {
		w.log.Info("Sheets | External leads import worker disabled — configure Google Sheets credentials to enable",
			"component", "external_leads_sheet")
		return
	}
	w.log.Info("Sheets | External leads import worker started",
		"component", "external_leads_sheet", "poll_interval", pollInterval)

	w.tick(ctx)

	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("Sheets | External leads import worker stopping", "component", "external_leads_sheet")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	tCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	configs, err := w.repo.ListActiveExternalLeadsSheetSettings(tCtx)
	if err != nil {
		w.log.Warn("Sheets | External leads import worker — failed to list studio configs, will retry next poll",
			"component", "external_leads_sheet", "error", err.Error())
		return
	}

	imported := 0
	for _, cfg := range configs {
		n, err := w.importFromSheet(tCtx, cfg)
		if err != nil {
			w.log.Warn("Sheets | External leads import worker — failed to import from studio's external sheet",
				"component", "external_leads_sheet", "studio_id", cfg.StudioID, "error", err.Error())
			continue
		}
		imported += n
	}

	w.log.Info("Sheets | External leads import worker cycle complete",
		"component", "external_leads_sheet", "studios_checked", len(configs), "leads_imported", imported)
}

func (w *Worker) importFromSheet(ctx context.Context, cfg leads.ExternalLeadsSheetSettings) (int, error) {
	rows, err := w.client.ReadRows(ctx, cfg.SpreadsheetID, cfg.TabName)
	if err != nil {
		return 0, fmt.Errorf("read rows: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	lastImported, err := w.repo.GetExternalSheetImportWatermark(ctx, cfg.SpreadsheetID, cfg.TabName)
	if err != nil {
		return 0, fmt.Errorf("get watermark: %w", err)
	}

	nameCol := columnIndex(cfg.NameColumn)
	firstNameCol := columnIndex(cfg.FirstNameColumn)
	lastNameCol := columnIndex(cfg.LastNameColumn)
	emailCol := columnIndex(cfg.EmailColumn)
	phoneCol := columnIndex(cfg.PhoneColumn)
	sourceCol := columnIndex(cfg.SourceColumn)
	notesCol := columnIndex(cfg.NotesColumn)

	var defaultCampaign *leads.Campaign
	imported := 0
	maxRowSeen := lastImported

	for i, row := range rows {
		sheetRowNum := i + 2 // +2: header is row 1, data starts at row 2
		if sheetRowNum <= lastImported {
			continue
		}
		if sheetRowNum > maxRowSeen {
			maxRowSeen = sheetRowNum
		}

		firstName, lastName := resolveName(row, nameCol, firstNameCol, lastNameCol)
		email := cell(row, emailCol)
		phone := cell(row, phoneCol)
		if firstName == "" && lastName == "" && email == "" && phone == "" {
			continue // blank row
		}

		if defaultCampaign == nil {
			defaultCampaign, err = w.repo.GetOldestActiveCampaign(ctx, cfg.StudioID)
			if err != nil {
				return imported, fmt.Errorf("resolve default campaign: %w", err)
			}
		}

		source := cell(row, sourceCol)
		if source == "" {
			source = "external_sheet"
		}

		lead := &leads.Lead{
			StudioID:   cfg.StudioID,
			CampaignID: defaultCampaign.ID,
			FirstName:  firstName,
			LastName:   lastName,
			Email:      email,
			Phone:      phone,
			Source:     source,
			Notes:      cell(row, notesCol),
		}

		if err := w.repo.CreateLeadWithOutbox(ctx, lead, "lead_autocontact"); err != nil {
			w.log.Warn("Sheets | External leads import worker — failed to create lead from sheet row",
				"component", "external_leads_sheet", "studio_id", cfg.StudioID, "row", sheetRowNum, "error", err.Error())
			continue
		}

		w.log.Info("Sheets | External leads import worker — imported lead",
			"component", "external_leads_sheet", "studio_id", cfg.StudioID, "row", sheetRowNum, "lead_id", lead.ID)
		imported++
	}

	if maxRowSeen > lastImported {
		if err := w.repo.SetExternalSheetImportWatermark(ctx, cfg.SpreadsheetID, cfg.TabName, cfg.StudioID, maxRowSeen); err != nil {
			return imported, fmt.Errorf("advance watermark: %w", err)
		}
	}

	return imported, nil
}

func resolveName(row []any, nameCol, firstNameCol, lastNameCol int) (firstName, lastName string) {
	if nameCol >= 0 {
		full := cell(row, nameCol)
		parts := strings.SplitN(full, " ", 2)
		firstName = parts[0]
		if len(parts) > 1 {
			lastName = parts[1]
		}
		return firstName, lastName
	}
	return cell(row, firstNameCol), cell(row, lastNameCol)
}

func cell(row []any, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", row[idx]))
}

// columnIndex converts a spreadsheet column letter ("A", "B", ..., "AA") into
// a zero-based index. Returns -1 for an empty/unset column.
func columnIndex(col string) int {
	col = strings.ToUpper(strings.TrimSpace(col))
	if col == "" {
		return -1
	}
	// Support a raw numeric index too, for flexibility.
	if n, err := strconv.Atoi(col); err == nil {
		return n
	}
	idx := 0
	for _, ch := range col {
		if ch < 'A' || ch > 'Z' {
			return -1
		}
		idx = idx*26 + int(ch-'A'+1)
	}
	return idx - 1
}
