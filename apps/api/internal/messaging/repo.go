package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/projectx/api/internal/platform/cache"
	"github.com/projectx/api/internal/platform/secrets"
)

type Repo struct {
	pool      *pgxpool.Pool
	cipher    *secrets.Cipher
	planCache *cache.MemoryCache
}

func NewRepo(pool *pgxpool.Pool, cipher *secrets.Cipher) *Repo {
	return &Repo{pool: pool, cipher: cipher, planCache: cache.New()}
}

func (r *Repo) Pool() *pgxpool.Pool { return r.pool }

// SemanticMatch wraps a Message with the cosine similarity score returned by
// SearchSemanticHistory.
type SemanticMatch struct {
	Message
	Score float32
}

// ============================================================
// channel_accounts
// ============================================================

type CreateChannelInput struct {
	StudioID       uuid.UUID
	Kind           ChannelKind
	BSP            string
	ExternalID     string
	ParentID       string
	DisplayHandle  string
	AccessToken    string // plaintext; encrypted before write
	TokenExpiresAt *time.Time
}

func (r *Repo) CreateChannel(ctx context.Context, in CreateChannelInput) (*ChannelAccount, error) {
	enc, err := r.cipher.Encrypt(in.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("encrypt token: %w", err)
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO channel_accounts
		  (studio_id, kind, bsp, external_id, parent_id, display_handle,
		   access_token_enc, token_expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, status, connected_at, created_at, updated_at
	`, in.StudioID, in.Kind, in.BSP, in.ExternalID, in.ParentID, in.DisplayHandle,
		enc, in.TokenExpiresAt)
	out := &ChannelAccount{
		StudioID:      in.StudioID,
		Kind:          in.Kind,
		BSP:           in.BSP,
		ExternalID:    in.ExternalID,
		ParentID:      in.ParentID,
		DisplayHandle: in.DisplayHandle,
	}
	if err := row.Scan(&out.ID, &out.Status, &out.ConnectedAt, &out.CreatedAt, &out.UpdatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("this %s account is already connected to another studio", in.Kind)
		}
		return nil, fmt.Errorf("insert channel: %w", err)
	}
	return out, nil
}

func (r *Repo) ListChannels(ctx context.Context, studioID uuid.UUID) ([]ChannelAccount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, kind, bsp, external_id, parent_id, display_handle,
		       status, last_error, connected_at, disconnected_at, created_at, updated_at
		FROM channel_accounts
		WHERE studio_id = $1 AND status != 'disconnected'
		ORDER BY created_at DESC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()
	out := make([]ChannelAccount, 0)
	for rows.Next() {
		c := ChannelAccount{StudioID: studioID}
		if err := rows.Scan(&c.ID, &c.Kind, &c.BSP, &c.ExternalID, &c.ParentID, &c.DisplayHandle,
			&c.Status, &c.LastError, &c.ConnectedAt, &c.DisconnectedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetChannelByID returns a channel WITH the decrypted access token. Use for
// outbound dispatching only.
func (r *Repo) GetChannelByID(ctx context.Context, studioID, id uuid.UUID) (*ChannelAccount, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, kind, bsp, external_id, parent_id, display_handle,
		       access_token_enc, status, last_error, connected_at, disconnected_at,
		       created_at, updated_at
		FROM channel_accounts
		WHERE studio_id = $1 AND id = $2
	`, studioID, id)
	return r.scanChannelWithToken(row)
}

// GetChannelByExternalID is the inbound-webhook lookup. Returns the studio-scoped
// channel + decrypted token.
func (r *Repo) GetChannelByExternalID(ctx context.Context, kind ChannelKind, externalID string) (*ChannelAccount, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, kind, bsp, external_id, parent_id, display_handle,
		       access_token_enc, status, last_error, connected_at, disconnected_at,
		       created_at, updated_at
		FROM channel_accounts
		WHERE kind = $1 AND external_id = $2 AND status <> 'disconnected'
		ORDER BY CASE status WHEN 'active' THEN 0 ELSE 1 END, connected_at DESC
		LIMIT 1
	`, kind, externalID)
	return r.scanChannelWithToken(row)
}

// GetActiveChannelByStudio returns the most recently connected active channel
// for a studio. The inbox uses this to start a new conversation from the UI.
func (r *Repo) GetActiveChannelByStudio(ctx context.Context, studioID uuid.UUID) (*ChannelAccount, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, kind, bsp, external_id, parent_id, display_handle,
		       access_token_enc, status, last_error, connected_at, disconnected_at,
		       created_at, updated_at
		FROM channel_accounts
		WHERE studio_id = $1 AND status = 'active'
		ORDER BY connected_at DESC
		LIMIT 1
	`, studioID)
	return r.scanChannelWithToken(row)
}

