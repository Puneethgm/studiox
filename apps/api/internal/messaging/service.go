package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/messaging/channels"
)

// Service is the messaging use-case layer. Webhooks call HandleInboundWhatsApp,
// the UI calls SendOutbound, and the worker drains the outbound_jobs queue.
type Service struct {
	repo              *Repo
	bus               Bus
	publicFormBaseURL string
}

func NewService(repo *Repo, bus Bus, publicFormBaseURL string) *Service {
	return &Service{repo: repo, bus: bus, publicFormBaseURL: publicFormBaseURL}
}

// ============================================================
// Channel CRUD (thin wrappers around repo + future webhook subscription)
// ============================================================

type ConnectMetaInput struct {
	Kind          ChannelKind
	ExternalID    string // IG Account ID or Page ID
	ParentID      string // Optional WABA or App ID
	DisplayHandle string // e.g. "@username" or "Page Name"
	AccessToken   string
}

func (s *Service) ConnectMetaChannel(ctx context.Context, studioID uuid.UUID, in ConnectMetaInput) (*ChannelAccount, error) {
	in.ExternalID = strings.TrimSpace(in.ExternalID)
	in.DisplayHandle = strings.TrimSpace(in.DisplayHandle)
	in.AccessToken = strings.TrimSpace(in.AccessToken)

	if in.ExternalID == "" || in.DisplayHandle == "" || in.AccessToken == "" {
		return nil, errors.New("externalId, displayHandle, and accessToken are required")
	}

	return s.repo.CreateChannel(ctx, CreateChannelInput{
		StudioID:      studioID,
		Kind:          in.Kind,
		BSP:           "meta_direct",
		ExternalID:    in.ExternalID,
		ParentID:      in.ParentID,
		DisplayHandle: in.DisplayHandle,
		AccessToken:   in.AccessToken,
	})
}

func (s *Service) ListChannels(ctx context.Context, studioID uuid.UUID) ([]ChannelAccount, error) {
	return s.repo.ListChannels(ctx, studioID)
}

func (s *Service) DisconnectChannel(ctx context.Context, studioID, id uuid.UUID) error {
	return s.repo.DisconnectChannel(ctx, studioID, id)
}

func (s *Service) UpdateChannel(ctx context.Context, studioID, id uuid.UUID, externalID, parentID, displayHandle, accessToken string) (*ChannelAccount, error) {
	externalID = strings.TrimSpace(externalID)
	parentID = strings.TrimSpace(parentID)
	displayHandle = strings.TrimSpace(displayHandle)
	accessToken = strings.TrimSpace(accessToken)

	if externalID == "" || displayHandle == "" {
		return nil, errors.New("externalId and displayHandle are required")
	}

	return s.repo.UpdateChannel(ctx, UpdateChannelInput{
		ID:            id,
		StudioID:      studioID,
		ExternalID:    externalID,
		ParentID:      parentID,
		DisplayHandle: displayHandle,
		AccessToken:   accessToken,
	})
}

type CreateConversationInput struct {
	ChannelKind  ChannelKind
	ContactValue string
	DisplayName  string
	LeadID       *uuid.UUID
}

