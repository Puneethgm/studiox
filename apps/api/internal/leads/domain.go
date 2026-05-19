package leads

import (
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
)

type Campaign struct {
	ID            uuid.UUID `json:"id"`
	StudioID      uuid.UUID `json:"studioId"`
	StudioSlug    string    `json:"studioSlug,omitempty"`
	StudioName    string    `json:"studioName,omitempty"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	FitnessPlans  []string  `json:"fitnessPlans"`
	Active        bool      `json:"active"`
	CreatedBy     uuid.UUID `json:"createdBy"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	LeadCount     int       `json:"leadCount,omitempty"`
}

type LeadStatus string

const (
	StatusNew         LeadStatus = "new"
	StatusContacted   LeadStatus = "contacted"
	StatusTrialBooked LeadStatus = "trial_booked"
	StatusMember      LeadStatus = "member"
	StatusDropped     LeadStatus = "dropped"
)

func (s LeadStatus) Valid() bool {
	switch s {
	case StatusNew, StatusContacted, StatusTrialBooked, StatusMember, StatusDropped:
		return true
	}
	return false
}

type Lead struct {
	ID           uuid.UUID  `json:"id"`
	StudioID     uuid.UUID  `json:"studioId"`
	StudioName   string     `json:"studioName,omitempty"`
	StudioSlug   string     `json:"studioSlug,omitempty"`
	CampaignID   uuid.UUID  `json:"campaignId"`
	CampaignName string     `json:"campaignName,omitempty"`
	CampaignSlug string     `json:"campaignSlug,omitempty"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	Phone        string     `json:"phone"`
	FitnessPlan  string     `json:"fitnessPlan"`
	Goals        string     `json:"goals"`
	Source       string     `json:"source"`
	Status       LeadStatus `json:"status"`
	Notes        string     `json:"notes"`
	Referrer     string     `json:"referrer,omitempty"`
	UserAgent    string     `json:"userAgent,omitempty"`
	IPAddress    *net.IP    `json:"ipAddress,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

var (
	ErrCampaignNotFound = errors.New("campaign not found")
	ErrLeadNotFound     = errors.New("lead not found")
	ErrSlugTaken        = errors.New("slug already in use within this studio")
	ErrInvalidPlan      = errors.New("fitness plan not offered by this campaign")
)