// GetActiveChannelByKind returns the most recently connected active channel
// of a specific kind for a studio. Used by the outbound worker as a fallback.
func (r *Repo) GetActiveChannelByKind(ctx context.Context, studioID uuid.UUID, kind ChannelKind) (*ChannelAccount, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, kind, bsp, external_id, parent_id, display_handle,
		       access_token_enc, status, last_error, connected_at, disconnected_at,
		       created_at, updated_at
		FROM channel_accounts
		WHERE studio_id = $1 AND kind = $2 AND status = 'active'
		ORDER BY connected_at DESC
		LIMIT 1
	`, studioID, kind)
	return r.scanChannelWithToken(row)
}

func (r *Repo) scanChannelWithToken(row pgx.Row) (*ChannelAccount, error) {
	var c ChannelAccount
	var encToken string
	if err := row.Scan(&c.ID, &c.StudioID, &c.Kind, &c.BSP, &c.ExternalID, &c.ParentID, &c.DisplayHandle,
		&encToken, &c.Status, &c.LastError, &c.ConnectedAt, &c.DisconnectedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan channel: %w", err)
	}
	tok, err := r.cipher.Decrypt(encToken)
	if err != nil {
		return nil, fmt.Errorf("decrypt token: %w", err)
	}
	c.AccessToken = tok
	return &c, nil
}

func (r *Repo) DisconnectChannel(ctx context.Context, studioID, id uuid.UUID) error {
	// First try to hard delete the channel (will succeed if no conversations exist)
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM channel_accounts
		WHERE studio_id = $1 AND id = $2
	`, studioID, id)
	if err == nil {
		if tag.RowsAffected() > 0 {
			return nil
		}
		return ErrNotFound
	}

	// If it fails with a foreign key constraint violation (code 23503), fall back to soft-delete
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		tag, err = r.pool.Exec(ctx, `
			UPDATE channel_accounts
			SET status = 'disconnected', disconnected_at = now(), updated_at = now()
			WHERE studio_id = $1 AND id = $2
		`, studioID, id)
		if err != nil {
			return fmt.Errorf("disconnect: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	}

	return fmt.Errorf("delete channel: %w", err)
}

type UpdateChannelInput struct {
	ID            uuid.UUID
	StudioID      uuid.UUID
	ExternalID    string
	ParentID      string
	DisplayHandle string
	AccessToken   string
}

func (r *Repo) UpdateChannel(ctx context.Context, in UpdateChannelInput) (*ChannelAccount, error) {
	var enc string
	var err error
	if in.AccessToken != "" {
		enc, err = r.cipher.Encrypt(in.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("encrypt token: %w", err)
		}
	}

	var row pgx.Row
	if in.AccessToken != "" {
		row = r.pool.QueryRow(ctx, `
			UPDATE channel_accounts
			SET external_id = $3, parent_id = $4, display_handle = $5,
			    access_token_enc = $6, status = 'active', last_error = '', updated_at = now()
			WHERE studio_id = $1 AND id = $2
			RETURNING id, studio_id, kind, bsp, external_id, parent_id, display_handle, status, last_error, connected_at, disconnected_at, created_at, updated_at
		`, in.StudioID, in.ID, in.ExternalID, in.ParentID, in.DisplayHandle, enc)
	} else {
		row = r.pool.QueryRow(ctx, `
			UPDATE channel_accounts
			SET external_id = $3, parent_id = $4, display_handle = $5,
			    status = 'active', last_error = '', updated_at = now()
			WHERE studio_id = $1 AND id = $2
			RETURNING id, studio_id, kind, bsp, external_id, parent_id, display_handle, status, last_error, connected_at, disconnected_at, created_at, updated_at
		`, in.StudioID, in.ID, in.ExternalID, in.ParentID, in.DisplayHandle)
	}

	out := &ChannelAccount{}
	if err := row.Scan(&out.ID, &out.StudioID, &out.Kind, &out.BSP, &out.ExternalID, &out.ParentID, &out.DisplayHandle,
		&out.Status, &out.LastError, &out.ConnectedAt, &out.DisconnectedAt, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("this %s account is already connected to another studio", out.Kind)
		}
		return nil, fmt.Errorf("update channel: %w", err)
	}
	return out, nil
}

func (r *Repo) MarkChannelError(ctx context.Context, id uuid.UUID, msg string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE channel_accounts SET status = 'error', last_error = $2, updated_at = now()
		WHERE id = $1
	`, id, msg)
	return err
}

// UpsertWAWebChannel creates or re-activates the whatsapp_web channel for a studio.
// Called when the Baileys session connects successfully after QR scan.
// phone is the linked WhatsApp number (e.g. "6512345678").
func (r *Repo) UpsertWAWebChannel(ctx context.Context, studioID uuid.UUID, phone string) error {
	placeholder, _ := r.cipher.Encrypt("wa-web-baileys")

	// Try to re-activate an existing row for this studio first.
	tag, err := r.pool.Exec(ctx, `
		UPDATE channel_accounts
		SET status = 'active', display_handle = $2, external_id = $2,
		    connected_at = now(), disconnected_at = NULL, updated_at = now()
		WHERE studio_id = $1 AND kind = 'whatsapp_web'
	`, studioID, phone)
	if err != nil {
		return fmt.Errorf("upsert wa_web channel (update): %w", err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	// No existing row — insert fresh.
	_, err = r.pool.Exec(ctx, `
		INSERT INTO channel_accounts
		  (studio_id, kind, bsp, external_id, parent_id, display_handle,
		   access_token_enc, status, connected_at, updated_at)
		VALUES ($1, 'whatsapp_web', 'baileys', $2, '', $2, $3, 'active', now(), now())
	`, studioID, phone, placeholder)
	if err != nil {
		return fmt.Errorf("upsert wa_web channel (insert): %w", err)
	}
	return nil
}

// ListWAWebStudioIDs returns all studio IDs that have a whatsapp_web channel (any status).
func (r *Repo) ListWAWebStudioIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT studio_id FROM channel_accounts
		WHERE kind = 'whatsapp_web'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DisconnectWAWebChannel marks any active whatsapp_web channel for a studio as disconnected.
func (r *Repo) DisconnectWAWebChannel(ctx context.Context, studioID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE channel_accounts
		SET status = 'disconnected', disconnected_at = now(), updated_at = now()
		WHERE studio_id = $1 AND kind = 'whatsapp_web' AND status <> 'disconnected'
	`, studioID)
	return err
}

