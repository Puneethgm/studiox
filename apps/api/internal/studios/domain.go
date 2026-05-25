package studios

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type AvailabilitySlot struct {
	Day   string   `json:"day"`
	Times []string `json:"times"`
}

type Studio struct {
	ID           uuid.UUID `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	BrandColor   string    `json:"brandColor"`
	LogoURL      string    `json:"logoUrl"`
	ContactEmail string    `json:"contactEmail"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	AvailabilitySlots []AvailabilitySlot `json:"availabilitySlots"`
	AvailabilityTimezone string `json:"availabilityTimezone"`
	GeminiAPIKey string    `json:"geminiApiKey"`
	MetaAppID    string    `json:"metaAppId"`
	MetaAppSecret string   `json:"metaAppSecret"`

	// Optional summary fields used by list endpoints.
	CampaignCount int `json:"campaignCount,omitempty"`
	LeadCount     int `json:"leadCount,omitempty"`
}

var (
	ErrNotFound  = errors.New("studio not found")
	ErrSlugTaken = errors.New("studio slug already in use")
)
