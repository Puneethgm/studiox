package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ============================================================
// broadcast_import_staging
// ============================================================

func (r *Repo) CreateImportStaging(ctx context.Context, studioID uuid.UUID, rows []ImportedContactRow) (uuid.UUID, error) {
	data, err := json.Marshal(rows)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal staged rows: %w", err)
	}
	var id uuid.UUID
	err = r.pool.QueryRow(ctx, `
		INSERT INTO broadcast_import_staging (studio_id, rows) VALUES ($1, $2) RETURNING id
	`, studioID, string(data)).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert import staging: %w", err)
	}
	return id, nil
}

func (r *Repo) GetImportStaging(ctx context.Context, studioID, importID uuid.UUID) ([]ImportedContactRow, error) {
	var data []byte
	err := r.pool.QueryRow(ctx, `
		SELECT rows FROM broadcast_import_staging WHERE id = $1 AND studio_id = $2
	`, importID, studioID).Scan(&data)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get import staging: %w", err)
	}
	var rows []ImportedContactRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal staged rows: %w", err)
	}
	return rows, nil
}

func (r *Repo) DeleteImportStaging(ctx context.Context, studioID, importID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM broadcast_import_staging WHERE id = $1 AND studio_id = $2
	`, importID, studioID)
	if err != nil {
		return fmt.Errorf("delete import staging: %w", err)
	}
	return nil
}

// ============================================================
// broadcast_lists / broadcast_contacts
// ============================================================

// CreateBroadcastList creates the list and all its contacts in one
// transaction — a list with zero contacts (e.g. every row failed
// validation) is never left behind.
func (r *Repo) CreateBroadcastList(ctx context.Context, studioID uuid.UUID, name string, contacts []ImportedContactRow) (*BroadcastList, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	list := &BroadcastList{StudioID: studioID, Name: name}
	if err := tx.QueryRow(ctx, `
		INSERT INTO broadcast_lists (studio_id, name) VALUES ($1, $2)
		RETURNING id, created_at
	`, studioID, name).Scan(&list.ID, &list.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert broadcast list: %w", err)
	}

	for _, c := range contacts {
		if _, err := tx.Exec(ctx, `
			INSERT INTO broadcast_contacts (broadcast_list_id, name, phone, email)
			VALUES ($1, $2, $3, $4)
		`, list.ID, c.Name, c.Phone, c.Email); err != nil {
			return nil, fmt.Errorf("insert broadcast contact: %w", err)
		}
	}
	list.ContactCount = len(contacts)

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return list, nil
}

func (r *Repo) ListBroadcastLists(ctx context.Context, studioID uuid.UUID) ([]BroadcastList, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT l.id, l.studio_id, l.name, l.created_at, count(c.id)
		FROM broadcast_lists l
		LEFT JOIN broadcast_contacts c ON c.broadcast_list_id = l.id
		WHERE l.studio_id = $1
		GROUP BY l.id
		ORDER BY l.created_at DESC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list broadcast lists: %w", err)
	}
	defer rows.Close()

	out := make([]BroadcastList, 0)
	for rows.Next() {
		var l BroadcastList
		if err := rows.Scan(&l.ID, &l.StudioID, &l.Name, &l.CreatedAt, &l.ContactCount); err != nil {
			return nil, fmt.Errorf("scan broadcast list: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *Repo) GetBroadcastList(ctx context.Context, studioID, listID uuid.UUID) (*BroadcastList, error) {
	var l BroadcastList
	err := r.pool.QueryRow(ctx, `
		SELECT l.id, l.studio_id, l.name, l.created_at, count(c.id)
		FROM broadcast_lists l
		LEFT JOIN broadcast_contacts c ON c.broadcast_list_id = l.id
		WHERE l.id = $1 AND l.studio_id = $2
		GROUP BY l.id
	`, listID, studioID).Scan(&l.ID, &l.StudioID, &l.Name, &l.CreatedAt, &l.ContactCount)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get broadcast list: %w", err)
	}
	return &l, nil
}

func (r *Repo) ListBroadcastContacts(ctx context.Context, listID uuid.UUID) ([]BroadcastContact, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, broadcast_list_id, name, phone, email, created_at
		FROM broadcast_contacts WHERE broadcast_list_id = $1
		ORDER BY created_at ASC
	`, listID)
	if err != nil {
		return nil, fmt.Errorf("list broadcast contacts: %w", err)
	}
	defer rows.Close()

	out := make([]BroadcastContact, 0)
	for rows.Next() {
		var c BroadcastContact
		if err := rows.Scan(&c.ID, &c.BroadcastListID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan broadcast contact: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repo) DeleteBroadcastList(ctx context.Context, studioID, listID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM broadcast_lists WHERE id = $1 AND studio_id = $2
	`, listID, studioID)
	if err != nil {
		return fmt.Errorf("delete broadcast list: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================
// broadcast_campaigns / broadcast_campaign_recipients
// ============================================================

// CreateBroadcastCampaign creates the campaign and snapshots every current
// member of the list as a pending recipient, in one transaction — later
// additions to the list don't retroactively grow a campaign already
// scheduled against it.
func (r *Repo) CreateBroadcastCampaign(ctx context.Context, studioID, listID uuid.UUID, channel BroadcastChannel, subject, body string, attachments []Attachment, scheduledFor time.Time) (*BroadcastCampaign, error) {
	attsBytes, err := json.Marshal(attachments)
	if err != nil {
		return nil, fmt.Errorf("marshal attachments: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var contactIDs []uuid.UUID
	rows, err := tx.Query(ctx, `SELECT id FROM broadcast_contacts WHERE broadcast_list_id = $1`, listID)
	if err != nil {
		return nil, fmt.Errorf("list contacts for campaign: %w", err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan contact id: %w", err)
		}
		contactIDs = append(contactIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list contacts for campaign: %w", err)
	}
	if len(contactIDs) == 0 {
		return nil, ErrEmptyBroadcastList
	}

	c := &BroadcastCampaign{
		StudioID:        studioID,
		BroadcastListID: listID,
		Channel:         channel,
		Subject:         subject,
		Body:            body,
		Attachments:     attachments,
		ScheduledFor:    scheduledFor,
		Status:          BroadcastScheduled,
		TotalCount:      len(contactIDs),
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO broadcast_campaigns (studio_id, broadcast_list_id, channel_kind, subject, body, attachments, scheduled_for, total_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, status, created_at, updated_at
	`, studioID, listID, string(channel), subject, body, string(attsBytes), scheduledFor, len(contactIDs)).
		Scan(&c.ID, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert broadcast campaign: %w", err)
	}

	for _, contactID := range contactIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO broadcast_campaign_recipients (campaign_id, broadcast_contact_id)
			VALUES ($1, $2)
		`, c.ID, contactID); err != nil {
			return nil, fmt.Errorf("insert broadcast campaign recipient: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return c, nil
}

func (r *Repo) ListBroadcastCampaigns(ctx context.Context, studioID uuid.UUID) ([]BroadcastCampaign, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.studio_id, c.broadcast_list_id, l.name, c.channel_kind, c.subject, c.body, c.attachments,
		       c.scheduled_for, c.status, c.total_count, c.enqueued_count, c.created_at, c.updated_at
		FROM broadcast_campaigns c
		JOIN broadcast_lists l ON l.id = c.broadcast_list_id
		WHERE c.studio_id = $1
		ORDER BY c.created_at DESC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list broadcast campaigns: %w", err)
	}
	defer rows.Close()

	out := make([]BroadcastCampaign, 0)
	for rows.Next() {
		c, err := scanBroadcastCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanBroadcastCampaign(row pgx.Row) (BroadcastCampaign, error) {
	var c BroadcastCampaign
	var attsBytes []byte
	if err := row.Scan(&c.ID, &c.StudioID, &c.BroadcastListID, &c.ListName, &c.Channel, &c.Subject, &c.Body, &attsBytes,
		&c.ScheduledFor, &c.Status, &c.TotalCount, &c.EnqueuedCount, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return c, fmt.Errorf("scan broadcast campaign: %w", err)
	}
	if len(attsBytes) > 0 {
		if err := json.Unmarshal(attsBytes, &c.Attachments); err != nil {
			return c, fmt.Errorf("unmarshal campaign attachments: %w", err)
		}
	}
	return c, nil
}

// CancelBroadcastCampaign stops the campaign and also kills its jobs that
// are already queued but not yet sent — otherwise cancelling would only stop
// new recipients being enqueued while the existing ones kept sending/retrying.
func (r *Repo) CancelBroadcastCampaign(ctx context.Context, studioID, campaignID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("cancel broadcast campaign: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE broadcast_campaigns SET status = 'canceled', updated_at = now()
		WHERE id = $1 AND studio_id = $2 AND status IN ('scheduled','sending')
	`, campaignID, studioID)
	if err != nil {
		return fmt.Errorf("cancel broadcast campaign: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
		UPDATE outbound_jobs SET status = 'dead', last_error = 'broadcast campaign canceled'
		WHERE studio_id = $1 AND source_ref = $2 AND status = 'pending'
	`, studioID, fmt.Sprintf("broadcast:%s", campaignID)); err != nil {
		return fmt.Errorf("cancel broadcast campaign jobs: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("cancel broadcast campaign: %w", err)
	}
	return nil
}

// ListActiveBroadcastCampaigns returns every campaign the scheduler worker
// still has work to do on (due and not yet fully enqueued).
func (r *Repo) ListActiveBroadcastCampaigns(ctx context.Context) ([]BroadcastCampaign, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.studio_id, c.broadcast_list_id, l.name, c.channel_kind, c.subject, c.body, c.attachments,
		       c.scheduled_for, c.status, c.total_count, c.enqueued_count, c.created_at, c.updated_at
		FROM broadcast_campaigns c
		JOIN broadcast_lists l ON l.id = c.broadcast_list_id
		WHERE c.status IN ('scheduled','sending') AND c.scheduled_for <= now()
		ORDER BY c.scheduled_for ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list active broadcast campaigns: %w", err)
	}
	defer rows.Close()

	out := make([]BroadcastCampaign, 0)
	for rows.Next() {
		c, err := scanBroadcastCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// NextPendingRecipients returns up to limit pending recipients for a
// campaign, each paired with their contact details — exactly what the
// scheduler needs to enqueue the next slice respecting the daily cap.
type PendingBroadcastRecipient struct {
	RecipientID uuid.UUID
	Contact     BroadcastContact
}

func (r *Repo) NextPendingRecipients(ctx context.Context, campaignID uuid.UUID, limit int) ([]PendingBroadcastRecipient, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.id, c.id, c.broadcast_list_id, c.name, c.phone, c.email, c.created_at
		FROM broadcast_campaign_recipients r
		JOIN broadcast_contacts c ON c.id = r.broadcast_contact_id
		WHERE r.campaign_id = $1 AND r.status = 'pending'
		ORDER BY c.created_at ASC
		LIMIT $2
	`, campaignID, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending recipients: %w", err)
	}
	defer rows.Close()

	out := make([]PendingBroadcastRecipient, 0, limit)
	for rows.Next() {
		var p PendingBroadcastRecipient
		if err := rows.Scan(&p.RecipientID, &p.Contact.ID, &p.Contact.BroadcastListID,
			&p.Contact.Name, &p.Contact.Phone, &p.Contact.Email, &p.Contact.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending recipient: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) MarkRecipientEnqueued(ctx context.Context, recipientID uuid.UUID, outboundJobID int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE broadcast_campaign_recipients
		SET status = 'enqueued', outbound_job_id = $2, enqueued_at = now()
		WHERE id = $1
	`, recipientID, outboundJobID)
	if err != nil {
		return fmt.Errorf("mark recipient enqueued: %w", err)
	}
	return nil
}

func (r *Repo) MarkRecipientFailed(ctx context.Context, recipientID uuid.UUID, reason string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE broadcast_campaign_recipients SET status = 'failed', last_error = $2
		WHERE id = $1
	`, recipientID, reason)
	if err != nil {
		return fmt.Errorf("mark recipient failed: %w", err)
	}
	return nil
}

// AdvanceBroadcastCampaignProgress recomputes enqueued_count and flips the
// campaign to 'sending' (first progress) or 'completed' (no pending
// recipients left — failed ones don't block completion, they're surfaced
// via total_count vs enqueued_count in the UI instead).
func (r *Repo) AdvanceBroadcastCampaignProgress(ctx context.Context, campaignID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE broadcast_campaigns c
		SET enqueued_count = sub.enqueued,
		    status = CASE
		        WHEN sub.pending = 0 THEN 'completed'
		        WHEN sub.enqueued > 0 THEN 'sending'
		        ELSE c.status
		    END,
		    updated_at = now()
		FROM (
		    SELECT
		        count(*) FILTER (WHERE status = 'enqueued') AS enqueued,
		        count(*) FILTER (WHERE status = 'pending') AS pending
		    FROM broadcast_campaign_recipients WHERE campaign_id = $1
		) sub
		WHERE c.id = $1
	`, campaignID)
	if err != nil {
		return fmt.Errorf("advance broadcast campaign progress: %w", err)
	}
	return nil
}

// BroadcastRecipientDetail is one recipient's real delivery state for the
// campaign detail popup — joined through to outbound_jobs for the actual
// sent/failed status and timestamp, since broadcast_campaign_recipients.
// status only tracks "enqueued into outbound_jobs" (see that column's doc
// comment), not delivery itself.
type BroadcastRecipientDetail struct {
	Name   string     `json:"name"`
	Phone  string     `json:"phone"`
	Email  string     `json:"email"`
	Status string     `json:"status"` // "pending" | "sent" | "failed" | "dead"
	SentAt *time.Time `json:"sentAt"`
}

// ListBroadcastCampaignRecipients is studio-scoped via the join to
// broadcast_campaigns, so a campaign ID from another studio returns an
// empty slice rather than another studio's recipient data.
func (r *Repo) ListBroadcastCampaignRecipients(ctx context.Context, studioID, campaignID uuid.UUID) ([]BroadcastRecipientDetail, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT bc.name, bc.phone, bc.email,
		       COALESCE(oj.status, bcr.status) AS status,
		       oj.sent_at
		FROM broadcast_campaign_recipients bcr
		JOIN broadcast_campaigns c ON c.id = bcr.campaign_id AND c.studio_id = $1
		JOIN broadcast_contacts bc ON bc.id = bcr.broadcast_contact_id
		LEFT JOIN outbound_jobs oj ON oj.id = bcr.outbound_job_id
		WHERE bcr.campaign_id = $2
		ORDER BY bc.created_at ASC
	`, studioID, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list broadcast campaign recipients: %w", err)
	}
	defer rows.Close()

	out := make([]BroadcastRecipientDetail, 0)
	for rows.Next() {
		var d BroadcastRecipientDetail
		if err := rows.Scan(&d.Name, &d.Phone, &d.Email, &d.Status, &d.SentAt); err != nil {
			return nil, fmt.Errorf("scan broadcast campaign recipient: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