// SetWAWebBackfillStatus records history-import progress for a studio's
// whatsapp_web channel so the admin UI can poll it (running/done/failed).
func (r *Repo) SetWAWebBackfillStatus(ctx context.Context, studioID uuid.UUID, status string, messageCount int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE channel_accounts
		SET backfill_status = $2, backfill_message_count = $3, backfill_updated_at = now()
		WHERE studio_id = $1 AND kind = 'whatsapp_web'
	`, studioID, status, messageCount)
	return err
}

// GetWAWebBackfillStatus reports the current history-import status for a studio.
func (r *Repo) GetWAWebBackfillStatus(ctx context.Context, studioID uuid.UUID) (status string, messageCount int, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT backfill_status, backfill_message_count
		FROM channel_accounts
		WHERE studio_id = $1 AND kind = 'whatsapp_web'
		ORDER BY connected_at DESC
		LIMIT 1
	`, studioID).Scan(&status, &messageCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return "none", 0, nil
	}
	return status, messageCount, err
}

// ============================================================
// contact_identities
// ============================================================

// FindOrCreateIdentity is the heart of identity stitching: same kind+value
// resolves to the same row, attached to a lead if/when one exists.
func (r *Repo) FindOrCreateIdentity(ctx context.Context, tx pgx.Tx, studioID uuid.UUID, kind IdentityKind, value, displayName string) (*ContactIdentity, error) {
	row := tx.QueryRow(ctx, `
		INSERT INTO contact_identities (studio_id, kind, value, display_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (studio_id, kind, value) DO UPDATE
		  SET display_name = CASE
		    WHEN EXCLUDED.display_name <> '' THEN EXCLUDED.display_name
		    ELSE contact_identities.display_name
		  END,
		  updated_at = now()
		RETURNING id, lead_id, display_name, created_at
	`, studioID, kind, value, displayName)
	out := &ContactIdentity{StudioID: studioID, Kind: kind, Value: value}
	if err := row.Scan(&out.ID, &out.LeadID, &out.DisplayName, &out.CreatedAt); err != nil {
		return nil, fmt.Errorf("upsert identity: %w", err)
	}
	return out, nil
}

// ============================================================
// conversations
// ============================================================

// FindOrCreateConversation: same channel + same external thread = one row.
func (r *Repo) FindOrCreateConversation(ctx context.Context, tx pgx.Tx, studioID, channelID, identityID uuid.UUID, externalThreadID string) (*Conversation, error) {
	row := tx.QueryRow(ctx, `
		INSERT INTO conversations
		  (studio_id, channel_account_id, contact_identity_id, external_thread_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (channel_account_id, external_thread_id) DO UPDATE
		  SET updated_at = now()
		RETURNING id, status, lead_id, assigned_to, unread_count,
		          last_message_at, last_message_preview, last_message_direction,
		          created_at, updated_at
	`, studioID, channelID, identityID, externalThreadID)
	out := &Conversation{
		StudioID:          studioID,
		ChannelAccountID:  channelID,
		ContactIdentityID: identityID,
		ExternalThreadID:  externalThreadID,
	}
	var dir *string
	if err := row.Scan(&out.ID, &out.Status, &out.LeadID, &out.AssignedTo, &out.UnreadCount,
		&out.LastMessageAt, &out.LastMessagePreview, &dir,
		&out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, fmt.Errorf("upsert conversation: %w", err)
	}
	if dir != nil {
		d := Direction(*dir)
		out.LastMessageDirection = &d
	}
	return out, nil
}

type ListConversationsFilter struct {
	Status      *ConvStatus
	ChannelKind *ChannelKind
	Limit       int
	Offset      int
}

func (r *Repo) ListConversations(ctx context.Context, studioID uuid.UUID, f ListConversationsFilter) ([]Conversation, int, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 25
	}
	args := []any{studioID}
	cond := "c.studio_id = $1"
	if f.Status != nil {
		args = append(args, *f.Status)
		cond += fmt.Sprintf(" AND c.status = $%d", len(args))
	}
	if f.ChannelKind != nil {
		args = append(args, *f.ChannelKind)
		cond += fmt.Sprintf(" AND ch.kind = $%d", len(args))
	}

	var total int
	countQ := `SELECT COUNT(*) FROM conversations c JOIN channel_accounts ch ON ch.id = c.channel_account_id WHERE ` + cond
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count conversations: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	q := `
		SELECT c.id, c.studio_id, c.channel_account_id, ch.kind, ch.display_handle,
		       c.contact_identity_id, ci.display_name, ci.value, c.external_thread_id,
		       c.lead_id, c.status, c.assigned_to, c.unread_count,
		       c.last_message_at, c.last_message_preview, c.last_message_direction,
		       c.created_at, c.updated_at
		FROM conversations c
		JOIN channel_accounts ch ON ch.id = c.channel_account_id
		JOIN contact_identities ci ON ci.id = c.contact_identity_id
		WHERE ` + cond + `
		ORDER BY c.last_message_at DESC
		LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	out := make([]Conversation, 0)
	for rows.Next() {
		c, err := scanConversationRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *c)
	}
	return out, total, rows.Err()
}

func (r *Repo) GetConversation(ctx context.Context, studioID, id uuid.UUID) (*Conversation, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT c.id, c.studio_id, c.channel_account_id, ch.kind, ch.display_handle,
		       c.contact_identity_id, ci.display_name, ci.value, c.external_thread_id,
		       c.lead_id, c.status, c.assigned_to, c.unread_count,
		       c.last_message_at, c.last_message_preview, c.last_message_direction,
		       c.created_at, c.updated_at
		FROM conversations c
		JOIN channel_accounts ch ON ch.id = c.channel_account_id
		JOIN contact_identities ci ON ci.id = c.contact_identity_id
		WHERE c.studio_id = $1 AND c.id = $2
	`, studioID, id)
	return scanConversationRow(row)
}

