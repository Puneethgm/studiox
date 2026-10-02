package messaging

import (
	"time"

	"github.com/google/uuid"
)

// ----- broadcast (Manual Actions bulk-send) -----

type BroadcastList struct {
	ID           uuid.UUID `json:"id"`
	StudioID     uuid.UUID `json:"studioId"`
	Name         string    `json:"name"`
	ContactCount int       `json:"contactCount"`
	CreatedAt    time.Time `json:"createdAt"`
}

type BroadcastContact struct {
	ID              uuid.UUID `json:"id"`
	BroadcastListID uuid.UUID `json:"broadcastListId"`
	Name            string    `json:"name"`
	Phone           string    `json:"phone"`
	Email           string    `json:"email"`
	CreatedAt       time.Time `json:"createdAt"`
}

// ImportedContactRow is one parsed row from an uploaded sheet, before it's
// known whether its phone number needs a country code applied.
type ImportedContactRow struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// BroadcastImportPreview is returned right after upload — rows are staged,
// not yet saved as contacts, until the caller confirms (optionally with a
// default country code for numbers missing one).
type BroadcastImportPreview struct {
	ImportID            uuid.UUID `json:"importId"`
	TotalRows            int      `json:"totalRows"`
	MissingCountryCode   int      `json:"missingCountryCode"`
	SampleMissingNumbers []string `json:"sampleMissingNumbers"` // up to 5, for the confirm-dialog's "e.g. these numbers" copy
}

type BroadcastCampaignStatus string

const (
	BroadcastScheduled BroadcastCampaignStatus = "scheduled"
	BroadcastSending   BroadcastCampaignStatus = "sending"
	BroadcastCompleted BroadcastCampaignStatus = "completed"
	BroadcastCanceled  BroadcastCampaignStatus = "canceled"
)

// BroadcastChannel is which channel family a campaign goes out on — chosen
// at compose time, resolved to an actual connected ChannelKind (whatsapp ->
// whatsapp_meta or whatsapp_web, email -> email_smtp) by the worker.
type BroadcastChannel string

const (
	BroadcastChannelWhatsApp BroadcastChannel = "whatsapp"
	BroadcastChannelEmail    BroadcastChannel = "email"
)

type BroadcastCampaign struct {
	ID              uuid.UUID               `json:"id"`
	StudioID        uuid.UUID               `json:"studioId"`
	BroadcastListID uuid.UUID               `json:"broadcastListId"`
	ListName        string                  `json:"listName"`
	Channel         BroadcastChannel        `json:"channel"`
	Subject         string                  `json:"subject,omitempty"` // email only
	Body            string                  `json:"body"`
	Attachments     []Attachment            `json:"attachments"`
	ScheduledFor    time.Time               `json:"scheduledFor"`
	Status          BroadcastCampaignStatus `json:"status"`
	TotalCount      int                     `json:"totalCount"`
	EnqueuedCount   int                     `json:"enqueuedCount"`
	CreatedAt       time.Time               `json:"createdAt"`
	UpdatedAt       time.Time               `json:"updatedAt"`
}

type BroadcastRecipientStatus string

const (
	BroadcastRecipientPending  BroadcastRecipientStatus = "pending"
	BroadcastRecipientEnqueued BroadcastRecipientStatus = "enqueued"
	BroadcastRecipientFailed   BroadcastRecipientStatus = "failed"
)