// CreateConversation opens a thread for a contact on the newest active
// channel in the studio so the inbox can start from a typed receiver number.
func (s *Service) CreateConversation(ctx context.Context, studioID uuid.UUID, in CreateConversationInput) (*Conversation, error) {
	in.ContactValue = strings.TrimSpace(in.ContactValue)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if in.ContactValue == "" {
		return nil, errors.New("contactValue is required")
	}

	channel, err := s.repo.GetActiveChannelByKind(ctx, studioID, in.ChannelKind)
	if err != nil {
		return nil, err
	}

	tx, err := s.repo.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if in.DisplayName == "" {
		in.DisplayName = in.ContactValue
	}

	idKind := IdentityPhone
	if in.ChannelKind == KindMessengerMeta {
		idKind = IdentityFBPSID
	} else if in.ChannelKind == KindInstagramMeta {
		idKind = IdentityIGPSID
	}

	identity, err := s.repo.FindOrCreateIdentity(ctx, tx, studioID, idKind, in.ContactValue, in.DisplayName)
	if err != nil {
		return nil, err
	}

	// Link identity to lead if provided
	if in.LeadID != nil {
		_, err = tx.Exec(ctx, `
			UPDATE contact_identities SET lead_id = $2 WHERE id = $1
		`, identity.ID, *in.LeadID)
		if err != nil {
			return nil, fmt.Errorf("link identity to lead: %w", err)
		}
		identity.LeadID = in.LeadID
	}

	conv, err := s.repo.FindOrCreateConversation(ctx, tx, studioID, channel.ID, identity.ID, in.ContactValue)
	if err != nil {
		return nil, err
	}

	// Link conversation to lead if provided
	if in.LeadID != nil {
		_, err = tx.Exec(ctx, `
			UPDATE conversations SET lead_id = $2 WHERE id = $1
		`, conv.ID, *in.LeadID)
		if err != nil {
			return nil, fmt.Errorf("link conversation to lead: %w", err)
		}
		leadIDStr := *in.LeadID
		conv.LeadID = &leadIDStr
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	s.bus.Publish(ctx, Event{
		Kind:           EvtConversationUpdated,
		StudioID:       studioID,
		ConversationID: conv.ID,
	})

	return conv, nil
}

// ============================================================
// Inbound (called by the Meta webhook handler)
// ============================================================

// HandleInboundWhatsAppMessage processes one message from Meta's webhook.
// Idempotent — duplicate webhook deliveries collapse into a single message
// row via the unique index on (conversation_id, external_id).
func (s *Service) HandleInboundWhatsAppMessage(ctx context.Context,
	wabaID string,
	meta channels.WhatsAppWebhookMetadata,
	contact *channels.WhatsAppWebhookContact,
	msg channels.WhatsAppWebhookMessage,
) error {
	// 1. Resolve the channel account by phone_number_id.
	channel, err := s.repo.GetChannelByExternalID(ctx, KindWhatsAppMeta, meta.PhoneNumberID)
	if err != nil {
		// Not connected to any studio — nothing to do (don't error to Meta).
		return nil
	}

	displayName := ""
	if contact != nil {
		displayName = contact.Profile.Name
	}

	// 2. Open a tx for the identity → conversation → message chain so we
	//    never end up with a half-stitched conversation.
	tx, err := s.repo.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 3. Identity stitching: phone → contact_identity (find or create).
	identity, err := s.repo.FindOrCreateIdentity(ctx, tx, channel.StudioID, IdentityPhone, msg.From, displayName)
	if err != nil {
		return err
	}

	// 4. Find/open the conversation for (channel, contact-phone).
	conv, err := s.repo.FindOrCreateConversation(ctx, tx, channel.StudioID, channel.ID, identity.ID, msg.From)
	if err != nil {
		return err
	}

	// Link identity and conversation to lead (or auto-create lead for walk-in/direct messages)
	var activeLeadID *uuid.UUID
	if identity.LeadID != nil {
		activeLeadID = identity.LeadID
	} else if conv.LeadID != nil {
		activeLeadID = conv.LeadID
	}

	if activeLeadID == nil {
		var leadID uuid.UUID
		// Search for an existing lead with this phone number in this studio (using clean digits)
		sanitizedFrom := cleanPhoneNumber(msg.From)
		err = tx.QueryRow(ctx, `
			SELECT id FROM leads 
			WHERE studio_id = $1 AND (
				REPLACE(REPLACE(REPLACE(REPLACE(phone, '+', ''), ' ', ''), '-', ''), '(', '') = $2 
				OR REPLACE(REPLACE(REPLACE(REPLACE(phone, '+', ''), ' ', ''), '-', ''), '(', '') = $3
			)
			LIMIT 1
		`, channel.StudioID, msg.From, sanitizedFrom).Scan(&leadID)

		if err != nil && errors.Is(err, pgx.ErrNoRows) {
			// Fetch active campaign and its first fitness plan
			var campaignID uuid.UUID
			var fitnessPlans []string
			errCampaign := tx.QueryRow(ctx, `
				SELECT id, fitness_plans FROM campaigns 
				WHERE studio_id = $1 AND active = true 
				ORDER BY created_at DESC 
				LIMIT 1
			`, channel.StudioID).Scan(&campaignID, &fitnessPlans)
			if errCampaign != nil {
				_ = tx.QueryRow(ctx, `
					SELECT id, fitness_plans FROM campaigns 
					WHERE studio_id = $1 
					LIMIT 1
				`, channel.StudioID).Scan(&campaignID, &fitnessPlans)
			}
			
			defaultPlan := "Trial Class"
			if len(fitnessPlans) > 0 {
				defaultPlan = fitnessPlans[0]
			}

			// No existing lead, create one automatically
			leadID = uuid.New()
			fName := displayName
			lName := ""
			if displayName == "" {
				displayName = msg.From
				fName = msg.From
			} else {
				parts := strings.SplitN(displayName, " ", 2)
				fName = parts[0]
				if len(parts) > 1 {
					lName = parts[1]
				}
			}
			emailPlaceholder := fmt.Sprintf("whatsapp-%s@example.com", msg.From)

			_, err = tx.Exec(ctx, `
				INSERT INTO leads (id, studio_id, campaign_id, name, first_name, last_name, email, phone, fitness_plan, status, source, auto_contact_stage, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'contacted', 'whatsapp', 'awaiting_options', now(), now())
			`, leadID, channel.StudioID, campaignID, displayName, fName, lName, emailPlaceholder, msg.From, defaultPlan)
			if err != nil {
				return fmt.Errorf("auto-create lead: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("lookup lead by phone: %w", err)
		}
		activeLeadID = &leadID
	}

	// Update identity and conversation with the lead ID if not set
	if identity.LeadID == nil {
		_, err = tx.Exec(ctx, `
			UPDATE contact_identities SET lead_id = $2 WHERE id = $1
		`, identity.ID, *activeLeadID)
		if err != nil {
			return fmt.Errorf("link identity to lead: %w", err)
		}
		identity.LeadID = activeLeadID
	}

	if conv.LeadID == nil {
		_, err = tx.Exec(ctx, `
			UPDATE conversations SET lead_id = $2 WHERE id = $1
		`, conv.ID, *activeLeadID)
		if err != nil {
			return fmt.Errorf("link conversation to lead: %w", err)
		}
		leadIDStr := *activeLeadID
		conv.LeadID = &leadIDStr
	}

	// 5. Insert the message (deduped by external_id).
	body := ""
	atts := []Attachment{}
	if msg.Text != nil {
		body = msg.Text.Body
	}
	if msg.Interactive != nil {
		if msg.Interactive.ButtonReply != nil {
			body = msg.Interactive.ButtonReply.Title
		} else if msg.Interactive.ListReply != nil {
			body = msg.Interactive.ListReply.Title
		}
	}
	if msg.Button != nil {
		body = msg.Button.Text
	}
	if msg.Image != nil {
		url, name := "", ""
		if downloadedURL, downloadedName, err := downloadWhatsAppMedia(ctx, channel.AccessToken, msg.Image.ID, msg.Image.MimeType, msg.ID); err == nil {
			url, name = downloadedURL, downloadedName
			if strings.HasPrefix(url, "/") && s.publicFormBaseURL != "" {
				url = strings.TrimRight(s.publicFormBaseURL, "/") + url
			}
		}
		atts = append(atts, Attachment{Type: "image", URL: url, Mime: msg.Image.MimeType, Name: name})
		if body == "" {
			body = msg.Image.Caption
		}
	}
	if msg.Video != nil {
		url, name := "", ""
		if downloadedURL, downloadedName, err := downloadWhatsAppMedia(ctx, channel.AccessToken, msg.Video.ID, msg.Video.MimeType, msg.ID); err == nil {
			url, name = downloadedURL, downloadedName
			if strings.HasPrefix(url, "/") && s.publicFormBaseURL != "" {
				url = strings.TrimRight(s.publicFormBaseURL, "/") + url
			}
		}
		atts = append(atts, Attachment{Type: "video", URL: url, Mime: msg.Video.MimeType, Name: name})
		if body == "" {
			body = msg.Video.Caption
		}
	}
	if msg.Audio != nil {
		url, name := "", ""
		if downloadedURL, downloadedName, err := downloadWhatsAppMedia(ctx, channel.AccessToken, msg.Audio.ID, msg.Audio.MimeType, msg.ID); err == nil {
			url, name = downloadedURL, downloadedName
			if strings.HasPrefix(url, "/") && s.publicFormBaseURL != "" {
				url = strings.TrimRight(s.publicFormBaseURL, "/") + url
			}
		}
		atts = append(atts, Attachment{Type: "audio", URL: url, Mime: msg.Audio.MimeType, Name: name})
	}
	if msg.Document != nil {
		url, name := "", ""
		if downloadedURL, downloadedName, err := downloadWhatsAppMedia(ctx, channel.AccessToken, msg.Document.ID, msg.Document.MimeType, msg.ID); err == nil {
			url, name = downloadedURL, downloadedName
			if strings.HasPrefix(url, "/") && s.publicFormBaseURL != "" {
				url = strings.TrimRight(s.publicFormBaseURL, "/") + url
			}
		}
		atts = append(atts, Attachment{Type: "document", URL: url, Mime: msg.Document.MimeType, Name: name})
	}
	if body == "" && len(atts) == 0 {
		// Unknown type — skip but don't error to Meta.
		return tx.Commit(ctx)
	}

	inReply := ""
	if msg.Context != nil {
		inReply = msg.Context.ID
	}

	stored, err := s.repo.InsertMessage(ctx, tx, CreateMessageInput{
		ConversationID: conv.ID,
		StudioID:       channel.StudioID,
		Direction:      DirectionInbound,
		SourceKind:     SourceCustomer,
		Body:           body,
		Attachments:    atts,
		ExternalID:     msg.ID,
		InReplyTo:      inReply,
		Status:         MsgSent,
		SentAt:         channels.ParseTimestamp(msg.Timestamp),
	})
	if err != nil {
		return err
	}

	// Deterministically update lead status based on button clicks / option selections.
	// This runs inside the transaction and works even if the AI (Claude) worker is disabled.
	if err := s.processInboundLeadAutomation(ctx, tx, channel.StudioID, conv, stored, body); err != nil {
		return fmt.Errorf("inbound lead automation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// 6. Publish event for SSE / future automations / future AI suggester.
	if stored != nil {
		s.bus.Publish(ctx, Event{
			Kind:           EvtMessageReceived,
			StudioID:       channel.StudioID,
			ConversationID: conv.ID,
			MessageID:      &stored.ID,
		})
	} else {
		// Duplicate — still notify so the UI re-fetches in case the dedupe
		// happened across replicas. Cheap.
		s.bus.Publish(ctx, Event{
			Kind:           EvtConversationUpdated,
			StudioID:       channel.StudioID,
			ConversationID: conv.ID,
		})
	}
	return nil
}

func downloadWhatsAppMedia(ctx context.Context, accessToken, mediaID, mimeType, messageID string) (string, string, error) {
	if accessToken == "" || mediaID == "" {
		return "", "", fmt.Errorf("missing media id or access token")
	}

	metaReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", channels.MetaGraphBaseURL, mediaID), nil)
	if err != nil {
		return "", "", err
	}
	metaReq.Header.Set("Authorization", "Bearer "+accessToken)

	metaResp, err := http.DefaultClient.Do(metaReq)
	if err != nil {
		return "", "", fmt.Errorf("media metadata request: %w", err)
	}
	defer metaResp.Body.Close()
	metaBody, _ := io.ReadAll(metaResp.Body)
	if metaResp.StatusCode >= 400 {
		return "", "", fmt.Errorf("media metadata HTTP %d: %s", metaResp.StatusCode, string(metaBody))
	}

	var metaOK struct {
		URL      string `json:"url"`
		MimeType string `json:"mime_type"`
	}
	if err := json.Unmarshal(metaBody, &metaOK); err != nil {
		return "", "", fmt.Errorf("decode media metadata: %w", err)
	}
	if metaOK.URL == "" {
		return "", "", fmt.Errorf("empty media url")
	}

	mediaReq, err := http.NewRequestWithContext(ctx, http.MethodGet, metaOK.URL, nil)
	if err != nil {
		return "", "", err
	}
	mediaReq.Header.Set("Authorization", "Bearer "+accessToken)

	mediaResp, err := http.DefaultClient.Do(mediaReq)
	if err != nil {
		return "", "", fmt.Errorf("download media: %w", err)
	}
	defer mediaResp.Body.Close()
	if mediaResp.StatusCode >= 400 {
		body, _ := io.ReadAll(mediaResp.Body)
		return "", "", fmt.Errorf("media download HTTP %d: %s", mediaResp.StatusCode, string(body))
	}

	if err := os.MkdirAll("uploads", 0o755); err != nil {
		return "", "", fmt.Errorf("create uploads dir: %w", err)
	}

	ext := extFromMimeType(mimeType)
	if ext == "" {
		ext = extFromMimeType(metaOK.MimeType)
	}
	if ext == "" {
		if exts, _ := mime.ExtensionsByType(mediaResp.Header.Get("Content-Type")); len(exts) > 0 {
			ext = exts[0]
		}
	}
	if ext == "" {
		ext = ".bin"
	}

	fileName := fmt.Sprintf("whatsapp-%s%s", messageID, ext)
	outPath := filepath.Join("uploads", fileName)
	outFile, err := os.Create(outPath)
	if err != nil {
		return "", "", fmt.Errorf("create media file: %w", err)
	}
	defer outFile.Close()
	if _, err := io.Copy(outFile, mediaResp.Body); err != nil {
		return "", "", fmt.Errorf("write media file: %w", err)
	}

	return "/uploads/" + fileName, fileName, nil
}

func extFromMimeType(mimeType string) string {
	if mimeType == "" {
		return ""
	}
	if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
		return exts[0]
	}
	return ""
}

// HandleInboundMessaging processes a DM from Instagram or Facebook Messenger.
func (s *Service) HandleInboundMessaging(ctx context.Context, kind ChannelKind, m channels.MetaWebhookMessaging) error {
	if m.Message == nil || m.Message.Mid == "" {
		return nil
	}

	// 1. Resolve the channel account by the recipient's ID (the IG Account or FB Page PSID).
	fmt.Printf("DEBUG: HandleInboundMessaging: kind=%s, recipientID=%s, senderID=%s\n", kind, m.Recipient.ID, m.Sender.ID)
	channel, err := s.repo.GetChannelByExternalID(ctx, kind, m.Recipient.ID)
	if err != nil {
		// Fallback: try sender ID (e.g. if the page itself sent the message or for echo events)
		channel, err = s.repo.GetChannelByExternalID(ctx, kind, m.Sender.ID)
		if err != nil {
			// Log the mismatch so we can debug.
			fmt.Printf("DEBUG: Meta message received for unknown channel: kind=%s, recipientID=%s, senderID=%s\n", kind, m.Recipient.ID, m.Sender.ID)
			return nil
		}
	}

	tx, err := s.repo.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 2. Identity: use the specific Meta identity kind (ig_psid or fb_psid).
	idKind := IdentityIGPSID
	if kind == KindMessengerMeta {
		idKind = IdentityFBPSID
	}

	identity, err := s.repo.FindOrCreateIdentity(ctx, tx, channel.StudioID, idKind, m.Sender.ID, "")
	if err != nil {
		return err
	}

	// 3. Conversation.
	conv, err := s.repo.FindOrCreateConversation(ctx, tx, channel.StudioID, channel.ID, identity.ID, m.Sender.ID)
	if err != nil {
		return err
	}

	// Link identity and conversation to lead (or auto-create lead for walk-in/direct messages)
	var activeLeadID *uuid.UUID
	if identity.LeadID != nil {
		activeLeadID = identity.LeadID
	} else if conv.LeadID != nil {
		activeLeadID = conv.LeadID
	}

	if activeLeadID == nil {
		var campaignID uuid.UUID
		var fitnessPlans []string
		errCampaign := tx.QueryRow(ctx, `
			SELECT id, fitness_plans FROM campaigns 
			WHERE studio_id = $1 AND active = true 
			ORDER BY created_at DESC 
			LIMIT 1
		`, channel.StudioID).Scan(&campaignID, &fitnessPlans)
		if errCampaign != nil {
			_ = tx.QueryRow(ctx, `
				SELECT id, fitness_plans FROM campaigns 
				WHERE studio_id = $1 
				LIMIT 1
			`, channel.StudioID).Scan(&campaignID, &fitnessPlans)
		}
		
		defaultPlan := "Trial Class"
		if len(fitnessPlans) > 0 {
			defaultPlan = fitnessPlans[0]
		}

		// No existing lead, create one automatically
		leadID := uuid.New()
		displayName := "Messenger Guest"
		if kind == KindInstagramMeta {
			displayName = "Instagram Guest"
		}
		emailPlaceholder := fmt.Sprintf("%s-%s@example.com", string(kind), m.Sender.ID)
		phonePlaceholder := fmt.Sprintf("meta-%s", m.Sender.ID)

		_, err = tx.Exec(ctx, `
			INSERT INTO leads (id, studio_id, campaign_id, name, first_name, last_name, email, phone, fitness_plan, status, source, auto_contact_stage, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'contacted', $10, 'awaiting_options', now(), now())
		`, leadID, channel.StudioID, campaignID, displayName, displayName, "", emailPlaceholder, phonePlaceholder, defaultPlan, string(kind))
		if err != nil {
			return fmt.Errorf("auto-create messenger lead: %w", err)
		}
		activeLeadID = &leadID
	}

	// Update identity and conversation with the lead ID if not set
	if identity.LeadID == nil {
		_, err = tx.Exec(ctx, `
			UPDATE contact_identities SET lead_id = $2 WHERE id = $1
		`, identity.ID, *activeLeadID)
		if err != nil {
			return fmt.Errorf("link identity to lead: %w", err)
		}
		identity.LeadID = activeLeadID
	}

	if conv.LeadID == nil {
		_, err = tx.Exec(ctx, `
			UPDATE conversations SET lead_id = $2 WHERE id = $1
		`, conv.ID, *activeLeadID)
		if err != nil {
			return fmt.Errorf("link conversation to lead: %w", err)
		}
		leadIDStr := *activeLeadID
		conv.LeadID = &leadIDStr
	}

	var atts []Attachment
	for _, a := range m.Message.Attachments {
		mimeType := ""
		if a.Type == "image" {
			mimeType = "image/jpeg"
		} else if a.Type == "video" {
			mimeType = "video/mp4"
		} else if a.Type == "audio" {
			mimeType = "audio/mpeg"
		} else if a.Type == "file" || a.Type == "document" {
			mimeType = "application/octet-stream"
		}
		
		atts = append(atts, Attachment{
			Type: a.Type,
			URL:  a.Payload.URL,
			Mime: mimeType,
			Name: "attachment",
		})
	}

	// 4. Insert message.
	stored, err := s.repo.InsertMessage(ctx, tx, CreateMessageInput{
		ConversationID: conv.ID,
		StudioID:       channel.StudioID,
		Direction:      DirectionInbound,
		SourceKind:     SourceCustomer,
		Body:           m.Message.Text,
		Attachments:    atts,
		ExternalID:     m.Message.Mid,
		SentAt:         time.Unix(m.Timestamp/1000, (m.Timestamp%1000)*1000000).UTC(),
	})
	if err != nil {
		return err
	}

	inputText := m.Message.Text
	if m.Message.QuickReply != nil && m.Message.QuickReply.Payload != "" {
		inputText = m.Message.QuickReply.Payload
	}

	// Deterministically update lead status based on button clicks / option selections.
	if err := s.processInboundLeadAutomation(ctx, tx, channel.StudioID, conv, stored, inputText); err != nil {
		return fmt.Errorf("inbound lead automation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if stored != nil {
		s.bus.Publish(ctx, Event{
			Kind:           EvtMessageReceived,
			StudioID:       channel.StudioID,
			ConversationID: conv.ID,
			MessageID:      &stored.ID,
		})
	}
	return nil
}

// HandleStatus updates an outbound message's delivery state when Meta tells
// us it was delivered/read/failed. Looked up by Meta wamid.
func (s *Service) HandleStatus(ctx context.Context, st channels.WhatsAppWebhookStatus) error {
	if st.ID == "" || st.Status == "" {
		return nil
	}
	ts := channels.ParseTimestamp(st.Timestamp)
	switch st.Status {
	case "delivered":
		_, err := s.repo.Pool().Exec(ctx, `
			UPDATE messages SET delivered_at = $2, status = 'delivered'
			WHERE external_id = $1 AND direction = 'outbound'
		`, st.ID, ts)
		return err
	case "read":
		_, err := s.repo.Pool().Exec(ctx, `
			UPDATE messages SET read_at = $2, status = 'read'
			WHERE external_id = $1 AND direction = 'outbound'
		`, st.ID, ts)
		return err
	case "failed":
		_, err := s.repo.Pool().Exec(ctx, `
			UPDATE messages SET status = 'failed' WHERE external_id = $1 AND direction = 'outbound'
		`, st.ID)
		return err
	}
	return nil
}

// ============================================================
// Outbound (called by the UI via REST → enqueues)
// ============================================================

type SendInput struct {
	StudioID       uuid.UUID
	ConversationID uuid.UUID
	UserID         uuid.UUID // who in the studio is sending
	Body           string
	Attachments    []Attachment
}

// EnqueueReply queues a manual reply on an existing conversation. Worker
// dispatches via the channel adapter. Returns the job id so the UI can
// optimistically render.
func (s *Service) EnqueueReply(ctx context.Context, in SendInput) (int64, error) {
	in.Body = strings.TrimSpace(in.Body)
	if in.Body == "" && len(in.Attachments) == 0 {
		return 0, errors.New("body or attachments are required")
	}
	conv, err := s.repo.GetConversation(ctx, in.StudioID, in.ConversationID)
	if err != nil {
		return 0, err
	}
	if conv.Status == ConvClosed {
		return 0, errors.New("conversation is closed")
	}
	return s.repo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       in.StudioID,
		ConversationID: in.ConversationID,
		Body:           in.Body,
		Attachments:    in.Attachments,
		SourceKind:     SourceStudioUser,
		SourceUserID:   &in.UserID,
		ScheduledFor:   time.Now().UTC(),
	})
}

// ============================================================
// Listing / reading (REST endpoints call these)
// ============================================================

func (s *Service) ListConversations(ctx context.Context, studioID uuid.UUID, f ListConversationsFilter) ([]Conversation, int, error) {
	return s.repo.ListConversations(ctx, studioID, f)
}

func (s *Service) GetConversation(ctx context.Context, studioID, id uuid.UUID) (*Conversation, error) {
	return s.repo.GetConversation(ctx, studioID, id)
}

func (s *Service) ListMessages(ctx context.Context, studioID, conversationID uuid.UUID, limit int) ([]Message, error) {
	return s.repo.ListMessages(ctx, studioID, conversationID, limit)
}

func (s *Service) MarkRead(ctx context.Context, studioID, conversationID uuid.UUID) error {
	if err := s.repo.MarkConversationRead(ctx, studioID, conversationID); err != nil {
		return err
	}
	s.bus.Publish(ctx, Event{
		Kind:           EvtConversationUpdated,
		StudioID:       studioID,
		ConversationID: conversationID,
	})
	return nil
}

// ============================================================
// message_templates
// ============================================================

func (s *Service) ListTemplates(ctx context.Context, studioID uuid.UUID) ([]MessageTemplate, error) {
	return s.repo.ListTemplates(ctx, studioID)
}

func (s *Service) CreateTemplate(ctx context.Context, studioID uuid.UUID, name, body string, channelKinds []string, attachments []Attachment) (*MessageTemplate, error) {
	name = strings.TrimSpace(name)
	body = strings.TrimSpace(body)
	if name == "" {
		return nil, errors.New("template name is required")
	}
	if body == "" {
		return nil, errors.New("template body is required")
	}
	mt := &MessageTemplate{
		StudioID:     studioID,
		Name:         name,
		Body:         body,
		ChannelKinds: channelKinds,
		Attachments:  attachments,
	}
	if err := s.repo.CreateTemplate(ctx, mt); err != nil {
		return nil, err
	}
	return mt, nil
}

func (s *Service) DeleteTemplate(ctx context.Context, studioID, id uuid.UUID) error {
	return s.repo.DeleteTemplate(ctx, studioID, id)
}

func (s *Service) UpdateTemplate(ctx context.Context, studioID, id uuid.UUID, name, body string, channelKinds []string, attachments []Attachment) (*MessageTemplate, error) {
	name = strings.TrimSpace(name)
	body = strings.TrimSpace(body)
	if name == "" {
		return nil, errors.New("template name is required")
	}
	if body == "" {
		return nil, errors.New("template body is required")
	}
	mt := &MessageTemplate{
		ID:           id,
		StudioID:     studioID,
		Name:         name,
		Body:         body,
		ChannelKinds: channelKinds,
		Attachments:  attachments,
	}
	if err := s.repo.UpdateTemplate(ctx, mt); err != nil {
		return nil, err
	}
	return mt, nil
}

// ============================================================
// trigger_links
// ============================================================

func (s *Service) ListTriggerLinks(ctx context.Context, studioID uuid.UUID) ([]TriggerLink, error) {
	return s.repo.ListTriggerLinks(ctx, studioID)
}

func (s *Service) CreateTriggerLink(ctx context.Context, studioID uuid.UUID, name, url string) (*TriggerLink, error) {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" {
		return nil, errors.New("trigger link name is required")
	}
	if url == "" {
		return nil, errors.New("trigger link target url is required")
	}
	tl := &TriggerLink{
		StudioID: studioID,
		Name:     name,
		URL:      url,
	}
	if err := s.repo.CreateTriggerLink(ctx, tl); err != nil {
		return nil, err
	}
	return tl, nil
}

func (s *Service) DeleteTriggerLink(ctx context.Context, studioID, id uuid.UUID) error {
	return s.repo.DeleteTriggerLink(ctx, studioID, id)
}

func (s *Service) UpdateTriggerLink(ctx context.Context, studioID, id uuid.UUID, name, url string) (*TriggerLink, error) {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" {
		return nil, errors.New("trigger link name is required")
	}
	if url == "" {
		return nil, errors.New("trigger link target url is required")
	}
	tl := &TriggerLink{
		ID:       id,
		StudioID: studioID,
		Name:     name,
		URL:      url,
	}
	if err := s.repo.UpdateTriggerLink(ctx, tl); err != nil {
		return nil, err
	}
	return tl, nil
}

func (s *Service) GetTriggerLinkByID(ctx context.Context, id uuid.UUID) (*TriggerLink, error) {
	return s.repo.GetTriggerLinkByID(ctx, id)
}

func (s *Service) RecordTriggerLinkClick(ctx context.Context, linkID uuid.UUID, leadID *uuid.UUID) error {
	return s.repo.RecordTriggerLinkClick(ctx, linkID, leadID)
}

// ============================================================
// outbound_jobs / automated messages log
// ============================================================

func (s *Service) ListPendingJobs(ctx context.Context, studioID uuid.UUID) ([]PendingJobInfo, error) {
	return s.repo.ListPendingJobs(ctx, studioID)
}

func (s *Service) DeleteJob(ctx context.Context, studioID uuid.UUID, id int64) error {
	return s.repo.DeleteJob(ctx, studioID, id)
}

func (s *Service) TriggerJobNow(ctx context.Context, studioID uuid.UUID, id int64) error {
	return s.repo.SetJobScheduledForNow(ctx, studioID, id)
}

func (s *Service) CreateJob(ctx context.Context, studioID uuid.UUID, conversationID uuid.UUID, body string, scheduledFor time.Time, attachments []Attachment) (int64, error) {
	body = strings.TrimSpace(body)
	if body == "" && len(attachments) == 0 {
		return 0, errors.New("message body or attachment is required")
	}
	if conversationID == uuid.Nil {
		return 0, errors.New("recipient conversation is required")
	}
	job := OutboundJob{
		StudioID:       studioID,
		ConversationID: conversationID,
		Body:           body,
		Attachments:    attachments,
		SourceKind:     SourceStudioUser,
		ScheduledFor:   scheduledFor,
	}
	return s.repo.EnqueueOutbound(ctx, job)
}

func (s *Service) UpdateJob(ctx context.Context, studioID uuid.UUID, id int64, body string, scheduledFor time.Time, attachments []Attachment) error {
	body = strings.TrimSpace(body)
	if body == "" && len(attachments) == 0 {
		return errors.New("message body or attachment is required")
	}
	return s.repo.UpdateJob(ctx, studioID, id, body, scheduledFor, attachments)
}

func format12Hour(tm string) string {
	tm = strings.TrimSpace(tm)
	parsed, err := time.Parse("15:04", tm)
	if err != nil {
		if parsed12, err12 := time.Parse("03:04 PM", tm); err12 == nil {
			return parsed12.Format("03:04 PM")
		}
		if parsed12NoZero, err12NoZero := time.Parse("3:04 PM", tm); err12NoZero == nil {
			return parsed12NoZero.Format("03:04 PM")
		}
		return tm
	}
	return parsed.Format("03:04 PM")
}

func cleanPhoneNumber(in string) string {
	var sb strings.Builder
	for _, r := range in {
		if r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	s := sb.String()
	if len(s) == 10 {
		s = "91" + s
	}
	return s
}

func (s *Service) processInboundLeadAutomation(ctx context.Context, tx pgx.Tx, studioID uuid.UUID, conv *Conversation, stored *Message, body string) error {
	if conv.LeadID == nil || stored == nil {
		return nil
	}
	var leadName, leadStatus, leadNotes, autoContactStage, studioSlug, campaignSlug string
	var slotsJSON []byte
	var timezone string
	err := tx.QueryRow(ctx, `
		SELECT l.name, l.status, l.notes, l.auto_contact_stage, s.slug, COALESCE(c.slug, ''), s.availability_slots, s.availability_timezone
		FROM leads l
		JOIN studios s ON s.id = l.studio_id
		LEFT JOIN campaigns c ON c.id = l.campaign_id
		WHERE l.studio_id = $1 AND l.id = $2
	`, studioID, *conv.LeadID).Scan(&leadName, &leadStatus, &leadNotes, &autoContactStage, &studioSlug, &campaignSlug, &slotsJSON, &timezone)

	if err != nil {
		return nil // Lead not found or other db error, skip automation
	}

	text := strings.ToLower(strings.TrimSpace(body))
	targetStage := autoContactStage
	targetStatus := leadStatus
	targetNotes := leadNotes
	var outboundBody string

	// Parse availability timezone and slots
	loc, errLoc := time.LoadLocation(timezone)
	if errLoc != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)

	type availabilitySlot struct {
		Day   string   `json:"day"`
		Times []string `json:"times"`
	}
	var slots []availabilitySlot
	if len(slotsJSON) > 0 {
		_ = json.Unmarshal(slotsJSON, &slots)
	}
	slotsMap := make(map[string][]string)
	for _, s := range slots {
		if len(s.Times) > 0 {
			slotsMap[strings.ToLower(s.Day)] = s.Times
		}
	}

	getAvailableDays := func() ([]string, []string) {
		var days []string
		var daysWeekdayStr []string
		t := now
		for i := 0; i < 30 && len(days) < 3; i++ {
			weekdayStr := strings.ToLower(t.Weekday().String())
			hasSlots := false
			if len(slotsMap) > 0 {
				_, hasSlots = slotsMap[weekdayStr]
			} else {
				hasSlots = t.Weekday() != time.Sunday
			}

			if hasSlots {
				days = append(days, t.Format("Mon, Jan 02"))
				daysWeekdayStr = append(daysWeekdayStr, weekdayStr)
			}
			t = t.Add(24 * time.Hour)
		}
		if len(days) < 3 {
			days = nil
			daysWeekdayStr = nil
			t = now
			for len(days) < 3 {
				if t.Weekday() != time.Sunday {
					days = append(days, t.Format("Mon, Jan 02"))
					daysWeekdayStr = append(daysWeekdayStr, strings.ToLower(t.Weekday().String()))
				}
				t = t.Add(24 * time.Hour)
			}
		}
		return days, daysWeekdayStr
	}

	isInterested := text == "1" || text == "interested" || (strings.Contains(text, "interested") && !strings.Contains(text, "not interested"))
	isNotInterested := text == "2" || text == "not interested" || strings.Contains(text, "not interested") || strings.Contains(text, "not_interested")
	isTrial := text == "1" || strings.Contains(text, "book a trial") || strings.Contains(text, "book trial") || strings.Contains(text, "trial") || strings.Contains(text, "book a trail") || strings.Contains(text, "book trail") || strings.Contains(text, "trail")
	isMember := text == "2" || strings.Contains(text, "become a member") || strings.Contains(text, "become member") || strings.Contains(text, "member")

	if autoContactStage == "awaiting_interest" {
		if isInterested && !isNotInterested {
			targetStage = "awaiting_options"
			targetStatus = "contacted"
			outboundBody = "Hi {{contact.first_name}}, great! Please select an option:\n1. Book a Trial\n2. Become a Member"
		} else if isNotInterested {
			targetStage = "awaiting_reason"
			targetStatus = "dropped"
			outboundBody = "We would like to know why are u not interested."
		}
	} else if autoContactStage == "awaiting_options" || (leadStatus == "trial_booked" && autoContactStage != "awaiting_trial_date" && autoContactStage != "awaiting_trial_time" && (isTrial || isMember)) {
		if isTrial && !isMember {
			targetStage = "awaiting_trial_date"
			targetStatus = "trial_booked"
			days, _ := getAvailableDays()
			outboundBody = fmt.Sprintf("Please select a date for your trial:\n1. %s\n2. %s\n3. %s", days[0], days[1], days[2])
		} else if isMember && !isTrial {
			targetStage = "completed"
			targetStatus = "member"
			outboundBody = "Our team would reach u ASAP."
		}
	} else if autoContactStage == "awaiting_trial_date" {
		days, daysWeekdayStr := getAvailableDays()
		selectedDate := ""
		selectedWeekday := ""
		isOption1 := text == "1" || strings.Contains(text, "choice_1") || strings.Contains(text, strings.ToLower(days[0]))
		isOption2 := text == "2" || strings.Contains(text, "choice_2") || strings.Contains(text, strings.ToLower(days[1]))
		isOption3 := text == "3" || strings.Contains(text, "choice_3") || strings.Contains(text, strings.ToLower(days[2]))

		if isOption1 {
			selectedDate = days[0]
			selectedWeekday = daysWeekdayStr[0]
		} else if isOption2 {
			selectedDate = days[1]
			selectedWeekday = daysWeekdayStr[1]
		} else if isOption3 {
			selectedDate = days[2]
			selectedWeekday = daysWeekdayStr[2]
		} else {
			selectedDate = days[0]
			selectedWeekday = daysWeekdayStr[0]
		}

		targetNotes = strings.TrimSpace(targetNotes + "\n[Selected Trial Date]: " + selectedDate)
		targetStage = "awaiting_trial_time"

		var timeSlots []string
		if len(slotsMap) > 0 {
			if times, ok := slotsMap[selectedWeekday]; ok && len(times) > 0 {
				for _, tm := range times {
					timeSlots = append(timeSlots, format12Hour(tm))
				}
			}
		}
		if len(timeSlots) == 0 {
			timeSlots = []string{"09:00 AM", "12:00 PM", "04:00 PM"}
		}

		var sb strings.Builder
		sb.WriteString("Please select a time slot:")
		for idx, ts := range timeSlots {
			sb.WriteString(fmt.Sprintf("\n%d. %s", idx+1, ts))
		}
		outboundBody = sb.String()
	} else if autoContactStage == "awaiting_trial_time" {
		dateStr := ""
		idx := strings.Index(targetNotes, "[Selected Trial Date]: ")
		if idx != -1 {
			dateStr = targetNotes[idx+len("[Selected Trial Date]: "):]
			if end := strings.Index(dateStr, "\n"); end != -1 {
				dateStr = dateStr[:end]
			}
			dateStr = strings.TrimSpace(dateStr)
		}
		if dateStr == "" {
			dateStr = now.Format("Mon, Jan 02")
		}

		// Determine weekday from dateStr in local timezone
		dateStrWithYear := fmt.Sprintf("%s %d", dateStr, now.Year())
		parsedDate, errDate := time.ParseInLocation("Mon, Jan 02 2006", dateStrWithYear, loc)
		selectedWeekday := ""
		if errDate == nil {
			selectedWeekday = strings.ToLower(parsedDate.Weekday().String())
		} else {
			selectedWeekday = strings.ToLower(now.Weekday().String())
		}

		var timeSlots []string
		if len(slotsMap) > 0 {
			if times, ok := slotsMap[selectedWeekday]; ok && len(times) > 0 {
				for _, tm := range times {
					timeSlots = append(timeSlots, format12Hour(tm))
				}
			}
		}
		if len(timeSlots) == 0 {
			timeSlots = []string{"09:00 AM", "12:00 PM", "04:00 PM"}
		}

		selectedIndex := -1
		for idx := range timeSlots {
			choiceStr := fmt.Sprintf("%d", idx+1)
			if text == choiceStr || strings.Contains(text, "choice_"+choiceStr) {
				selectedIndex = idx
				break
			}
		}
		if selectedIndex == -1 {
			for idx, ts := range timeSlots {
				cleanText := strings.ReplaceAll(strings.ReplaceAll(text, " ", ""), ":", "")
				cleanTs := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(ts), " ", ""), ":", "")
				if strings.Contains(cleanText, cleanTs) {
					selectedIndex = idx
					break
				}
			}
		}
		if selectedIndex == -1 || selectedIndex >= len(timeSlots) {
			selectedIndex = 0
		}
		selectedTime := timeSlots[selectedIndex]

		targetNotes = strings.ReplaceAll(targetNotes, "[Selected Trial Date]: "+dateStr, "")
		targetNotes = strings.TrimSpace(targetNotes + "\n[Selected Trial Slot]: " + dateStr + " " + selectedTime)
		targetStage = "completed"
		outboundBody = "Thank you our team will reach u out "
	} else if autoContactStage == "awaiting_reason" {
		targetNotes = strings.TrimSpace(targetNotes + "\n[Dropped Reason]: " + body)
		targetStage = "completed"
		outboundBody = "Thank you for your time we would get back to u."
	}

	// Update the lead if anything changed
	if targetStage != autoContactStage || targetStatus != leadStatus || targetNotes != leadNotes {
		_, err = tx.Exec(ctx, `
			UPDATE leads 
			SET status = $3, notes = $4, auto_contact_stage = $5, updated_at = now()
			WHERE studio_id = $1 AND id = $2
		`, studioID, *conv.LeadID, targetStatus, targetNotes, targetStage)
		if err != nil {
			return err
		}

		// If outbound message needs to be sent
		if outboundBody != "" {
			_, err = tx.Exec(ctx, `
				INSERT INTO outbound_jobs (studio_id, conversation_id, body, attachments,
				                           source_kind, source_ref, scheduled_for, next_attempt_at)
				VALUES ($1, $2, $3, '[]'::jsonb, 'automation', $4, $5, $5)
			`, studioID, conv.ID, outboundBody, fmt.Sprintf("lead:%s:auto_reply:%s", conv.LeadID.String(), targetStage), time.Now().UTC())
			if err != nil {
				return err
			}
		}

		// If they just transitioned to trial_booked, schedule the 1-day check-in follow-up
		if targetStatus == "trial_booked" && leadStatus != "trial_booked" {
			followupBody := "Hi {{contact.first_name}}, we hope you're enjoying your trial! Are you ready to take the next step and become a member? Please select an option:\n1. Book a Trial\n2. Become a Member"
			_, err = tx.Exec(ctx, `
				INSERT INTO outbound_jobs (studio_id, conversation_id, body, attachments,
				                           source_kind, source_ref, scheduled_for, next_attempt_at)
				VALUES ($1, $2, $3, '[]'::jsonb, 'automation', $4, $5, $5)
			`, studioID, conv.ID, followupBody, fmt.Sprintf("lead:%s:trial_followup:1day", conv.LeadID.String()), time.Now().UTC().Add(24*time.Hour))
			if err != nil {
				return err
			}
		}

		// Enqueue Google Sheets update if status or notes changed
		if targetStatus != leadStatus || targetNotes != leadNotes {
			var l leads.Lead
			var ipText *string
			row := tx.QueryRow(ctx, `
				SELECT l.id, l.studio_id, l.campaign_id, l.name, COALESCE(l.first_name, ''), COALESCE(l.last_name, ''), l.email, l.phone, l.fitness_plan, l.goals,
				       l.source, l.status, l.notes, l.contact_attempts, l.last_contacted_at, l.contact_made, l.hot_lead, l.trial_purchased, l.auto_contact_stage, l.referrer, l.user_agent, l.ip_address::text, l.created_at, l.updated_at,
				       s.name, s.slug, c.name, c.slug
				FROM leads l
				JOIN campaigns c ON c.id = l.campaign_id
				JOIN studios s ON s.id = l.studio_id
				WHERE l.id = $1
			`, *conv.LeadID)
			scanErr := row.Scan(&l.ID, &l.StudioID, &l.CampaignID, &l.Name, &l.FirstName, &l.LastName, &l.Email, &l.Phone, &l.FitnessPlan, &l.Goals,
				&l.Source, &l.Status, &l.Notes, &l.ContactAttempts, &l.LastContactedAt, &l.ContactMade, &l.HotLead, &l.TrialPurchased, &l.AutoContactStage, &l.Referrer, &l.UserAgent, &ipText, &l.CreatedAt, &l.UpdatedAt,
				&l.StudioName, &l.StudioSlug, &l.CampaignName, &l.CampaignSlug)
			if scanErr == nil {
				payload, mErr := json.Marshal(l)
				if mErr == nil {
					_, _ = tx.Exec(ctx, `
						INSERT INTO outbox (aggregate_type, aggregate_id, event_type, destination, payload)
						VALUES ('lead', $1, 'lead.updated', 'google_sheets', $2)
					`, l.ID, payload)
				}
			}
		}
	}
	return nil
}