func scanConversationRow(row pgx.Row) (*Conversation, error) {
	var c Conversation
	var dir *string
	if err := row.Scan(&c.ID, &c.StudioID, &c.ChannelAccountID, &c.ChannelKind, &c.ChannelHandle,
		&c.ContactIdentityID, &c.ContactDisplayName, &c.ContactValue, &c.ExternalThreadID,
		&c.LeadID, &c.Status, &c.AssignedTo, &c.UnreadCount,
		&c.LastMessageAt, &c.LastMessagePreview, &dir,
		&c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan conversation: %w", err)
	}
	if dir != nil {
		d := Direction(*dir)
		c.LastMessageDirection = &d
	}
	return &c, nil
}

// ListConversationIDsByChannel returns every conversation ID for a studio's
// channel account, unpaginated — used by the post-backfill summarization
// pass, which needs to walk every conversation once rather than page through
// ListConversations' admin-facing 100-row cap.
func (r *Repo) ListConversationIDsByChannel(ctx context.Context, studioID, channelAccountID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id FROM conversations
		WHERE studio_id = $1 AND channel_account_id = $2
	`, studioID, channelAccountID)
	if err != nil {
		return nil, fmt.Errorf("list conversation ids: %w", err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SetConversationAISummary stores an internal-only AI-generated summary of a
// conversation's recent history, used as extra context for the AI auto-reply
// worker. Never exposed in admin-facing JSON responses.
func (r *Repo) SetConversationAISummary(ctx context.Context, studioID, convID uuid.UUID, summary string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE conversations
		SET ai_context_summary = $3, ai_context_summary_updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, studioID, convID, summary)
	return err
}

// GetConversationAISummary returns the stored internal AI summary for a
// conversation, or "" if none has been generated yet.
func (r *Repo) GetConversationAISummary(ctx context.Context, studioID, convID uuid.UUID) (string, error) {
	var summary string
	err := r.pool.QueryRow(ctx, `
		SELECT ai_context_summary FROM conversations WHERE studio_id = $1 AND id = $2
	`, studioID, convID).Scan(&summary)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return summary, err
}

func (r *Repo) MarkConversationRead(ctx context.Context, studioID, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE conversations SET unread_count = 0, updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, studioID, id)
	return err
}

// ============================================================
// messages
// ============================================================

type CreateMessageInput struct {
	ConversationID uuid.UUID
	StudioID       uuid.UUID
	Direction      Direction
	SourceKind     SourceKind
	SourceUserID   *uuid.UUID
	SourceRef      string
	Body           string
	Attachments    []Attachment
	ExternalID     string
	InReplyTo      string
	Status         MessageStatus
	SentAt         time.Time
}

// InsertMessage persists a message, dedupes by (conversation_id, external_id),
// and updates the conversation's last-message metadata + unread counter (for
// inbound). Runs inside the caller's transaction so the conversation snapshot
// stays consistent.
func (r *Repo) InsertMessage(ctx context.Context, tx pgx.Tx, in CreateMessageInput) (*Message, error) {
	attsBytes, err := json.Marshal(in.Attachments)
	if err != nil {
		return nil, fmt.Errorf("marshal attachments: %w", err)
	}
	atts := string(attsBytes)
	if in.SentAt.IsZero() {
		in.SentAt = time.Now().UTC()
	}
	if in.Status == "" {
		in.Status = MsgSent
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, studio_id, direction, source_kind,
		                      source_user_id, source_ref, body, attachments,
		                      external_id, in_reply_to, status, sent_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (conversation_id, external_id) DO NOTHING
		RETURNING id, created_at
	`, in.ConversationID, in.StudioID, in.Direction, in.SourceKind,
		in.SourceUserID, in.SourceRef, in.Body, atts,
		nullIfEmpty(in.ExternalID), nullIfEmpty(in.InReplyTo), in.Status, in.SentAt)

	out := &Message{
		ConversationID: in.ConversationID,
		StudioID:       in.StudioID,
		Direction:      in.Direction,
		SourceKind:     in.SourceKind,
		SourceUserID:   in.SourceUserID,
		SourceRef:      in.SourceRef,
		Body:           in.Body,
		Attachments:    in.Attachments,
		ExternalID:     in.ExternalID,
		InReplyTo:      in.InReplyTo,
		Status:         in.Status,
		SentAt:         in.SentAt,
	}
	if err := row.Scan(&out.ID, &out.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Duplicate inbound (Meta retry). Caller treats as no-op.
			return nil, nil
		}
		return nil, fmt.Errorf("insert message: %w", err)
	}

	// Update conversation snapshot.
	preview := in.Body
	if len(preview) > 120 {
		preview = preview[:120]
	}
	bumpUnread := 0
	if in.Direction == DirectionInbound {
		bumpUnread = 1
	}
	if _, err := tx.Exec(ctx, `
		UPDATE conversations
		SET last_message_at = $2,
		    last_message_preview = $3,
		    last_message_direction = $4,
		    unread_count = unread_count + $5,
		    updated_at = now()
		WHERE id = $1
	`, in.ConversationID, in.SentAt, preview, in.Direction, bumpUnread); err != nil {
		return nil, fmt.Errorf("bump conversation: %w", err)
	}

	return out, nil
}

