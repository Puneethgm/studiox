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

type KnowledgeBaseFile struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Text     string `json:"text"`
	Platform string `json:"platform"` // "all","whatsapp","instagram","facebook","sms"
}

type Studio struct {
	ID                   uuid.UUID           `json:"id"`
	Slug                 string              `json:"slug"`
	Name                 string              `json:"name"`
	BrandColor           string              `json:"brandColor"`
	LogoURL              string              `json:"logoUrl"`
	ContactEmail         string              `json:"contactEmail"`
	ContactPhone         string              `json:"contactPhone"`
	Active               bool                `json:"active"`
	ManagedBy1Hero       bool                `json:"managedBy1Hero"`
	CreatedAt            time.Time           `json:"createdAt"`
	UpdatedAt            time.Time           `json:"updatedAt"`
	AvailabilitySlots    []AvailabilitySlot  `json:"availabilitySlots"`
	AvailabilityTimezone string              `json:"availabilityTimezone"`
	GeminiAPIKey         string              `json:"geminiApiKey"`
	GroqAPIKey           string              `json:"groqApiKey"`
	ClaudeAPIKey         string              `json:"claudeApiKey"`
	MetaAppID            string              `json:"metaAppId"`
	MetaAppSecret        string              `json:"metaAppSecret"`
	GoogleClientID       string              `json:"googleClientId"`
	GoogleClientSecret   string              `json:"googleClientSecret"`
	GoogleDeveloperToken string              `json:"googleDeveloperToken"`
	StripeAccountID      string              `json:"stripeAccountId"`
	StripeSecretKey      string              `json:"stripeSecretKey"`
	StripePublishableKey string              `json:"stripePublishableKey"`
	StripeWebhookSecret  string              `json:"stripeWebhookSecret"`
	SubscriptionTier     string              `json:"subscriptionTier"`
	SocialPlannerEnabled bool                `json:"socialPlannerEnabled"`
	KnowledgeBase        string              `json:"knowledgeBase"`
	KnowledgeBaseFiles   []KnowledgeBaseFile `json:"knowledgeBaseFiles"`
	// GreetingMessage is sent automatically on the first inbound message of a new conversation.
	GreetingMessage               string `json:"greetingMessage"`
	TrialAmountSGD                int    `json:"trialAmountSgd"`
	BookingHeroImageURL           string `json:"bookingHeroImageUrl"`
	BookingHeroVideoURL           string `json:"bookingHeroVideoUrl"`
	TrialConfirmationMessage      string `json:"trialConfirmationMessage"`
	MembershipConfirmationMessage string `json:"membershipConfirmationMessage"`
	// Glofox membership/plan-code mapping — when set, a real Stripe payment
	// creates an actual credit-pack/membership purchase in Glofox (not just
	// a bare lead record). Looked up once via Glofox's own dashboard/API.
	TrialGlofoxMembershipID      string `json:"trialGlofoxMembershipId"`
	TrialGlofoxPlanCode          string `json:"trialGlofoxPlanCode"`
	MembershipGlofoxMembershipID string `json:"membershipGlofoxMembershipId"`
	MembershipGlofoxPlanCode     string `json:"membershipGlofoxPlanCode"`

	// CommunicationStyleProfile is a short writeup of how this studio's staff
	// actually talk to customers, distilled by the style worker from their own
	// past studio_user replies (see internal/studios/style_worker.go). Editable
	// by the studio admin, same as the rest of the Knowledge Base page.
	CommunicationStyleProfile string     `json:"communicationStyleProfile"`
	StyleProfileUpdatedAt     *time.Time `json:"styleProfileUpdatedAt,omitempty"`
	// StyleRefreshIntervalMinutes is how often (at minimum) the style worker
	// re-learns this studio's profile, editable on the Knowledge Base page —
	// see studios.Repo.ListStudiosNeedingStyleRefresh for how it combines
	// with the new-replies threshold.
	StyleRefreshIntervalMinutes int `json:"styleRefreshIntervalMinutes"`

	// ProgramStartDate anchors a week-by-week program document (see
	// ParseProgramSchedule/studio_program_sessions) to real calendar dates,
	// so the AI worker can compute "today = Week X, [day]" itself with real
	// date math instead of asking an LLM to find the right day among many
	// near-identical weekly entries via semantic search — see
	// Repo.GetProgramSession. Nil until a studio admin sets it (or until a
	// program-schedule-shaped document is uploaded with no date set yet).
	ProgramStartDate *time.Time `json:"programStartDate,omitempty"`

	// Optional summary fields used by list endpoints.
	CampaignCount int `json:"campaignCount,omitempty"`
	LeadCount     int `json:"leadCount,omitempty"`
}

var (
	ErrNotFound  = errors.New("studio not found")
	ErrSlugTaken = errors.New("studio slug already in use")
)
