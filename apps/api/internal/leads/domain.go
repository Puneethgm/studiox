package leads

import (
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
)

type Campaign struct {
	ID           uuid.UUID `json:"id"`
	StudioID     uuid.UUID `json:"studioId"`
	StudioSlug   string    `json:"studioSlug,omitempty"`
	StudioName   string    `json:"studioName,omitempty"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	FitnessPlans []string  `json:"fitnessPlans"`
	Active       bool      `json:"active"`
	CreatedBy    uuid.UUID `json:"createdBy"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	LeadCount    int       `json:"leadCount,omitempty"`
}

type LeadStatus string

const (
	StatusNew         LeadStatus = "new"
	StatusContacted   LeadStatus = "contacted"
	StatusTrialBooked LeadStatus = "trial_booked"
	StatusMember      LeadStatus = "member"
	StatusDropped     LeadStatus = "dropped"
	StatusPaused      LeadStatus = "paused"
)

func (s LeadStatus) Valid() bool {
	switch s {
	case StatusNew, StatusContacted, StatusTrialBooked, StatusMember, StatusDropped, StatusPaused:
		return true
	}
	return false
}

type Lead struct {
	ID               uuid.UUID  `json:"id"`
	StudioID         uuid.UUID  `json:"studioId"`
	StudioName       string     `json:"studioName,omitempty"`
	StudioSlug       string     `json:"studioSlug,omitempty"`
	CampaignID       uuid.UUID  `json:"campaignId"`
	CampaignName     string     `json:"campaignName,omitempty"`
	CampaignSlug     string     `json:"campaignSlug,omitempty"`
	Name             string     `json:"name"`
	FirstName        string     `json:"firstName"`
	LastName         string     `json:"lastName"`
	Email            string     `json:"email"`
	Phone            string     `json:"phone"`
	FitnessPlan      string     `json:"fitnessPlan"`
	Goals            string     `json:"goals"`
	Source           string     `json:"source"`
	Status           LeadStatus `json:"status"`
	Currency         string     `json:"currency"`
	Notes            string     `json:"notes"`
	ContactAttempts  int        `json:"contactAttempts"`
	LastContactedAt  *time.Time `json:"lastContactedAt,omitempty"`
	ContactMade      bool       `json:"contactMade"`
	HotLead          bool       `json:"hotLead"`
	TrialPurchased   bool       `json:"trialPurchased"`
	AutoContactStage string     `json:"autoContactStage"`
	DNDEnabled       bool       `json:"dndEnabled"`
	AssignedTo       string     `json:"assignedTo"`
	TrialAttended    bool       `json:"trialAttended"`
	MemberSold       bool       `json:"memberSold"`
	MonthlyFee       float64    `json:"monthlyFee"`
	Offer            string     `json:"offer"`
	FurtherNotes     string     `json:"furtherNotes"`
	Referrer         string     `json:"referrer,omitempty"`
	UserAgent        string     `json:"userAgent,omitempty"`
	IPAddress        *net.IP    `json:"ipAddress,omitempty"`
	// Gender / DateOfBirth are collected on the pre-payment trial details
	// page, right before Stripe checkout — optional, used to send real
	// values to Glofox instead of a neutral placeholder birth date.
	Gender       string     `json:"gender,omitempty"`
	DateOfBirth  *time.Time `json:"dateOfBirth,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type StudioSheetsSettings struct {
	ID            uuid.UUID `json:"id"`
	StudioID      uuid.UUID `json:"studioId"`
	SpreadsheetID string    `json:"spreadsheetId"`
	TabName       string    `json:"tabName"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// ExternalLeadsSheetSettings configures read-only polling of a third-party
// company's Google Sheet for new leads. Unlike StudioSheetsSettings (which is
// our own export destination), we never write to this sheet — column letters
// are configurable since we don't control its layout.
type ExternalLeadsSheetSettings struct {
	ID              uuid.UUID `json:"id"`
	StudioID        uuid.UUID `json:"studioId"`
	SpreadsheetID   string    `json:"spreadsheetId"`
	TabName         string    `json:"tabName"`
	NameColumn      string    `json:"nameColumn"`
	FirstNameColumn string    `json:"firstNameColumn"`
	LastNameColumn  string    `json:"lastNameColumn"`
	EmailColumn     string    `json:"emailColumn"`
	PhoneColumn     string    `json:"phoneColumn"`
	SourceColumn    string    `json:"sourceColumn"`
	NotesColumn     string    `json:"notesColumn"`
	DateColumn      string    `json:"dateColumn"`
	HotLeadColumn   string    `json:"hotLeadColumn"`
	TrialPurchasedColumn string `json:"trialPurchasedColumn"`
	ContinueAIAfterGreeting bool `json:"continueAiAfterGreeting"`
	Active          bool      `json:"active"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

var (
	ErrCampaignNotFound = errors.New("campaign not found")
	ErrLeadNotFound     = errors.New("lead not found")
	ErrSlugTaken        = errors.New("slug already in use within this studio")
	ErrInvalidPlan      = errors.New("fitness plan not offered by this campaign")
)

type AnalyticsSummary struct {
	TotalLeads                 int                 `json:"totalLeads"`
	NewLeads                   int                 `json:"newLeads"`
	TrialBookedLeads           int                 `json:"trialBookedLeads"`
	MemberLeads                int                 `json:"memberLeads"`
	DroppedLeads               int                 `json:"droppedLeads"`
	PausedLeads                int                 `json:"pausedLeads"`
	TrialToMemberRate          float64             `json:"trialToMemberRate"`
	DroppedRate                float64             `json:"droppedRate"`
	PausedRate                 float64             `json:"pausedRate"`
	FollowupsRequired          int                 `json:"followupsRequired"`
	UnrespondedMessages        int                 `json:"unrespondedMessages"`
	AvgResponseTimeLapseSecs   float64             `json:"avgResponseTimeLapseSecs"`
	LeadToTrialTimeLapseSecs   float64             `json:"leadToTrialTimeLapseSecs"`
	TrialToMemberTimeLapseSecs float64             `json:"trialToMemberTimeLapseSecs"`
	ByCampaign                 []CampaignAnalytics `json:"byCampaign"`
	ByPlatform                 []PlatformAnalytics `json:"byPlatform"`
}

type CampaignAnalytics struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Slug           string  `json:"slug"`
	TotalLeads     int     `json:"totalLeads"`
	ConvertedLeads int     `json:"convertedLeads"`
	ConversionRate float64 `json:"conversionRate"`
}

type PlatformAnalytics struct {
	Platform       string  `json:"platform"`
	TotalLeads     int     `json:"totalLeads"`
	ConvertedLeads int     `json:"convertedLeads"`
	ConversionRate float64 `json:"conversionRate"`
}