// InsertMessageBackfill persists a historical message imported from a WhatsApp
// Web chat backfill. It shares InsertMessage's dedupe key (conversation_id,
// external_id) but deliberately never bumps unread_count and only advances
// the conversation's last-message snapshot if the backfilled message is
// actually newer than what's already there (GREATEST-guarded) — so importing
// old history can never regress or spam unread counts for a conversation
// that's already had live traffic.
func (r *Repo) InsertMessageBackfill(ctx context.Context, tx pgx.Tx, in CreateMessageInput) (*Message, error) {
	attsBytes, err := json.Marshal(in.Attachments)
	if err != nil {
		return nil, fmt.Errorf("marshal attachments: %w", err)
	}
	atts := string(attsBytes)
	if in.SentAt.IsZero() {
		in.SentAt = time.Now().UTC()
	}
	if in.Status == "" {
		in.Status = MsgSent
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, studio_id, direction, source_kind,
		                      source_user_id, source_ref, body, attachments,
		                      external_id, in_reply_to, status, sent_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (conversation_id, external_id) DO NOTHING
		RETURNING id, created_at
	`, in.ConversationID, in.StudioID, in.Direction, in.SourceKind,
		in.SourceUserID, in.SourceRef, in.Body, atts,
		nullIfEmpty(in.ExternalID), nullIfEmpty(in.InReplyTo), in.Status, in.SentAt)

	out := &Message{
		ConversationID: in.ConversationID,
		StudioID:       in.StudioID,
		Direction:      in.Direction,
		SourceKind:     in.SourceKind,
		SourceUserID:   in.SourceUserID,
		SourceRef:      in.SourceRef,
		Body:           in.Body,
		Attachments:    in.Attachments,
		ExternalID:     in.ExternalID,
		InReplyTo:      in.InReplyTo,
		Status:         in.Status,
		SentAt:         in.SentAt,
	}
	if err := row.Scan(&out.ID, &out.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Already imported or already seen live — no-op.
			return nil, nil
		}
		return nil, fmt.Errorf("insert backfill message: %w", err)
	}

	preview := in.Body
	if len(preview) > 120 {
		preview = preview[:120]
	}
	if _, err := tx.Exec(ctx, `
		UPDATE conversations
		SET last_message_at = GREATEST(last_message_at, $2),
		    last_message_preview = CASE WHEN $2 >= last_message_at THEN $3 ELSE last_message_preview END,
		    last_message_direction = CASE WHEN $2 >= last_message_at THEN $4 ELSE last_message_direction END,
		    updated_at = now()
		WHERE id = $1
	`, in.ConversationID, in.SentAt, preview, in.Direction); err != nil {
		return nil, fmt.Errorf("bump conversation (backfill): %w", err)
	}

	return out, nil
}

func (r *Repo) ListMessages(ctx context.Context, studioID, conversationID uuid.UUID, limit int) ([]Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT sub.id, sub.conversation_id, sub.studio_id, sub.direction, sub.source_kind, sub.source_user_id,
		       sub.source_ref, sub.body, sub.attachments, sub.external_id, sub.in_reply_to, sub.status,
		       sub.failure_reason, sub.sent_at, sub.delivered_at, sub.read_at, sub.created_at
		FROM (
			SELECT id, conversation_id, studio_id, direction, source_kind, source_user_id,
			       source_ref, body, attachments, external_id, in_reply_to, status,
			       failure_reason, sent_at, delivered_at, read_at, created_at
			FROM messages
			WHERE studio_id = $1 AND conversation_id = $2
			ORDER BY sent_at DESC, created_at DESC
			LIMIT $3
		) sub
		ORDER BY sub.sent_at ASC, sub.created_at ASC
	`, studioID, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	out := make([]Message, 0)
	for rows.Next() {
		var m Message
		var atts []byte
		var srcRef, externalID, inReplyTo *string
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.StudioID, &m.Direction, &m.SourceKind,
			&m.SourceUserID, &srcRef, &m.Body, &atts, &externalID, &inReplyTo, &m.Status,
			&m.FailureReason, &m.SentAt, &m.DeliveredAt, &m.ReadAt, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		if srcRef != nil {
			m.SourceRef = *srcRef
		}
		if externalID != nil {
			m.ExternalID = *externalID
		}
		if inReplyTo != nil {
			m.InReplyTo = *inReplyTo
		}
		if len(atts) > 0 {
			_ = json.Unmarshal(atts, &m.Attachments)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repo) GetMessageByID(ctx context.Context, studioID, id uuid.UUID) (*Message, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, conversation_id, studio_id, direction, source_kind, source_user_id,
			   source_ref, body, attachments, external_id, in_reply_to, status,
			   failure_reason, sent_at, delivered_at, read_at, created_at
		FROM messages
		WHERE studio_id = $1 AND id = $2
	`, studioID, id)
	var m Message
	var atts []byte
	var srcRef, externalID, inReplyTo *string
	if err := row.Scan(&m.ID, &m.ConversationID, &m.StudioID, &m.Direction, &m.SourceKind,
		&m.SourceUserID, &srcRef, &m.Body, &atts, &externalID, &inReplyTo, &m.Status,
		&m.FailureReason, &m.SentAt, &m.DeliveredAt, &m.ReadAt, &m.CreatedAt); err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	if srcRef != nil {
		m.SourceRef = *srcRef
	}
	if externalID != nil {
		m.ExternalID = *externalID
	}
	if inReplyTo != nil {
		m.InReplyTo = *inReplyTo
	}
	if len(atts) > 0 {
		_ = json.Unmarshal(atts, &m.Attachments)
	}
	return &m, nil
}

// SearchSemanticHistory finds the top-K messages from a conversation that are
// semantically similar to the query embedding. Uses the pre-computed HNSW index
// on messages.embedding — much faster than real-time embedding of each message.
// Returns each match together with its cosine similarity score (0–1, higher = more similar).
func (r *Repo) SearchSemanticHistory(ctx context.Context, studioID, conversationID uuid.UUID, queryEmbedding []float32, excludeIDs []uuid.UUID, limit int) ([]SemanticMatch, error) {
	embStr := formatVec(queryEmbedding)

	rows, err := r.pool.Query(ctx, `
		SELECT id, conversation_id, studio_id, direction, source_kind, source_user_id,
		       source_ref, body, attachments, external_id, in_reply_to, status,
		       failure_reason, sent_at, delivered_at, read_at, created_at,
		       1 - (embedding <=> $4::vector) AS similarity_score
		FROM messages
		WHERE studio_id = $1
		  AND conversation_id = $2
		  AND embedding IS NOT NULL
		  AND body != ''
		  AND id != ALL($3)
		ORDER BY embedding <=> $4::vector
		LIMIT $5
	`, studioID, conversationID, excludeIDs, embStr, limit)
	if err != nil {
		return nil, fmt.Errorf("semantic history search: %w", err)
	}
	defer rows.Close()

	var out []SemanticMatch
	for rows.Next() {
		var sm SemanticMatch
		var atts []byte
		var srcRef, externalID, inReplyTo *string
		if err := rows.Scan(
			&sm.ID, &sm.ConversationID, &sm.StudioID, &sm.Direction, &sm.SourceKind,
			&sm.SourceUserID, &srcRef, &sm.Body, &atts, &externalID, &inReplyTo, &sm.Status,
			&sm.FailureReason, &sm.SentAt, &sm.DeliveredAt, &sm.ReadAt, &sm.CreatedAt,
			&sm.Score,
		); err != nil {
			return nil, fmt.Errorf("scan semantic history: %w", err)
		}
		if srcRef != nil {
			sm.SourceRef = *srcRef
		}
		if externalID != nil {
			sm.ExternalID = *externalID
		}
		if inReplyTo != nil {
			sm.InReplyTo = *inReplyTo
		}
		if len(atts) > 0 {
			_ = json.Unmarshal(atts, &sm.Attachments)
		}
		out = append(out, sm)
	}
	return out, rows.Err()
}

// SaveMessageEmbedding persists a pre-computed embedding and LLM-classified intent/sentiment
// on an existing message row. Called asynchronously after the message is inserted.
func (r *Repo) SaveMessageEmbedding(ctx context.Context, messageID uuid.UUID, embedding []float32, intent string, sentiment int, confidence float32) error {
	embStr := formatVec(embedding)
	_, err := r.pool.Exec(ctx, `
		UPDATE messages
		SET embedding = $2::vector,
		    intent = $3,
		    sentiment = $4,
		    sentiment_confidence = $5
		WHERE id = $1
	`, messageID, embStr, intent, sentiment, confidence)
	return err
}

// formatVec serialises []float32 → "[v1,v2,...]" for pgvector.
func formatVec(vec []float32) string {
	if len(vec) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", v)
	}
	sb.WriteByte(']')
	return sb.String()
}

// ============================================================
// outbound_jobs
// ============================================================

func (r *Repo) EnqueueOutbound(ctx context.Context, j OutboundJob) (int64, error) {
	attsBytes, _ := json.Marshal(j.Attachments)
	atts := string(attsBytes)
	if j.ScheduledFor.IsZero() {
		j.ScheduledFor = time.Now().UTC()
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO outbound_jobs (studio_id, conversation_id, body, attachments,
		                           template_name, source_kind, source_user_id, source_ref,
		                           scheduled_for, next_attempt_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)
		RETURNING id
	`, j.StudioID, j.ConversationID, j.Body, atts,
		nullIfEmpty(j.TemplateName), j.SourceKind, j.SourceUserID, nullIfEmpty(j.SourceRef),
		j.ScheduledFor)
	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, fmt.Errorf("enqueue: %w", err)
	}
	return id, nil
}

// ClaimOutboundBatch atomically reserves a batch of pending jobs whose
// scheduled_for is due. Uses FOR UPDATE SKIP LOCKED to support multiple workers.
func (r *Repo) ClaimOutboundBatch(ctx context.Context, n int) ([]OutboundJob, error) {
	rows, err := r.pool.Query(ctx, `
		WITH picked AS (
			SELECT id FROM outbound_jobs
			WHERE status = 'pending' AND next_attempt_at <= now()
			ORDER BY id
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbound_jobs o
		SET next_attempt_at = now() + INTERVAL '1 minute'  -- soft re-queue if worker dies
		FROM picked
		WHERE o.id = picked.id
		RETURNING o.id, o.studio_id, o.conversation_id, o.body, o.attachments,
		          o.template_name, o.source_kind, o.source_user_id, o.source_ref,
		          o.scheduled_for, o.attempts
	`, n)
	if err != nil {
		return nil, fmt.Errorf("claim outbound: %w", err)
	}
	defer rows.Close()
	out := make([]OutboundJob, 0)
	for rows.Next() {
		var j OutboundJob
		var atts []byte
		var tpl, srcRef *string
		if err := rows.Scan(&j.ID, &j.StudioID, &j.ConversationID, &j.Body, &atts,
			&tpl, &j.SourceKind, &j.SourceUserID, &srcRef,
			&j.ScheduledFor, &j.Attempts); err != nil {
			return nil, fmt.Errorf("scan outbound: %w", err)
		}
		if tpl != nil {
			j.TemplateName = *tpl
		}
		if srcRef != nil {
			j.SourceRef = *srcRef
		}
		if len(atts) > 0 {
			_ = json.Unmarshal(atts, &j.Attachments)
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *Repo) MarkOutboundSent(ctx context.Context, id int64, messageID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE outbound_jobs
		SET status = 'sent', sent_at = now(), message_id = $2, last_error = ''
		WHERE id = $1
	`, id, messageID)
	return err
}

func (r *Repo) MarkOutboundFailed(ctx context.Context, id int64, errMsg string, backoff time.Duration, dead bool) error {
	status := "pending"
	if dead {
		status = "dead"
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE outbound_jobs
		SET attempts = attempts + 1,
		    next_attempt_at = now() + ($3 * INTERVAL '1 second'),
		    last_error = $2,
		    status = $4
		WHERE id = $1
	`, id, errMsg, backoff.Seconds(), status)
	return err
}

// ----- helpers -----

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ============================================================
// message_templates
// ============================================================

func (r *Repo) ListTemplates(ctx context.Context, studioID uuid.UUID) ([]MessageTemplate, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, studio_id, name, body, channel_kinds, attachments,
		       whatsapp_template_name, whatsapp_template_lang, created_at, updated_at
		FROM message_templates
		WHERE studio_id = $1
		ORDER BY name ASC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	out := make([]MessageTemplate, 0)
	for rows.Next() {
		var mt MessageTemplate
		var atts []byte
		var waName, waLang *string
		if err := rows.Scan(&mt.ID, &mt.StudioID, &mt.Name, &mt.Body, &mt.ChannelKinds, &atts,
			&waName, &waLang, &mt.CreatedAt, &mt.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan template: %w", err)
		}
		if waName != nil {
			mt.WhatsAppTemplateName = *waName
		}
		if waLang != nil {
			mt.WhatsAppTemplateLang = *waLang
		}
		if len(atts) > 0 {
			_ = json.Unmarshal(atts, &mt.Attachments)
		}
		out = append(out, mt)
	}
	return out, rows.Err()
}

func (r *Repo) CreateTemplate(ctx context.Context, mt *MessageTemplate) error {
	attsBytes, _ := json.Marshal(mt.Attachments)
	atts := string(attsBytes)
	row := r.pool.QueryRow(ctx, `
		INSERT INTO message_templates (studio_id, name, body, channel_kinds, attachments,
		                               whatsapp_template_name, whatsapp_template_lang)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`, mt.StudioID, mt.Name, mt.Body, mt.ChannelKinds, atts,
		nullIfEmpty(mt.WhatsAppTemplateName), nullIfEmpty(mt.WhatsAppTemplateLang))
	return row.Scan(&mt.ID, &mt.CreatedAt, &mt.UpdatedAt)
}

func (r *Repo) UpdateTemplate(ctx context.Context, mt *MessageTemplate) error {
	attsBytes, _ := json.Marshal(mt.Attachments)
	atts := string(attsBytes)
	tag, err := r.pool.Exec(ctx, `
		UPDATE message_templates
		SET name = $3, body = $4, channel_kinds = $5, attachments = $6,
		    whatsapp_template_name = $7, whatsapp_template_lang = $8, updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, mt.StudioID, mt.ID, mt.Name, mt.Body, mt.ChannelKinds, atts,
		nullIfEmpty(mt.WhatsAppTemplateName), nullIfEmpty(mt.WhatsAppTemplateLang))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) DeleteTemplate(ctx context.Context, studioID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM message_templates
		WHERE studio_id = $1 AND id = $2
	`, studioID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================
// trigger_links
// ============================================================

func (r *Repo) ListTriggerLinks(ctx context.Context, studioID uuid.UUID) ([]TriggerLink, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tl.id, tl.studio_id, tl.name, tl.url, 
		       COALESCE(COUNT(tlc.id), 0)::int AS clicks,
		       tl.created_at, tl.updated_at
		FROM trigger_links tl
		LEFT JOIN trigger_link_clicks tlc ON tlc.link_id = tl.id
		WHERE tl.studio_id = $1
		GROUP BY tl.id
		ORDER BY tl.name ASC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list trigger links: %w", err)
	}
	defer rows.Close()

	out := make([]TriggerLink, 0)
	for rows.Next() {
		var tl TriggerLink
		if err := rows.Scan(&tl.ID, &tl.StudioID, &tl.Name, &tl.URL, &tl.Clicks, &tl.CreatedAt, &tl.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan trigger link: %w", err)
		}
		out = append(out, tl)
	}
	return out, rows.Err()
}

func (r *Repo) CreateTriggerLink(ctx context.Context, tl *TriggerLink) error {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO trigger_links (studio_id, name, url)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`, tl.StudioID, tl.Name, tl.URL)
	return row.Scan(&tl.ID, &tl.CreatedAt, &tl.UpdatedAt)
}

func (r *Repo) UpdateTriggerLink(ctx context.Context, tl *TriggerLink) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE trigger_links
		SET name = $3, url = $4, updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, tl.StudioID, tl.ID, tl.Name, tl.URL)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) DeleteTriggerLink(ctx context.Context, studioID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM trigger_links
		WHERE studio_id = $1 AND id = $2
	`, studioID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) GetTriggerLinkByID(ctx context.Context, id uuid.UUID) (*TriggerLink, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, name, url, created_at, updated_at
		FROM trigger_links
		WHERE id = $1
	`, id)
	var tl TriggerLink
	if err := row.Scan(&tl.ID, &tl.StudioID, &tl.Name, &tl.URL, &tl.CreatedAt, &tl.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &tl, nil
}

func (r *Repo) RecordTriggerLinkClick(ctx context.Context, linkID uuid.UUID, leadID *uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO trigger_link_clicks (link_id, lead_id)
		VALUES ($1, $2)
	`, linkID, leadID)
	return err
}

// ============================================================
// outbound_jobs / pending automated actions
// ============================================================

type PendingJobInfo struct {
	ID                 int64        `json:"id"`
	StudioID           uuid.UUID    `json:"studioId"`
	ConversationID     uuid.UUID    `json:"conversationId"`
	ContactDisplayName string       `json:"contactDisplayName"`
	ContactValue       string       `json:"contactValue"`
	ChannelKind        string       `json:"channelKind"`
	Body               string       `json:"body"`
	Attachments        []Attachment `json:"attachments"`
	ScheduledFor       time.Time    `json:"scheduledFor"`
	Attempts           int          `json:"attempts"`
	Status             string       `json:"status"`
}

func (r *Repo) ListPendingJobs(ctx context.Context, studioID uuid.UUID) ([]PendingJobInfo, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT o.id, o.studio_id, o.conversation_id, ci.display_name, ci.value, ch.kind,
		       o.body, o.attachments, o.scheduled_for, o.attempts, o.status
		FROM outbound_jobs o
		JOIN conversations c ON c.id = o.conversation_id
		JOIN channel_accounts ch ON ch.id = c.channel_account_id
		JOIN contact_identities ci ON ci.id = c.contact_identity_id
		WHERE o.studio_id = $1 AND o.status = 'pending'
		ORDER BY o.scheduled_for ASC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list pending jobs: %w", err)
	}
	defer rows.Close()

	out := make([]PendingJobInfo, 0)
	for rows.Next() {
		var ji PendingJobInfo
		var atts []byte
		if err := rows.Scan(&ji.ID, &ji.StudioID, &ji.ConversationID, &ji.ContactDisplayName,
			&ji.ContactValue, &ji.ChannelKind, &ji.Body, &atts, &ji.ScheduledFor,
			&ji.Attempts, &ji.Status); err != nil {
			return nil, fmt.Errorf("scan pending job: %w", err)
		}
		if len(atts) > 0 {
			_ = json.Unmarshal(atts, &ji.Attachments)
		}
		out = append(out, ji)
	}
	return out, rows.Err()
}

// CancelPendingJobsForConversation deletes every still-pending outbound job
// for a conversation (e.g. queued autocontact follow-ups) so a lead who has
// opted out doesn't receive any further scheduled messages. Returns the
// number of jobs removed.
func (r *Repo) CancelPendingJobsForConversation(ctx context.Context, studioID, conversationID uuid.UUID) (int, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM outbound_jobs
		WHERE studio_id = $1 AND conversation_id = $2 AND status = 'pending'
	`, studioID, conversationID)
	if err != nil {
		return 0, fmt.Errorf("cancel pending jobs: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// CancelPendingJobsForLead deletes every still-pending outbound job across
// ALL of a lead's conversations (a lead may have more than one channel).
// Used when Do Not Disturb is turned on. Returns the number of jobs removed.
func (r *Repo) CancelPendingJobsForLead(ctx context.Context, studioID, leadID uuid.UUID) (int, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM outbound_jobs o
		USING conversations c
		WHERE o.conversation_id = c.id
		  AND o.studio_id = $1 AND c.lead_id = $2 AND o.status = 'pending'
	`, studioID, leadID)
	if err != nil {
		return 0, fmt.Errorf("cancel pending jobs for lead: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (r *Repo) DeleteJob(ctx context.Context, studioID uuid.UUID, id int64) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM outbound_jobs
		WHERE studio_id = $1 AND id = $2
	`, studioID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) UpdateJob(ctx context.Context, studioID uuid.UUID, id int64, body string, scheduledFor time.Time, attachments []Attachment) error {
	attsBytes, _ := json.Marshal(attachments)
	atts := string(attsBytes)
	tag, err := r.pool.Exec(ctx, `
		UPDATE outbound_jobs
		SET body = $3, scheduled_for = $4, next_attempt_at = $4, attachments = $5
		WHERE studio_id = $1 AND id = $2 AND status = 'pending'
	`, studioID, id, body, scheduledFor, atts)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) SetJobScheduledForNow(ctx context.Context, studioID uuid.UUID, id int64) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE outbound_jobs
		SET scheduled_for = now(), next_attempt_at = now()
		WHERE studio_id = $1 AND id = $2 AND status = 'pending'
	`, studioID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) GetStudioMetaAppSecret(ctx context.Context, studioID uuid.UUID) (string, error) {
	var secret string
	err := r.pool.QueryRow(ctx, `
		SELECT meta_app_secret FROM studios WHERE id = $1
	`, studioID).Scan(&secret)
	if err != nil {
		return "", err
	}
	return secret, nil
}

func (r *Repo) GetStripeConfig(ctx context.Context, studioID uuid.UUID) (secretKey string, amountSGD int, name string, slug string, err error) {
	var encKey string
	err = r.pool.QueryRow(ctx, "SELECT stripe_secret_key, trial_amount_sgd, name, slug FROM studios WHERE id = $1", studioID).Scan(&encKey, &amountSGD, &name, &slug)
	if err != nil {
		return "", 0, "", "", err
	}
	if encKey != "" && r.cipher != nil {
		secretKey, err = r.cipher.Decrypt(encKey)
		if err != nil {
			return "", 0, "", "", err
		}
	} else {
		secretKey = encKey
	}
	return
}

type Plan struct {
	ID           uuid.UUID
	StudioID     uuid.UUID
	PlanName     string
	PriceSGD     int
	BillingCycle string
	Features     []string
}

func (r *Repo) ListActivePlans(ctx context.Context, studioID uuid.UUID) ([]Plan, error) {
	cacheKey := "plans:" + studioID.String()
	if v, ok := r.planCache.Get(cacheKey); ok {
		if plans, ok := v.([]Plan); ok {
			return plans, nil
		}
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, studio_id, plan_name, price_sgd, billing_cycle, features
		FROM plans
		WHERE studio_id = $1 AND is_active = true AND plan_name != 'Trial'
		ORDER BY price_sgd ASC
	`, studioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.StudioID, &p.PlanName, &p.PriceSGD, &p.BillingCycle, &p.Features); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	r.planCache.Set(cacheKey, out, 5*time.Minute)
	return out, nil
}

// LogLLMUsage inserts a fire-and-forget LLM usage record. Errors are silently
// dropped because logging must never block the AI response path.
func (r *Repo) LogLLMUsage(ctx context.Context, studioID uuid.UUID, provider, model string, latencyMs int, success bool, errMsg string, tokensIn, tokensOut int) {
	_, _ = r.pool.Exec(ctx, `
		INSERT INTO llm_usage_logs (studio_id, provider, model, latency_ms, success, error_msg, tokens_in, tokens_out)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, studioID, provider, model, latencyMs, success, errMsg, tokensIn, tokensOut)
}
