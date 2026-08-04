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
	GreetingMessage     string `json:"greetingMessage"`
	TrialAmountSGD      int    `json:"trialAmountSgd"`
	BookingHeroImageURL string `json:"bookingHeroImageUrl"`
	BookingHeroVideoURL string `json:"bookingHeroVideoUrl"`
	TrialConfirmationMessage      string `json:"trialConfirmationMessage"`
	MembershipConfirmationMessage string `json:"membershipConfirmationMessage"`
	// Glofox membership/plan-code mapping — when set, a real Stripe payment
	// creates an actual credit-pack/membership purchase in Glofox (not just
	// a bare lead record). Looked up once via Glofox's own dashboard/API.
	TrialGlofoxMembershipID      string `json:"trialGlofoxMembershipId"`
	TrialGlofoxPlanCode          string `json:"trialGlofoxPlanCode"`
	MembershipGlofoxMembershipID string `json:"membershipGlofoxMembershipId"`
	MembershipGlofoxPlanCode     string `json:"membershipGlofoxPlanCode"`

	// Optional summary fields used by list endpoints.
	CampaignCount int `json:"campaignCount,omitempty"`
	LeadCount     int `json:"leadCount,omitempty"`
}

var (
	ErrNotFound  = errors.New("studio not found")
	ErrSlugTaken = errors.New("studio slug already in use")
)
