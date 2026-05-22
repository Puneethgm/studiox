package messaging

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ----- channel kinds -----

type ChannelKind string

const (
	KindWhatsAppMeta  ChannelKind = "whatsapp_meta"
	KindInstagramMeta ChannelKind = "instagram_meta"
	KindMessengerMeta ChannelKind = "messenger_meta"
	KindXDM           ChannelKind = "x_dm"
	KindSMS           ChannelKind = "sms"
)

func (k ChannelKind) Valid() bool {
	switch k {
	case KindWhatsAppMeta, KindInstagramMeta, KindMessengerMeta, KindXDM, KindSMS:
		return true
	}
	return false
}

// ----- identity -----

type IdentityKind string

const (
	IdentityPhone  IdentityKind = "phone"
	IdentityEmail  IdentityKind = "email"
	IdentityIGPSID IdentityKind = "ig_psid"
	IdentityFBPSID IdentityKind = "fb_psid"
	IdentityXID    IdentityKind = "x_id"
)

type ContactIdentity struct {
	ID          uuid.UUID    `json:"id"`
	StudioID    uuid.UUID    `json:"studioId"`
	LeadID      *uuid.UUID   `json:"leadId,omitempty"`
	Kind        IdentityKind `json:"kind"`
	Value       string       `json:"value"`
	DisplayName string       `json:"displayName"`
	CreatedAt   time.Time    `json:"createdAt"`
}

// ----- channel account -----

type ChannelStatus string

const (
	StatusActive       ChannelStatus = "active"
	StatusPaused       ChannelStatus = "paused"
	StatusDisconnected ChannelStatus = "disconnected"
	StatusError        ChannelStatus = "error"
)

type ChannelAccount struct {
	ID             uuid.UUID     `json:"id"`
	StudioID       uuid.UUID     `json:"studioId"`
	Kind           ChannelKind   `json:"kind"`
	BSP            string        `json:"bsp"`
	ExternalID     string        `json:"externalId"`    // WhatsApp: phone_number_id
	ParentID       string        `json:"parentId"`      // WhatsApp: WABA id
	DisplayHandle  string        `json:"displayHandle"` // human-readable phone or @handle
	Status         ChannelStatus `json:"status"`
	LastError      string        `json:"lastError,omitempty"`
	ConnectedAt    time.Time     `json:"connectedAt"`
	DisconnectedAt *time.Time    `json:"disconnectedAt,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`

	// AccessToken is the *decrypted* access token. Populated only by the repo
	// methods that need it (e.g. OutboxClaim) — list endpoints leave it empty.
	AccessToken string `json:"-"`
}

// ----- conversation -----

type ConvStatus string

const (
	ConvOpen    ConvStatus = "open"
	ConvSnoozed ConvStatus = "snoozed"
	ConvClosed  ConvStatus = "closed"
)

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

type Conversation struct {
	ID                   uuid.UUID   `json:"id"`
	StudioID             uuid.UUID   `json:"studioId"`
	ChannelAccountID     uuid.UUID   `json:"channelAccountId"`
	ChannelKind          ChannelKind `json:"channelKind"`             // joined for the UI
	ChannelHandle        string      `json:"channelHandle,omitempty"` // joined for the UI
	ContactIdentityID    uuid.UUID   `json:"contactIdentityId"`
	ContactDisplayName   string      `json:"contactDisplayName"`
	ContactValue         string      `json:"contactValue"` // phone or handle
	ExternalThreadID     string      `json:"externalThreadId"`
	LeadID               *uuid.UUID  `json:"leadId,omitempty"`
	Status               ConvStatus  `json:"status"`
	AssignedTo           *uuid.UUID  `json:"assignedTo,omitempty"`
	UnreadCount          int         `json:"unreadCount"`
	LastMessageAt        time.Time   `json:"lastMessageAt"`
	LastMessagePreview   string      `json:"lastMessagePreview"`
	LastMessageDirection *Direction  `json:"lastMessageDirection,omitempty"`
	CreatedAt            time.Time   `json:"createdAt"`
	UpdatedAt            time.Time   `json:"updatedAt"`
}

