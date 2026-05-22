// Package channels holds per-channel adapters: each implements `Sender` for
// outbound and exposes a webhook-payload parser for inbound. The package is
// intentionally decoupled from the messaging package — Sender takes primitives
// (token, channel id, recipient, body) so we never get an import cycle.
package channels

import (
	"context"
	"errors"
)

type Attachment struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	Name string `json:"name"` // original filename, required for WhatsApp document messages
}

// SendResult is what the dispatcher needs to update its outbound_jobs row.
type SendResult struct {
	// ExternalID is the channel-native message id (e.g. Meta wamid).
	ExternalID string
}

// Sender abstracts the outbound side of every channel. Implementations are
// stateless given the credentials per call so the same instance can serve
// many studios.
type Sender interface {
	// SendText delivers a plain-text message.
	//
	//   accessToken:        the studio's per-channel access token (decrypted)
	//   channelExternalID:  e.g. WhatsApp phone_number_id
	//   recipient:          e.g. customer phone in international format (no '+')
	//   body:               UTF-8 text
	SendText(ctx context.Context, accessToken, channelExternalID, recipient, body string, attachments []Attachment) (*SendResult, error)
}

// ErrInvalidCredentials is returned by adapters when the credentials are
// missing or rejected by the platform — the worker uses this to mark the
// channel as 'error' rather than retry forever.
var ErrInvalidCredentials = errors.New("invalid or revoked channel credentials")
