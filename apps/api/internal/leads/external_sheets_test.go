package leads

import (
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeExternalSheet(t *testing.T) {
	studio := uuid.New()

	t.Run("applies the same defaults the single-sheet version always did", func(t *testing.T) {
		got := normalizeExternalSheet(studio, ExternalLeadsSheetSettings{
			SpreadsheetID: "https://docs.google.com/spreadsheets/d/abc123XYZ/edit#gid=0",
		})
		if got.StudioID != studio {
			t.Errorf("studio = %v, want %v", got.StudioID, studio)
		}
		if got.SpreadsheetID != "abc123XYZ" {
			t.Errorf("spreadsheet id = %q, want the ID extracted from the URL", got.SpreadsheetID)
		}
		if got.TabName != "Sheet1" || got.FirstNameColumn != "A" || got.LastNameColumn != "B" ||
			got.EmailColumn != "C" || got.PhoneColumn != "D" {
			t.Errorf("defaults not applied: tab=%q first=%q last=%q email=%q phone=%q",
				got.TabName, got.FirstNameColumn, got.LastNameColumn, got.EmailColumn, got.PhoneColumn)
		}
	})

	t.Run("full-name column suppresses the first/last defaults", func(t *testing.T) {
		got := normalizeExternalSheet(studio, ExternalLeadsSheetSettings{SpreadsheetID: "id1", NameColumn: " b "})
		if got.NameColumn != "B" || got.FirstNameColumn != "" || got.LastNameColumn != "" {
			t.Errorf("name=%q first=%q last=%q", got.NameColumn, got.FirstNameColumn, got.LastNameColumn)
		}
	})

	t.Run("each sheet keeps its own settings", func(t *testing.T) {
		a := normalizeExternalSheet(studio, ExternalLeadsSheetSettings{SpreadsheetID: "sheetA", TabName: "Leads", EmailColumn: "f", AutoContactEnabled: true, Active: true})
		b := normalizeExternalSheet(studio, ExternalLeadsSheetSettings{SpreadsheetID: "sheetB", TabName: " Jan ", PhoneColumn: "g", ContinueAIAfterGreeting: true})
		if a.EmailColumn != "F" || a.PhoneColumn != "D" || !a.AutoContactEnabled || !a.Active {
			t.Errorf("sheet A mixed up: %+v", a)
		}
		if b.TabName != "Jan" || b.PhoneColumn != "G" || b.EmailColumn != "C" || b.AutoContactEnabled || b.Active || !b.ContinueAIAfterGreeting {
			t.Errorf("sheet B mixed up: %+v", b)
		}
	})

	t.Run("keeps the id it is given so updates target one sheet", func(t *testing.T) {
		id := uuid.New()
		got := normalizeExternalSheet(studio, ExternalLeadsSheetSettings{ID: id, SpreadsheetID: "x"})
		if got.ID != id {
			t.Errorf("id = %v, want %v", got.ID, id)
		}
	})
}