// ----- message -----

type SourceKind string

const (
	SourceCustomer   SourceKind = "customer"
	SourceStudioUser SourceKind = "studio_user"
	SourceAutomation SourceKind = "automation"
	SourceAI         SourceKind = "ai"
)

type MessageStatus string

const (
	MsgPending   MessageStatus = "pending"
	MsgSent      MessageStatus = "sent"
	MsgDelivered MessageStatus = "delivered"
	MsgRead      MessageStatus = "read"
	MsgFailed    MessageStatus = "failed"
)

type Attachment struct {
	Type string `json:"type"` // image | video | audio | document
	URL  string `json:"url"`
	Mime string `json:"mime,omitempty"`
	Name string `json:"name,omitempty"`
}

type Message struct {
	ID             uuid.UUID     `json:"id"`
	ConversationID uuid.UUID     `json:"conversationId"`
	StudioID       uuid.UUID     `json:"studioId"`
	Direction      Direction     `json:"direction"`
	SourceKind     SourceKind    `json:"sourceKind"`
	SourceUserID   *uuid.UUID    `json:"sourceUserId,omitempty"`
	SourceRef      string        `json:"sourceRef,omitempty"`
	Body           string        `json:"body"`
	Attachments    []Attachment  `json:"attachments,omitempty"`
	ExternalID     string        `json:"externalId,omitempty"`
	InReplyTo      string        `json:"inReplyTo,omitempty"`
	Status         MessageStatus `json:"status"`
	FailureReason  string        `json:"failureReason,omitempty"`
	SentAt         time.Time     `json:"sentAt"`
	DeliveredAt    *time.Time    `json:"deliveredAt,omitempty"`
	ReadAt         *time.Time    `json:"readAt,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"`
}

// ----- outbound job -----

type OutboundJobStatus string

const (
	JobPending OutboundJobStatus = "pending"
	JobSent    OutboundJobStatus = "sent"
	JobFailed  OutboundJobStatus = "failed"
	JobDead    OutboundJobStatus = "dead"
)

type OutboundJob struct {
	ID             int64
	StudioID       uuid.UUID
	ConversationID uuid.UUID
	Body           string
	Attachments    []Attachment
	TemplateName   string
	SourceKind     SourceKind
	SourceUserID   *uuid.UUID
	SourceRef      string
	ScheduledFor   time.Time
	Status         OutboundJobStatus
	Attempts       int
	NextAttemptAt  time.Time
	LastError      string
	MessageID      *uuid.UUID
	CreatedAt      time.Time
	SentAt         *time.Time
}

// ----- errors -----

var (
	ErrNotFound        = errors.New("not found")
	ErrChannelMismatch = errors.New("channel does not belong to this studio")
)

// ----- trigger link -----

type TriggerLink struct {
	ID        uuid.UUID `json:"id"`
	StudioID  uuid.UUID `json:"studioId"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Clicks    int       `json:"clicks"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TriggerLinkClick struct {
	ID        uuid.UUID  `json:"id"`
	LinkID    uuid.UUID  `json:"linkId"`
	LeadID    *uuid.UUID `json:"leadId,omitempty"`
	ClickedAt time.Time  `json:"clickedAt"`
}

// ----- message template -----

type MessageTemplate struct {
	ID                   uuid.UUID    `json:"id"`
	StudioID             uuid.UUID    `json:"studioId"`
	Name                 string       `json:"name"`
	Body                 string       `json:"body"`
	ChannelKinds         []string     `json:"channelKinds"`
	Attachments          []Attachment `json:"attachments"`
	WhatsAppTemplateName string       `json:"whatsappTemplateName,omitempty"`
	WhatsAppTemplateLang string       `json:"whatsappTemplateLang,omitempty"`
	CreatedAt            time.Time    `json:"createdAt"`
	UpdatedAt            time.Time    `json:"updatedAt"`
}
