package messaging

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// Import: parse -> stage -> (prompt for country code if needed) -> confirm
// ============================================================

// ParseBroadcastContactRows maps a raw sheet (first row = header) into
// ImportedContactRow, using fuzzy header matching like leads' CSV import —
// "Client Name"/"Name", "Phone"/"Phone Number"/"Mobile", "Email" are all
// recognized. Rows missing both a name and a phone are skipped (blank
// trailing rows some spreadsheet exports leave behind).
func ParseBroadcastContactRows(sheet [][]string) ([]ImportedContactRow, error) {
	if len(sheet) == 0 {
		return nil, errors.New("file is empty")
	}
	col := map[string]int{}
	for i, h := range sheet[0] {
		h = strings.ToLower(strings.TrimSpace(h))
		switch {
		case strings.Contains(h, "name"):
			col["name"] = i
		case strings.Contains(h, "phone") || strings.Contains(h, "mobile") || strings.Contains(h, "number") || strings.Contains(h, "contact"):
			col["phone"] = i
		case strings.Contains(h, "email"):
			col["email"] = i
		}
	}
	if _, ok := col["phone"]; !ok {
		return nil, errors.New("couldn't find a phone number column — expected a header containing \"phone\"")
	}

	get := func(row []string, key string) string {
		i, ok := col[key]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	var out []ImportedContactRow
	for _, row := range sheet[1:] {
		name := get(row, "name")
		phone := get(row, "phone")
		if name == "" && phone == "" {
			continue
		}
		out = append(out, ImportedContactRow{
			Name:  name,
			Phone: phone,
			Email: get(row, "email"),
		})
	}
	if len(out) == 0 {
		return nil, errors.New("no contact rows found below the header")
	}
	return out, nil
}

// phoneDigitsRe strips everything but digits, for normalizing a number
// before a country code is prepended.
var phoneDigitsRe = regexp.MustCompile(`[^0-9]`)

// hasCountryCode reports whether phone already looks like it has one — a
// leading '+' is the only reliable signal we treat as "already has a
// country code"; everything else (bare local-format numbers) needs one
// applied via applyCountryCode.
func hasCountryCode(phone string) bool {
	return strings.HasPrefix(strings.TrimSpace(phone), "+")
}

// applyCountryCode strips any leading zeros (a local trunk prefix, e.g.
// Singapore's old "0" or various local formats) and non-digit characters,
// then prepends "+<code>". Not a full E.164 validator — just enough to
// turn "91234567" + "65" into "+6591234567", matching what a studio admin
// would type by hand.
func applyCountryCode(phone, countryCode string) string {
	digits := phoneDigitsRe.ReplaceAllString(phone, "")
	digits = strings.TrimLeft(digits, "0")
	code := phoneDigitsRe.ReplaceAllString(countryCode, "")
	return "+" + code + digits
}

// StageBroadcastImport parses the uploaded sheet and stores it for a
// follow-up confirm call — see broadcast_import_staging's doc comment for
// why this is a two-step flow instead of one.
func (s *Service) StageBroadcastImport(ctx context.Context, studioID uuid.UUID, sheet [][]string) (*BroadcastImportPreview, error) {
	rows, err := ParseBroadcastContactRows(sheet)
	if err != nil {
		return nil, err
	}

	importID, err := s.repo.CreateImportStaging(ctx, studioID, rows)
	if err != nil {
		return nil, err
	}

	preview := &BroadcastImportPreview{ImportID: importID, TotalRows: len(rows)}
	for _, r := range rows {
		if !hasCountryCode(r.Phone) {
			preview.MissingCountryCode++
			if len(preview.SampleMissingNumbers) < 5 {
				preview.SampleMissingNumbers = append(preview.SampleMissingNumbers, r.Phone)
			}
		}
	}
	return preview, nil
}

// ConfirmBroadcastImport finalizes a staged import into a saved list.
// defaultCountryCode is applied to every row missing one; required if the
// preview reported MissingCountryCode > 0 (callers should have already
// prompted the admin for it), but harmless to pass even when not needed
// (rows that already have a '+' are left untouched).
func (s *Service) ConfirmBroadcastImport(ctx context.Context, studioID, importID uuid.UUID, listName, defaultCountryCode string) (*BroadcastList, error) {
	listName = strings.TrimSpace(listName)
	if listName == "" {
		return nil, errors.New("list name is required")
	}

	rows, err := s.repo.GetImportStaging(ctx, studioID, importID)
	if err != nil {
		return nil, err
	}

	for i := range rows {
		if !hasCountryCode(rows[i].Phone) {
			if defaultCountryCode == "" {
				return nil, fmt.Errorf("some numbers are missing a country code — defaultCountryCode is required")
			}
			rows[i].Phone = applyCountryCode(rows[i].Phone, defaultCountryCode)
		}
	}

	list, err := s.repo.CreateBroadcastList(ctx, studioID, listName, rows)
	if err != nil {
		return nil, err
	}
	// Best-effort cleanup — a leftover staging row is harmless, not worth
	// failing the whole import over.
	_ = s.repo.DeleteImportStaging(ctx, studioID, importID)
	return list, nil
}

func (s *Service) ListBroadcastLists(ctx context.Context, studioID uuid.UUID) ([]BroadcastList, error) {
	return s.repo.ListBroadcastLists(ctx, studioID)
}

func (s *Service) GetBroadcastListWithContacts(ctx context.Context, studioID, listID uuid.UUID) (*BroadcastList, []BroadcastContact, error) {
	list, err := s.repo.GetBroadcastList(ctx, studioID, listID)
	if err != nil {
		return nil, nil, err
	}
	contacts, err := s.repo.ListBroadcastContacts(ctx, listID)
	if err != nil {
		return nil, nil, err
	}
	return list, contacts, nil
}

func (s *Service) DeleteBroadcastList(ctx context.Context, studioID, listID uuid.UUID) error {
	return s.repo.DeleteBroadcastList(ctx, studioID, listID)
}

// ============================================================
// Campaigns
// ============================================================

type CreateBroadcastCampaignInput struct {
	BroadcastListID uuid.UUID
	Channel         BroadcastChannel // defaults to whatsapp when empty, for callers written before email existed
	Subject         string           // required when Channel == email
	Body            string
	Attachments     []Attachment
	ScheduledFor    time.Time // zero means "now"
}

func (s *Service) CreateBroadcastCampaign(ctx context.Context, studioID uuid.UUID, in CreateBroadcastCampaignInput) (*BroadcastCampaign, error) {
	in.Body = strings.TrimSpace(in.Body)
	in.Subject = strings.TrimSpace(in.Subject)
	if in.Body == "" && len(in.Attachments) == 0 {
		return nil, errors.New("message body or an attachment is required")
	}
	if in.Channel == "" {
		in.Channel = BroadcastChannelWhatsApp
	}
	if in.Channel != BroadcastChannelWhatsApp && in.Channel != BroadcastChannelEmail {
		return nil, fmt.Errorf("unknown channel %q — use whatsapp or email", in.Channel)
	}
	if in.Channel == BroadcastChannelEmail && in.Subject == "" {
		return nil, errors.New("subject is required for an email broadcast")
	}
	if _, err := s.repo.GetBroadcastList(ctx, studioID, in.BroadcastListID); err != nil {
		return nil, err
	}
	// Confirm the studio actually has somewhere to send this before
	// scheduling it — otherwise it'd sit as "scheduled" forever since the
	// worker's resolveChannelKind would never find a connected channel.
	if _, err := s.resolveBroadcastChannelKind(ctx, studioID, in.Channel); err != nil {
		return nil, err
	}
	if in.ScheduledFor.IsZero() {
		in.ScheduledFor = time.Now().UTC()
	}
	return s.repo.CreateBroadcastCampaign(ctx, studioID, in.BroadcastListID, in.Channel, in.Subject, in.Body, in.Attachments, in.ScheduledFor)
}

// resolveBroadcastChannelKind maps a BroadcastChannel to whichever actual
// connected ChannelKind the studio has for it (WhatsApp prefers the Meta
// Cloud API, falling back to WhatsApp Web/QR). Returns an error naming what
// to connect when nothing matches, so campaign creation fails fast instead
// of silently scheduling something that can never send.
func (s *Service) resolveBroadcastChannelKind(ctx context.Context, studioID uuid.UUID, ch BroadcastChannel) (ChannelKind, error) {
	if ch == BroadcastChannelEmail {
		if _, err := s.repo.GetActiveChannelByKind(ctx, studioID, KindEmailSMTP); err == nil {
			return KindEmailSMTP, nil
		}
		return "", errors.New("no connected email channel for this studio — connect one under Channels first")
	}
	if _, err := s.repo.GetActiveChannelByKind(ctx, studioID, KindWhatsAppMeta); err == nil {
		return KindWhatsAppMeta, nil
	}
	if _, err := s.repo.GetActiveChannelByKind(ctx, studioID, KindWhatsAppWeb); err == nil {
		return KindWhatsAppWeb, nil
	}
	return "", errors.New("no connected WhatsApp channel for this studio — connect one under Channels first")
}

func (s *Service) ListBroadcastCampaigns(ctx context.Context, studioID uuid.UUID) ([]BroadcastCampaign, error) {
	return s.repo.ListBroadcastCampaigns(ctx, studioID)
}

func (s *Service) CancelBroadcastCampaign(ctx context.Context, studioID, campaignID uuid.UUID) error {
	return s.repo.CancelBroadcastCampaign(ctx, studioID, campaignID)
}

func (s *Service) ListBroadcastCampaignRecipients(ctx context.Context, studioID, campaignID uuid.UUID) ([]BroadcastRecipientDetail, error) {
	return s.repo.ListBroadcastCampaignRecipients(ctx, studioID, campaignID)
}
