package studios

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/s3"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/client"
	stripeoauth "github.com/stripe/stripe-go/v78/oauth"
)

type Handler struct {
	svc             *Service
	credentialsPath string
	s3Uploader      *s3.Uploader
}

func NewHandler(svc *Service, credentialsPath string, s3Uploader *s3.Uploader) *Handler {
	return &Handler{svc: svc, credentialsPath: credentialsPath, s3Uploader: s3Uploader}
}

// studioResponse is the safe API shape for Studio — never returns raw secret values.
type studioResponse struct {
	ID                            uuid.UUID           `json:"id"`
	Slug                          string              `json:"slug"`
	Name                          string              `json:"name"`
	BrandColor                    string              `json:"brandColor"`
	LogoURL                       string              `json:"logoUrl"`
	ContactEmail                  string              `json:"contactEmail"`
	ContactPhone                  string              `json:"contactPhone"`
	Active                        bool                `json:"active"`
	ManagedBy1Hero                bool                `json:"managedBy1Hero"`
	CreatedAt                     time.Time           `json:"createdAt"`
	UpdatedAt                     time.Time           `json:"updatedAt"`
	AvailabilitySlots             []AvailabilitySlot  `json:"availabilitySlots"`
	AvailabilityTimezone          string              `json:"availabilityTimezone"`
	MetaAppID                     string              `json:"metaAppId"`
	GoogleClientID                string              `json:"googleClientId"`
	StripeAccountID               string              `json:"stripeAccountId"`
	StripePublishableKey          string              `json:"stripePublishableKey"`
	SubscriptionTier              string              `json:"subscriptionTier"`
	SocialPlannerEnabled          bool                `json:"socialPlannerEnabled"`
	KnowledgeBase                 string              `json:"knowledgeBase"`
	KnowledgeBaseFiles            []KnowledgeBaseFile `json:"knowledgeBaseFiles"`
	GreetingMessage               string              `json:"greetingMessage"`
	TrialAmountSGD                int                 `json:"trialAmountSgd"`
	BookingHeroImageURL           string              `json:"bookingHeroImageUrl"`
	BookingHeroVideoURL           string              `json:"bookingHeroVideoUrl"`
	TrialConfirmationMessage      string              `json:"trialConfirmationMessage"`
	MembershipConfirmationMessage string              `json:"membershipConfirmationMessage"`
	TrialGlofoxMembershipID       string              `json:"trialGlofoxMembershipId"`
	TrialGlofoxPlanCode           string              `json:"trialGlofoxPlanCode"`
	MembershipGlofoxMembershipID  string              `json:"membershipGlofoxMembershipId"`
	MembershipGlofoxPlanCode      string              `json:"membershipGlofoxPlanCode"`
	CampaignCount                 int                 `json:"campaignCount,omitempty"`
	LeadCount                     int                 `json:"leadCount,omitempty"`
	ID                            uuid.UUID           `json:"id"`
	Slug                          string              `json:"slug"`
	Name                          string              `json:"name"`
	BrandColor                    string              `json:"brandColor"`
	LogoURL                       string              `json:"logoUrl"`
	ContactEmail                  string              `json:"contactEmail"`
	ContactPhone                  string              `json:"contactPhone"`
	Active                        bool                `json:"active"`
	ManagedBy1Hero                bool                `json:"managedBy1Hero"`
	CreatedAt                     time.Time           `json:"createdAt"`
	UpdatedAt                     time.Time           `json:"updatedAt"`
	AvailabilitySlots             []AvailabilitySlot  `json:"availabilitySlots"`
	AvailabilityTimezone          string              `json:"availabilityTimezone"`
	MetaAppID                     string              `json:"metaAppId"`
	GoogleClientID                string              `json:"googleClientId"`
	StripeAccountID               string              `json:"stripeAccountId"`
	StripePublishableKey          string              `json:"stripePublishableKey"`
	SubscriptionTier              string              `json:"subscriptionTier"`
	SocialPlannerEnabled          bool                `json:"socialPlannerEnabled"`
	KnowledgeBase                 string              `json:"knowledgeBase"`
	KnowledgeBaseFiles            []KnowledgeBaseFile `json:"knowledgeBaseFiles"`
	GreetingMessage               string              `json:"greetingMessage"`
	TrialAmountSGD                int                 `json:"trialAmountSgd"`
	BookingHeroImageURL           string              `json:"bookingHeroImageUrl"`
	BookingHeroVideoURL           string              `json:"bookingHeroVideoUrl"`
	TrialConfirmationMessage      string              `json:"trialConfirmationMessage"`
	MembershipConfirmationMessage string              `json:"membershipConfirmationMessage"`
	TrialGlofoxMembershipID       string              `json:"trialGlofoxMembershipId"`
	TrialGlofoxPlanCode           string              `json:"trialGlofoxPlanCode"`
	MembershipGlofoxMembershipID  string              `json:"membershipGlofoxMembershipId"`
	MembershipGlofoxPlanCode      string              `json:"membershipGlofoxPlanCode"`
	CommunicationStyleProfile     string              `json:"communicationStyleProfile"`
	StyleProfileUpdatedAt         *time.Time          `json:"styleProfileUpdatedAt,omitempty"`
	CampaignCount                 int                 `json:"campaignCount,omitempty"`
	LeadCount                     int                 `json:"leadCount,omitempty"`
	// Presence indicators — actual secret values are never returned.
	HasGeminiApiKey         bool `json:"hasGeminiApiKey"`
	HasGroqApiKey           bool `json:"hasGroqApiKey"`
	HasMetaAppSecret        bool `json:"hasMetaAppSecret"`
	HasGoogleClientSecret   bool `json:"hasGoogleClientSecret"`
	HasGoogleDeveloperToken bool `json:"hasGoogleDeveloperToken"`
	HasStripeSecretKey      bool `json:"hasStripeSecretKey"`
	HasStripeWebhookSecret  bool `json:"hasStripeWebhookSecret"`
}

func toStudioResponse(s *Studio) studioResponse {
	return studioResponse{
		ID:                            s.ID,
		Slug:                          s.Slug,
		Name:                          s.Name,
		BrandColor:                    s.BrandColor,
		LogoURL:                       s.LogoURL,
		ContactEmail:                  s.ContactEmail,
		ContactPhone:                  s.ContactPhone,
		Active:                        s.Active,
		ManagedBy1Hero:                s.ManagedBy1Hero,
		CreatedAt:                     s.CreatedAt,
		UpdatedAt:                     s.UpdatedAt,
		AvailabilitySlots:             s.AvailabilitySlots,
		AvailabilityTimezone:          s.AvailabilityTimezone,
		MetaAppID:                     s.MetaAppID,
		GoogleClientID:                s.GoogleClientID,
		StripeAccountID:               s.StripeAccountID,
		StripePublishableKey:          s.StripePublishableKey,
		SubscriptionTier:              s.SubscriptionTier,
		SocialPlannerEnabled:          s.SocialPlannerEnabled,
		KnowledgeBase:                 s.KnowledgeBase,
		KnowledgeBaseFiles:            s.KnowledgeBaseFiles,
		GreetingMessage:               s.GreetingMessage,
		TrialAmountSGD:                s.TrialAmountSGD,
		BookingHeroImageURL:           s.BookingHeroImageURL,
		BookingHeroVideoURL:           s.BookingHeroVideoURL,
		ID:                            s.ID,
		Slug:                          s.Slug,
		Name:                          s.Name,
		BrandColor:                    s.BrandColor,
		LogoURL:                       s.LogoURL,
		ContactEmail:                  s.ContactEmail,
		ContactPhone:                  s.ContactPhone,
		Active:                        s.Active,
		ManagedBy1Hero:                s.ManagedBy1Hero,
		CreatedAt:                     s.CreatedAt,
		UpdatedAt:                     s.UpdatedAt,
		AvailabilitySlots:             s.AvailabilitySlots,
		AvailabilityTimezone:          s.AvailabilityTimezone,
		MetaAppID:                     s.MetaAppID,
		GoogleClientID:                s.GoogleClientID,
		StripeAccountID:               s.StripeAccountID,
		StripePublishableKey:          s.StripePublishableKey,
		SubscriptionTier:              s.SubscriptionTier,
		SocialPlannerEnabled:          s.SocialPlannerEnabled,
		KnowledgeBase:                 s.KnowledgeBase,
		KnowledgeBaseFiles:            s.KnowledgeBaseFiles,
		GreetingMessage:               s.GreetingMessage,
		TrialAmountSGD:                s.TrialAmountSGD,
		BookingHeroImageURL:           s.BookingHeroImageURL,
		BookingHeroVideoURL:           s.BookingHeroVideoURL,
		TrialConfirmationMessage:      s.TrialConfirmationMessage,
		MembershipConfirmationMessage: s.MembershipConfirmationMessage,
		TrialGlofoxMembershipID:       s.TrialGlofoxMembershipID,
		TrialGlofoxPlanCode:           s.TrialGlofoxPlanCode,
		MembershipGlofoxMembershipID:  s.MembershipGlofoxMembershipID,
		MembershipGlofoxPlanCode:      s.MembershipGlofoxPlanCode,
		CampaignCount:                 s.CampaignCount,
		LeadCount:                     s.LeadCount,
		HasGeminiApiKey:               s.GeminiAPIKey != "",
		HasGroqApiKey:                 s.GroqAPIKey != "",
		HasMetaAppSecret:              s.MetaAppSecret != "",
		HasGoogleClientSecret:         s.GoogleClientSecret != "",
		HasGoogleDeveloperToken:       s.GoogleDeveloperToken != "",
		HasStripeSecretKey:            s.StripeSecretKey != "",
		HasStripeWebhookSecret:        s.StripeWebhookSecret != "",
		TrialGlofoxMembershipID:       s.TrialGlofoxMembershipID,
		TrialGlofoxPlanCode:           s.TrialGlofoxPlanCode,
		MembershipGlofoxMembershipID:  s.MembershipGlofoxMembershipID,
		MembershipGlofoxPlanCode:      s.MembershipGlofoxPlanCode,
		CommunicationStyleProfile:     s.CommunicationStyleProfile,
		StyleProfileUpdatedAt:         s.StyleProfileUpdatedAt,
		CampaignCount:                 s.CampaignCount,
		LeadCount:                     s.LeadCount,
		HasGeminiApiKey:               s.GeminiAPIKey != "",
		HasGroqApiKey:                 s.GroqAPIKey != "",
		HasMetaAppSecret:              s.MetaAppSecret != "",
		HasGoogleClientSecret:         s.GoogleClientSecret != "",
		HasGoogleDeveloperToken:       s.GoogleDeveloperToken != "",
		HasStripeSecretKey:            s.StripeSecretKey != "",
		HasStripeWebhookSecret:        s.StripeWebhookSecret != "",
	}
}

// AdminRoutes are super-admin only — only the platform owner manages studios.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Use(identity.RequireRole(identity.RoleSuperAdmin))
	r.Get("/studios", h.list)
	r.Post("/studios", h.create)
	r.Get("/studios/{id}", h.get)
	r.Patch("/studios/{id}", h.update)

	r.Get("/google-credentials", h.getGoogleCredentials)
	r.Post("/google-credentials", h.uploadGoogleCredentials)
}

func (h *Handler) SelfRoutes(r chi.Router) {
	r.Get("/studios/{id}", h.getScoped)
	r.Patch("/studios/{id}", h.updateScoped)
	r.Post("/studios/{id}/logo", h.uploadLogo)

	// Plans routes
	r.Get("/studios/{id}/plans", h.listPlans)
	r.Post("/studios/{id}/plans", h.createPlan)
	r.Put("/studios/{id}/plans/{planId}", h.updatePlan)
	r.Delete("/studios/{id}/plans/{planId}", h.deletePlan)
	r.Get("/studios/{id}/payments", h.getPayments)
	r.Post("/studios/{id}/payments/stripe", h.linkStripe)
	// Platform Plans route
	r.Put("/studios/global/plans", h.UpdatePlatformPlans)

	r.Get("/studios/{id}/billing/history", h.getBillingHistory)
	r.Post("/studios/{id}/billing/upgrade", h.UpgradeStudioPlan)
	r.Post("/studios/{id}/billing/portal", h.CreatePortalSession)
	r.Post("/studios/{id}/billing/sync", h.SyncBillingStatus)
	r.Post("/studios/{id}/trial-checkout", h.createTrialCheckout)
	r.Get("/studios/{id}/trial-page-layout", h.getTrialPageLayout)
	r.Put("/studios/{id}/trial-page-layout", h.putTrialPageLayout)

	// Account deletion
	r.Delete("/studios/{id}/delete-account", h.deleteAccount)
}

// PublicRoutes expose the studio's brand info for the public form to render.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/public/studios/{slug}", h.publicGet)
	r.Get("/public/studios/{slug}/plans", h.publicGetPlans)
	r.Post("/public/studios/{slug}/checkout", h.publicCreateCheckout)
	r.Post("/public/studios/{slug}/payment-intent", h.publicCreatePaymentIntent)
	r.Get("/public/studios/{slug}/payment-receipt/{piId}", h.publicGetPaymentReceipt)
	r.Get("/public/platform/plans", h.GetPlatformPlans)
	r.Get("/public/studios/{slug}/trial-page-layout", h.publicGetTrialPageLayout)
	r.Post("/public/leads/{leadId}/trial-payment-intent", h.publicCreateTrialPaymentIntent)
}

func (h *Handler) RequireActiveStudio(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := identity.MustClaims(r.Context())
		if c.IsSuper() {
			next.ServeHTTP(w, r)
			return
		}
		if c.StudioID == nil {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "no studio bound to this user")
			return
		}

		// Prevent IDOR: Ensure the user is only accessing their own studio
		requestedStudioID := chi.URLParam(r, "studioId")
		if requestedStudioID != "" && requestedStudioID != "global" && requestedStudioID != c.StudioID.String() {
			// Also check "id" just in case the param is named "id" in some routes
			if chi.URLParam(r, "id") == "" || chi.URLParam(r, "id") != c.StudioID.String() {
				httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to access this studio")
				return
			}
		}

		s, err := h.svc.GetByID(r.Context(), *c.StudioID)
		if err != nil {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "studio not accessible")
			return
		}
		if !s.Active {
			httpx.WriteError(w, http.StatusForbidden, "studio_inactive",
				"this studio has been deactivated by the platform admin")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ----- super-admin handlers -----

type createReq struct {
	Slug                 string `json:"slug"`
	Name                 string `json:"name"`
	BrandColor           string `json:"brandColor"`
	LogoURL              string `json:"logoUrl"`
	ContactEmail         string `json:"contactEmail"`
	ContactPhone         string `json:"contactPhone"`
	AdminEmail           string `json:"adminEmail"`
	AdminPassword        string `json:"adminPassword"`
	SocialPlannerEnabled bool   `json:"socialPlannerEnabled"`
}

// create godoc
//
//	@Summary		Create a new studio
//	@Description	Creates a new studio along with its initial studio-admin account. Super-admin only.
//	@Tags			Studios
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		createReq	true	"New studio and admin account details"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"validation failed"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/studios [post]
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.BrandColor == "" {
		req.BrandColor = "#7c3aed"
	}
	res, errs, err := h.svc.CreateStudioWithAdmin(r.Context(), CreateStudioInput{
		Slug:                 req.Slug,
		Name:                 req.Name,
		BrandColor:           req.BrandColor,
		LogoURL:              req.LogoURL,
		ContactEmail:         req.ContactEmail,
		ContactPhone:         req.ContactPhone,
		AdminEmail:           req.AdminEmail,
		AdminPassword:        req.AdminPassword,
		SocialPlannerEnabled: req.SocialPlannerEnabled,
	})
	if errs != nil {
		httpx.WriteValidationError(w, errs)
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"studio":  toStudioResponse(res.Studio),
		"adminId": res.AdminID,
	})
}

// list godoc
//
//	@Summary		List all studios
//	@Description	Returns every studio on the platform. Super-admin only.
//	@Tags			Studios
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	map[string]interface{}
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/studios [get]
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	resp := make([]studioResponse, len(list))
	for i := range list {
		resp[i] = toStudioResponse(&list[i])
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"studios": resp})
}

// get godoc
//
//	@Summary		Get a studio by ID
//	@Description	Returns full studio details by ID. Super-admin only.
//	@Tags			Studios
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id	path		string	true	"Studio ID"
//	@Success		200	{object}	studioResponse
//	@Failure		400	{object}	httpx.ErrorResponse	"invalid id"
//	@Failure		404	{object}	httpx.ErrorResponse	"studio not found"
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/studios/{id} [get]
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, toStudioResponse(s))
}

type updateReq struct {
	Name                          *string              `json:"name"`
	BrandColor                    *string              `json:"brandColor"`
	LogoURL                       *string              `json:"logoUrl"`
	ContactEmail                  *string              `json:"contactEmail"`
	ContactPhone                  *string              `json:"contactPhone"`
	Active                        *bool                `json:"active"`
	ManagedBy1Hero                *bool                `json:"managedBy1Hero"`
	AvailabilitySlots             *[]AvailabilitySlot  `json:"availabilitySlots"`
	AvailabilityTimezone          *string              `json:"availabilityTimezone"`
	GeminiAPIKey                  *string              `json:"geminiApiKey"`
	GroqAPIKey                    *string              `json:"groqApiKey"`
	MetaAppID                     *string              `json:"metaAppId"`
	MetaAppSecret                 *string              `json:"metaAppSecret"`
	GoogleClientID                *string              `json:"googleClientId"`
	GoogleClientSecret            *string              `json:"googleClientSecret"`
	GoogleDeveloperToken          *string              `json:"googleDeveloperToken"`
	SocialPlannerEnabled          *bool                `json:"socialPlannerEnabled"`
	KnowledgeBase                 *string              `json:"knowledgeBase"`
	KnowledgeBaseFiles            *[]KnowledgeBaseFile `json:"knowledgeBaseFiles"`
	GreetingMessage               *string              `json:"greetingMessage"`
	TrialAmountSGD                *int                 `json:"trialAmountSgd"`
	TrialAmountINR                *int                 `json:"trialAmountInr"`
	TrialAmountUSD                *int                 `json:"trialAmountUsd"`
	BookingHeroImageURL           *string              `json:"bookingHeroImageUrl"`
	BookingHeroVideoURL           *string              `json:"bookingHeroVideoUrl"`
	TrialConfirmationMessage      *string              `json:"trialConfirmationMessage"`
	MembershipConfirmationMessage *string              `json:"membershipConfirmationMessage"`
	TrialGlofoxMembershipID       *string              `json:"trialGlofoxMembershipId"`
	TrialGlofoxPlanCode           *string              `json:"trialGlofoxPlanCode"`
	MembershipGlofoxMembershipID  *string              `json:"membershipGlofoxMembershipId"`
	MembershipGlofoxPlanCode      *string              `json:"membershipGlofoxPlanCode"`
	Name                          *string              `json:"name"`
	BrandColor                    *string              `json:"brandColor"`
	LogoURL                       *string              `json:"logoUrl"`
	ContactEmail                  *string              `json:"contactEmail"`
	ContactPhone                  *string              `json:"contactPhone"`
	Active                        *bool                `json:"active"`
	ManagedBy1Hero                *bool                `json:"managedBy1Hero"`
	AvailabilitySlots             *[]AvailabilitySlot  `json:"availabilitySlots"`
	AvailabilityTimezone          *string              `json:"availabilityTimezone"`
	GeminiAPIKey                  *string              `json:"geminiApiKey"`
	GroqAPIKey                    *string              `json:"groqApiKey"`
	MetaAppID                     *string              `json:"metaAppId"`
	MetaAppSecret                 *string              `json:"metaAppSecret"`
	GoogleClientID                *string              `json:"googleClientId"`
	GoogleClientSecret            *string              `json:"googleClientSecret"`
	GoogleDeveloperToken          *string              `json:"googleDeveloperToken"`
	SocialPlannerEnabled          *bool                `json:"socialPlannerEnabled"`
	KnowledgeBase                 *string              `json:"knowledgeBase"`
	KnowledgeBaseFiles            *[]KnowledgeBaseFile `json:"knowledgeBaseFiles"`
	GreetingMessage               *string              `json:"greetingMessage"`
	TrialAmountSGD                *int                 `json:"trialAmountSgd"`
	TrialAmountINR                *int                 `json:"trialAmountInr"`
	TrialAmountUSD                *int                 `json:"trialAmountUsd"`
	BookingHeroImageURL           *string              `json:"bookingHeroImageUrl"`
	BookingHeroVideoURL           *string              `json:"bookingHeroVideoUrl"`
	TrialConfirmationMessage      *string              `json:"trialConfirmationMessage"`
	MembershipConfirmationMessage *string              `json:"membershipConfirmationMessage"`
	TrialGlofoxMembershipID       *string              `json:"trialGlofoxMembershipId"`
	TrialGlofoxPlanCode           *string              `json:"trialGlofoxPlanCode"`
	MembershipGlofoxMembershipID  *string              `json:"membershipGlofoxMembershipId"`
	MembershipGlofoxPlanCode      *string              `json:"membershipGlofoxPlanCode"`
}

// update godoc
//
//	@Summary		Update a studio
//	@Description	Partially updates a studio's settings, including integration secrets (Gemini, Groq, Meta, Google, Stripe). Secret fields are only overwritten when a non-empty value is supplied. Super-admin only.
//	@Tags			Studios
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string		true	"Studio ID"
//	@Param			body	body		updateReq	true	"Fields to update"
//	@Success		200		{object}	studioResponse
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid id or validation failed"
//	@Failure		404		{object}	httpx.ErrorResponse	"studio not found"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/studios/{id} [patch]
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req updateReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	existing, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	input := UpdateStudioInput{
		Name:                          existing.Name,
		BrandColor:                    existing.BrandColor,
		LogoURL:                       existing.LogoURL,
		ContactEmail:                  existing.ContactEmail,
		ContactPhone:                  existing.ContactPhone,
		Active:                        existing.Active,
		ManagedBy1Hero:                existing.ManagedBy1Hero,
		AvailabilitySlots:             existing.AvailabilitySlots,
		AvailabilityTimezone:          existing.AvailabilityTimezone,
		GeminiAPIKey:                  existing.GeminiAPIKey,
		GroqAPIKey:                    existing.GroqAPIKey,
		MetaAppID:                     existing.MetaAppID,
		MetaAppSecret:                 existing.MetaAppSecret,
		GoogleClientID:                existing.GoogleClientID,
		GoogleClientSecret:            existing.GoogleClientSecret,
		GoogleDeveloperToken:          existing.GoogleDeveloperToken,
		SocialPlannerEnabled:          existing.SocialPlannerEnabled,
		KnowledgeBase:                 existing.KnowledgeBase,
		KnowledgeBaseFiles:            existing.KnowledgeBaseFiles,
		GreetingMessage:               existing.GreetingMessage,
		TrialAmountSGD:                existing.TrialAmountSGD,
		BookingHeroImageURL:           existing.BookingHeroImageURL,
		BookingHeroVideoURL:           existing.BookingHeroVideoURL,
		Name:                          existing.Name,
		BrandColor:                    existing.BrandColor,
		LogoURL:                       existing.LogoURL,
		ContactEmail:                  existing.ContactEmail,
		ContactPhone:                  existing.ContactPhone,
		Active:                        existing.Active,
		ManagedBy1Hero:                existing.ManagedBy1Hero,
		AvailabilitySlots:             existing.AvailabilitySlots,
		AvailabilityTimezone:          existing.AvailabilityTimezone,
		GeminiAPIKey:                  existing.GeminiAPIKey,
		GroqAPIKey:                    existing.GroqAPIKey,
		MetaAppID:                     existing.MetaAppID,
		MetaAppSecret:                 existing.MetaAppSecret,
		GoogleClientID:                existing.GoogleClientID,
		GoogleClientSecret:            existing.GoogleClientSecret,
		GoogleDeveloperToken:          existing.GoogleDeveloperToken,
		SocialPlannerEnabled:          existing.SocialPlannerEnabled,
		KnowledgeBase:                 existing.KnowledgeBase,
		KnowledgeBaseFiles:            existing.KnowledgeBaseFiles,
		GreetingMessage:               existing.GreetingMessage,
		TrialAmountSGD:                existing.TrialAmountSGD,
		BookingHeroImageURL:           existing.BookingHeroImageURL,
		BookingHeroVideoURL:           existing.BookingHeroVideoURL,
		TrialConfirmationMessage:      existing.TrialConfirmationMessage,
		MembershipConfirmationMessage: existing.MembershipConfirmationMessage,
		TrialGlofoxMembershipID:       existing.TrialGlofoxMembershipID,
		TrialGlofoxPlanCode:           existing.TrialGlofoxPlanCode,
		MembershipGlofoxMembershipID:  existing.MembershipGlofoxMembershipID,
		MembershipGlofoxPlanCode:      existing.MembershipGlofoxPlanCode,
		TrialGlofoxMembershipID:       existing.TrialGlofoxMembershipID,
		TrialGlofoxPlanCode:           existing.TrialGlofoxPlanCode,
		MembershipGlofoxMembershipID:  existing.MembershipGlofoxMembershipID,
		MembershipGlofoxPlanCode:      existing.MembershipGlofoxPlanCode,
	}
	if req.Name != nil {
		input.Name = *req.Name
	}
	if req.BrandColor != nil {
		input.BrandColor = *req.BrandColor
	}
	if req.LogoURL != nil {
		input.LogoURL = *req.LogoURL
	}
	if req.ContactEmail != nil {
		input.ContactEmail = *req.ContactEmail
	}
	if req.ContactPhone != nil {
		input.ContactPhone = *req.ContactPhone
	}
	if req.Active != nil {
		input.Active = *req.Active
	}
	if req.ManagedBy1Hero != nil {
		input.ManagedBy1Hero = *req.ManagedBy1Hero
	}
	if req.AvailabilitySlots != nil {
		input.AvailabilitySlots = *req.AvailabilitySlots
	}
	if req.AvailabilityTimezone != nil {
		input.AvailabilityTimezone = *req.AvailabilityTimezone
	}
	// Only overwrite secrets when a non-empty value is provided; empty means "keep existing".
	if req.GeminiAPIKey != nil && *req.GeminiAPIKey != "" {
		input.GeminiAPIKey = *req.GeminiAPIKey
	}
	if req.GroqAPIKey != nil && *req.GroqAPIKey != "" {
		input.GroqAPIKey = *req.GroqAPIKey
	}
	if req.MetaAppID != nil {
		input.MetaAppID = *req.MetaAppID
	}
	if req.MetaAppSecret != nil && *req.MetaAppSecret != "" {
		input.MetaAppSecret = *req.MetaAppSecret
	}
	if req.GoogleClientID != nil {
		input.GoogleClientID = *req.GoogleClientID
	}
	if req.GoogleClientSecret != nil && *req.GoogleClientSecret != "" {
		input.GoogleClientSecret = *req.GoogleClientSecret
	}
	if req.GoogleDeveloperToken != nil && *req.GoogleDeveloperToken != "" {
		input.GoogleDeveloperToken = *req.GoogleDeveloperToken
	}
	if req.SocialPlannerEnabled != nil {
		input.SocialPlannerEnabled = *req.SocialPlannerEnabled
	}
	if req.KnowledgeBase != nil {
		input.KnowledgeBase = *req.KnowledgeBase
	}
	if req.KnowledgeBaseFiles != nil {
		input.KnowledgeBaseFiles = *req.KnowledgeBaseFiles
	}
	if req.GreetingMessage != nil {
		input.GreetingMessage = *req.GreetingMessage
	}
	if req.TrialAmountSGD != nil {
		input.TrialAmountSGD = *req.TrialAmountSGD
	}
	if req.BookingHeroImageURL != nil {
		input.BookingHeroImageURL = *req.BookingHeroImageURL
	}
	if req.BookingHeroVideoURL != nil {
		input.BookingHeroVideoURL = *req.BookingHeroVideoURL
	}
	if req.TrialConfirmationMessage != nil {
		input.TrialConfirmationMessage = *req.TrialConfirmationMessage
	}
	if req.MembershipConfirmationMessage != nil {
		input.MembershipConfirmationMessage = *req.MembershipConfirmationMessage
	}
	if req.TrialGlofoxMembershipID != nil {
		input.TrialGlofoxMembershipID = *req.TrialGlofoxMembershipID
	}
	if req.TrialGlofoxPlanCode != nil {
		input.TrialGlofoxPlanCode = *req.TrialGlofoxPlanCode
	}
	if req.MembershipGlofoxMembershipID != nil {
		input.MembershipGlofoxMembershipID = *req.MembershipGlofoxMembershipID
	}
	if req.MembershipGlofoxPlanCode != nil {
		input.MembershipGlofoxPlanCode = *req.MembershipGlofoxPlanCode
	}

	errs, err := h.svc.Update(r.Context(), id, input)
	if errs != nil {
		httpx.WriteValidationError(w, errs)
		return
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	updated, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, toStudioResponse(updated))
}

// ----- studio-admin scoped handlers -----
//
// A studio_admin can read/update only their own studio. Super_admins can use
// the AdminRoutes endpoints above for any studio. We fail closed: if the path
// id doesn't match the caller's claim, return 403.

// getScoped godoc
//
//	@Summary		Get own studio
//	@Description	Returns the caller's own studio details. Super-admins may pass any studio ID; studio-admins are always scoped to their own studio regardless of the path value.
//	@Tags			Studios
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id	path		string	true	"Studio ID"
//	@Success		200	{object}	studioResponse
//	@Failure		400	{object}	httpx.ErrorResponse	"invalid id"
//	@Failure		403	{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		404	{object}	httpx.ErrorResponse	"studio not found"
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/api/v1/me/studios/{id} [get]
func (h *Handler) getScoped(w http.ResponseWriter, r *http.Request) {
	c := identity.MustClaims(r.Context())
	// Super admins use the URL param; studio_admins always get their own studio.
	var studioID uuid.UUID
	if c.IsSuper() {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
			return
		}
		studioID = id
	} else {
		if c.StudioID == nil {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "no studio bound to this user")
			return
		}
		studioID = *c.StudioID
	}
	s, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, toStudioResponse(s))
}

// updateScoped godoc
//
//	@Summary		Update own studio
//	@Description	Partially updates the caller's own studio settings, including integration secrets. Secret fields are only overwritten when a non-empty value is supplied. Super-admins may target any studio ID; studio-admins are always scoped to their own studio.
//	@Tags			Studios
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string		true	"Studio ID"
//	@Param			body	body		updateReq	true	"Fields to update"
//	@Success		200		{object}	studioResponse
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid id or validation failed"
//	@Failure		403		{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		404		{object}	httpx.ErrorResponse	"studio not found"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/me/studios/{id} [patch]
func (h *Handler) updateScoped(w http.ResponseWriter, r *http.Request) {
	c := identity.MustClaims(r.Context())
	// Super admins use the URL param; studio_admins always update their own studio.
	var studioID uuid.UUID
	if c.IsSuper() {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
			return
		}
		studioID = id
	} else {
		if c.StudioID == nil {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "no studio bound to this user")
			return
		}
		studioID = *c.StudioID
	}
	var req updateReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	existing, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	input := UpdateStudioInput{
		Name:                          existing.Name,
		BrandColor:                    existing.BrandColor,
		LogoURL:                       existing.LogoURL,
		ContactEmail:                  existing.ContactEmail,
		ContactPhone:                  existing.ContactPhone,
		Active:                        existing.Active,
		ManagedBy1Hero:                existing.ManagedBy1Hero,
		AvailabilitySlots:             existing.AvailabilitySlots,
		AvailabilityTimezone:          existing.AvailabilityTimezone,
		GeminiAPIKey:                  existing.GeminiAPIKey,
		GroqAPIKey:                    existing.GroqAPIKey,
		MetaAppID:                     existing.MetaAppID,
		MetaAppSecret:                 existing.MetaAppSecret,
		GoogleClientID:                existing.GoogleClientID,
		GoogleClientSecret:            existing.GoogleClientSecret,
		GoogleDeveloperToken:          existing.GoogleDeveloperToken,
		SocialPlannerEnabled:          existing.SocialPlannerEnabled,
		KnowledgeBase:                 existing.KnowledgeBase,
		KnowledgeBaseFiles:            existing.KnowledgeBaseFiles,
		GreetingMessage:               existing.GreetingMessage,
		TrialAmountSGD:                existing.TrialAmountSGD,
		BookingHeroImageURL:           existing.BookingHeroImageURL,
		BookingHeroVideoURL:           existing.BookingHeroVideoURL,
		Name:                          existing.Name,
		BrandColor:                    existing.BrandColor,
		LogoURL:                       existing.LogoURL,
		ContactEmail:                  existing.ContactEmail,
		ContactPhone:                  existing.ContactPhone,
		Active:                        existing.Active,
		ManagedBy1Hero:                existing.ManagedBy1Hero,
		AvailabilitySlots:             existing.AvailabilitySlots,
		AvailabilityTimezone:          existing.AvailabilityTimezone,
		GeminiAPIKey:                  existing.GeminiAPIKey,
		GroqAPIKey:                    existing.GroqAPIKey,
		MetaAppID:                     existing.MetaAppID,
		MetaAppSecret:                 existing.MetaAppSecret,
		GoogleClientID:                existing.GoogleClientID,
		GoogleClientSecret:            existing.GoogleClientSecret,
		GoogleDeveloperToken:          existing.GoogleDeveloperToken,
		SocialPlannerEnabled:          existing.SocialPlannerEnabled,
		KnowledgeBase:                 existing.KnowledgeBase,
		KnowledgeBaseFiles:            existing.KnowledgeBaseFiles,
		GreetingMessage:               existing.GreetingMessage,
		TrialAmountSGD:                existing.TrialAmountSGD,
		BookingHeroImageURL:           existing.BookingHeroImageURL,
		BookingHeroVideoURL:           existing.BookingHeroVideoURL,
		TrialConfirmationMessage:      existing.TrialConfirmationMessage,
		MembershipConfirmationMessage: existing.MembershipConfirmationMessage,
		TrialGlofoxMembershipID:       existing.TrialGlofoxMembershipID,
		TrialGlofoxPlanCode:           existing.TrialGlofoxPlanCode,
		MembershipGlofoxMembershipID:  existing.MembershipGlofoxMembershipID,
		MembershipGlofoxPlanCode:      existing.MembershipGlofoxPlanCode,
		TrialGlofoxMembershipID:       existing.TrialGlofoxMembershipID,
		TrialGlofoxPlanCode:           existing.TrialGlofoxPlanCode,
		MembershipGlofoxMembershipID:  existing.MembershipGlofoxMembershipID,
		MembershipGlofoxPlanCode:      existing.MembershipGlofoxPlanCode,
	}
	if req.Name != nil {
		input.Name = *req.Name
	}
	if req.BrandColor != nil {
		input.BrandColor = *req.BrandColor
	}
	if req.LogoURL != nil {
		input.LogoURL = *req.LogoURL
	}
	if req.ContactEmail != nil {
		input.ContactEmail = *req.ContactEmail
	}
	if req.ContactPhone != nil {
		input.ContactPhone = *req.ContactPhone
	}
	if req.Active != nil {
		input.Active = *req.Active
	}
	if req.ManagedBy1Hero != nil {
		input.ManagedBy1Hero = *req.ManagedBy1Hero
	}
	if req.AvailabilitySlots != nil {
		input.AvailabilitySlots = *req.AvailabilitySlots
	}
	if req.AvailabilityTimezone != nil {
		input.AvailabilityTimezone = *req.AvailabilityTimezone
	}
	// Only overwrite secrets when a non-empty value is provided; empty means "keep existing".
	if req.GeminiAPIKey != nil && *req.GeminiAPIKey != "" {
		input.GeminiAPIKey = *req.GeminiAPIKey
	}
	if req.GroqAPIKey != nil && *req.GroqAPIKey != "" {
		input.GroqAPIKey = *req.GroqAPIKey
	}
	if req.MetaAppID != nil {
		input.MetaAppID = *req.MetaAppID
	}
	if req.MetaAppSecret != nil && *req.MetaAppSecret != "" {
		input.MetaAppSecret = *req.MetaAppSecret
	}
	if req.GoogleClientID != nil {
		input.GoogleClientID = *req.GoogleClientID
	}
	if req.GoogleClientSecret != nil && *req.GoogleClientSecret != "" {
		input.GoogleClientSecret = *req.GoogleClientSecret
	}
	if req.GoogleDeveloperToken != nil && *req.GoogleDeveloperToken != "" {
		input.GoogleDeveloperToken = *req.GoogleDeveloperToken
	}
	if req.SocialPlannerEnabled != nil {
		input.SocialPlannerEnabled = *req.SocialPlannerEnabled
	}
	if req.KnowledgeBase != nil {
		input.KnowledgeBase = *req.KnowledgeBase
	}
	if req.KnowledgeBaseFiles != nil {
		input.KnowledgeBaseFiles = *req.KnowledgeBaseFiles
	}
	if req.GreetingMessage != nil {
		input.GreetingMessage = *req.GreetingMessage
	}
	if req.TrialAmountSGD != nil {
		input.TrialAmountSGD = *req.TrialAmountSGD
	}

	if req.BookingHeroImageURL != nil {
		input.BookingHeroImageURL = *req.BookingHeroImageURL
	}
	if req.BookingHeroVideoURL != nil {
		input.BookingHeroVideoURL = *req.BookingHeroVideoURL
	}
	if req.TrialConfirmationMessage != nil {
		input.TrialConfirmationMessage = *req.TrialConfirmationMessage
	}
	if req.MembershipConfirmationMessage != nil {
		input.MembershipConfirmationMessage = *req.MembershipConfirmationMessage
	}
	if req.TrialGlofoxMembershipID != nil {
		input.TrialGlofoxMembershipID = *req.TrialGlofoxMembershipID
	}
	if req.TrialGlofoxPlanCode != nil {
		input.TrialGlofoxPlanCode = *req.TrialGlofoxPlanCode
	}
	if req.MembershipGlofoxMembershipID != nil {
		input.MembershipGlofoxMembershipID = *req.MembershipGlofoxMembershipID
	}
	if req.MembershipGlofoxPlanCode != nil {
		input.MembershipGlofoxPlanCode = *req.MembershipGlofoxPlanCode
	}
	errs, err := h.svc.Update(r.Context(), studioID, input)
	if errs != nil {
		httpx.WriteValidationError(w, errs)
		return
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		slog.Error("updateScoped failed", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	updated, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, toStudioResponse(updated))
}

// uploadLogo godoc
//
//	@Summary		Upload studio logo
//	@Description	Uploads a studio logo image (max 5MB, JPEG/PNG/WebP/GIF). Stores the image in S3 when configured, otherwise falls back to local disk storage, and returns the resulting logo URL.
//	@Tags			Studios
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string	true	"Studio ID"
//	@Param			file	formData	file	true	"Logo image file (JPEG, PNG, WebP, or GIF)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid id, missing file, or unsupported file type"
//	@Failure		403		{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/me/studios/{id}/logo [post]
func (h *Handler) uploadLogo(w http.ResponseWriter, r *http.Request) {
	c := identity.MustClaims(r.Context())
	// Super admins use the URL param; studio_admins always update their own studio.
	var studioID uuid.UUID
	if c.IsSuper() {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
			return
		}
		studioID = id
	} else {
		if c.StudioID == nil {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "no studio bound to this user")
			return
		}
		studioID = *c.StudioID
	}

	// 5MB max for logo image
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "file field is required")
		return
	}
	defer file.Close()

	// Validate file type (only images)
	contentType := header.Header.Get("Content-Type")
	validTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}
	if !validTypes[contentType] {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_type", "only image files (JPEG, PNG, WebP, GIF) are allowed")
		return
	}

	// Create a unique filename using studio ID
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		// Infer extension from content type
		switch contentType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		case "image/gif":
			ext = ".gif"
		}
	}

	// Read file data
	bytes, err := io.ReadAll(file)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to read file")
		return
	}

	// Upload to S3 if configured, otherwise fall back to disk
	var logoURL string
	if h.s3Uploader != nil {
		key := fmt.Sprintf("logos/%s%s", studioID.String(), ext)
		url, err := h.s3Uploader.UploadImage(r.Context(), key, bytes, contentType)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "s3_upload", "failed to upload logo")
			return
		}
		logoURL = url
	} else {
		// Fallback to disk storage if S3 not configured
		filename := fmt.Sprintf("logo_%s%s", studioID.String(), ext)
		filepath := filepath.Join("./uploads", filename)
		if err := os.MkdirAll("./uploads", 0755); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to create uploads directory")
			return
		}
		if err := os.WriteFile(filepath, bytes, 0644); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to save file")
			return
		}
		logoURL = fmt.Sprintf("/uploads/%s", filename)
	}

	httpx.JSON(w, http.StatusOK, map[string]string{
		"logoUrl": logoURL,
	})
}

// UploadSocialPostImage godoc
//
//	@Summary		Upload a social post image
//	@Description	Uploads an image (max 10MB, JPEG/PNG/WebP/GIF) for use in the studio's social planner. Stores the image in S3 when configured, otherwise falls back to local disk storage, and returns the resulting media URL. Studio-admins may only upload for their own studio; super-admins are forbidden from this endpoint.
//	@Tags			Social Planner
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Param			file		formData	file	true	"Social post image file (JPEG, PNG, WebP, or GIF)"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studio id, missing file, or unsupported file type"
//	@Failure		403			{object}	httpx.ErrorResponse	"super admins cannot upload social post images, or cannot access this studio"
//	@Failure		500			{object}	httpx.ErrorResponse	"upload failed"
//	@Router			/api/v1/studios/{studioId}/social-posts/upload-image [post]
func (h *Handler) UploadSocialPostImage(w http.ResponseWriter, r *http.Request) {
	c := identity.MustClaims(r.Context())
	if c.IsSuper() {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "super admins cannot upload social post images")
		return
	}

	studioIDStr := chi.URLParam(r, "studioId")
	studioID, err := uuid.Parse(studioIDStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}

	// Verify the user owns the studio
	if c.StudioID == nil || c.StudioID.String() != studioID.String() {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "cannot access this studio")
		return
	}

	// 10MB max for social media images
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "file field is required")
		return
	}
	defer file.Close()

	// Validate image type
	contentType := header.Header.Get("Content-Type")
	validTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}
	if !validTypes[contentType] {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_type", "only JPEG, PNG, WebP, GIF allowed")
		return
	}

	// Read file data
	bytes, err := io.ReadAll(file)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to read file")
		return
	}

	// Get file extension
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = map[string]string{
			"image/jpeg": ".jpg",
			"image/png":  ".png",
			"image/webp": ".webp",
			"image/gif":  ".gif",
		}[contentType]
	}

	var mediaURL string

	// Try S3 first if configured, otherwise fall back to disk
	if h.s3Uploader != nil {
		key := fmt.Sprintf(
			"social-posts/%s/%d_%s%s",
			studioID.String(),
			time.Now().Unix(),
			uuid.New().String()[:8],
			ext,
		)
		url, err := h.s3Uploader.UploadImage(r.Context(), key, bytes, contentType)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "s3_upload", fmt.Sprintf("S3 upload failed: %v", err))
			return
		}
		mediaURL = url
	} else {
		// Fallback to disk storage if S3 not configured
		filename := fmt.Sprintf("social_%s_%d_%s%s", studioID.String(), time.Now().Unix(), uuid.New().String()[:8], ext)
		filepath := filepath.Join("./uploads", filename)
		if err := os.MkdirAll("./uploads", 0755); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("failed to create uploads directory: %v", err))
			return
		}
		if err := os.WriteFile(filepath, bytes, 0644); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("failed to save file: %v", err))
			return
		}
		mediaURL = fmt.Sprintf("/uploads/%s", filename)
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"mediaUrl": mediaURL})
}

// ----- public -----

type publicRes struct {
	Slug                 string `json:"slug"`
	Name                 string `json:"name"`
	BrandColor           string `json:"brandColor"`
	LogoURL              string `json:"logoUrl"`
	TrialAmountSGD       int    `json:"trialAmountSgd"`
	AvailabilitySlots    any    `json:"availabilitySlots,omitempty"`
	AvailabilityTimezone string `json:"availabilityTimezone,omitempty"`
	StripePublishableKey string `json:"stripePublishableKey,omitempty"`
	BookingHeroImageURL  string `json:"bookingHeroImageUrl,omitempty"`
	BookingHeroVideoURL  string `json:"bookingHeroVideoUrl,omitempty"`
}

type publicPlanRes struct {
	ID           string   `json:"id"`
	PlanName     string   `json:"planName"`
	PriceSGD     int      `json:"priceSgd"`
	BillingCycle string   `json:"billingCycle"`
	Features     []string `json:"features"`
	IsActive     bool     `json:"isActive"`
}

// publicGet godoc
//
//	@Summary		Get public studio brand info
//	@Description	Public endpoint returning a studio's public branding info (name, logo, colors, availability, hero media) by slug, for rendering the public booking form. No auth required.
//	@Tags			Studios (Public)
//	@Produce		json
//	@Param			slug	path		string	true	"Studio slug"
//	@Success		200		{object}	publicRes
//	@Failure		404		{object}	httpx.ErrorResponse	"studio not found"
//	@Router			/api/v1/public/studios/{slug} [get]
func (h *Handler) publicGet(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	s, err := h.svc.GetBySlug(r.Context(), slug)
	if err != nil || !s.Active {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}
	httpx.JSON(w, http.StatusOK, publicRes{
		Slug:                 s.Slug,
		Name:                 s.Name,
		BrandColor:           s.BrandColor,
		LogoURL:              s.LogoURL,
		TrialAmountSGD:       int(h.svc.ResolveTrialAmountSGD(r.Context(), s.ID, s.TrialAmountSGD)),
		AvailabilitySlots:    s.AvailabilitySlots,
		AvailabilityTimezone: s.AvailabilityTimezone,
		StripePublishableKey: s.StripePublishableKey,
		BookingHeroImageURL:  s.BookingHeroImageURL,
		BookingHeroVideoURL:  s.BookingHeroVideoURL,
	})
}

// publicGetPlans godoc
//
//	@Summary		List a studio's public membership plans
//	@Description	Public endpoint returning the studio's active, paid membership plans for display on the public booking page. Free/trial plans and inactive plans are excluded. No auth required.
//	@Tags			Studios (Public)
//	@Produce		json
//	@Param			slug	path		string	true	"Studio slug"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		404		{object}	httpx.ErrorResponse	"studio not found"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/public/studios/{slug}/plans [get]
func (h *Handler) publicGetPlans(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	s, err := h.svc.GetBySlug(r.Context(), slug)
	if err != nil || !s.Active {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}
	plans, err := h.svc.ListPlans(r.Context(), s.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not load plans")
		return
	}
	out := make([]publicPlanRes, 0, len(plans))
	for _, p := range plans {
		// Skip inactive and free/trial plans — booking page shows membership plans only
		if !p.IsActive || p.PriceSGD == 0 {
			continue
		}
		out = append(out, publicPlanRes{
			ID:           p.ID.String(),
			PlanName:     p.PlanName,
			PriceSGD:     p.PriceSGD,
			BillingCycle: p.BillingCycle,
			Features:     p.Features,
			IsActive:     p.IsActive,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"plans": out})
}

// publicCreateCheckout godoc
//
//	@Summary		Create a public Stripe Checkout session for a membership plan
//	@Description	Public endpoint that creates a Stripe Checkout Session (hosted payment page) for a lead to pay for a selected membership plan. Requires the studio to have a connected Stripe secret key and the plan to be active and non-free. No auth required.
//	@Tags			Studios (Public)
//	@Accept			json
//	@Produce		json
//	@Param			slug	path		string					true	"Studio slug"
//	@Param			body	body		map[string]interface{}	true	"Checkout details (planId, leadId, leadName)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid JSON, Stripe not configured, plan not found, or free plan"
//	@Failure		404		{object}	httpx.ErrorResponse	"studio not found"
//	@Failure		500		{object}	httpx.ErrorResponse	"Stripe error"
//	@Router			/api/v1/public/studios/{slug}/checkout [post]
func (h *Handler) publicCreateCheckout(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	s, err := h.svc.GetBySlug(r.Context(), slug)
	if err != nil || !s.Active {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}

	var req struct {
		PlanID   string `json:"planId"`
		LeadID   string `json:"leadId"`
		LeadName string `json:"leadName"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	if s.StripeSecretKey == "" {
		httpx.WriteError(w, http.StatusBadRequest, "stripe_not_configured", "Stripe not connected for this studio")
		return
	}

	// Load the selected plan
	plans, err := h.svc.ListPlans(r.Context(), s.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not load plans")
		return
	}
	var selected *Plan
	for i := range plans {
		if plans[i].ID.String() == req.PlanID && plans[i].IsActive {
			selected = &plans[i]
			break
		}
	}
	if selected == nil {
		httpx.WriteError(w, http.StatusBadRequest, "plan_not_found", "plan not found or inactive")
		return
	}
	if selected.PriceSGD == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "free_plan", "plan is free, no checkout needed")
		return
	}

	sc := &client.API{}
	sc.Init(s.StripeSecretKey, nil)

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	// Build success URL that returns lead back to booking calendar
	successURL := fmt.Sprintf(
		"%s/l/%s/%s/book?leadId=%s&paid=1",
		frontendURL, s.Slug, slug, req.LeadID,
	)
	// Find the campaign slug for the cancel URL via plans is not needed — use a generic return
	cancelURL := fmt.Sprintf("%s/payment-cancelled?studio=%s", frontendURL, s.Slug)

	mode := "payment"
	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String("sgd"),
					UnitAmount: stripe.Int64(int64(selected.PriceSGD)),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String(fmt.Sprintf("%s — %s", s.Name, selected.PlanName)),
						Description: stripe.String(fmt.Sprintf("Billing: %s", selected.BillingCycle)),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(mode),
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		Metadata: map[string]string{
			"studio_id":   s.ID.String(),
			"lead_id":     req.LeadID,
			"plan_id":     req.PlanID,
			"lead_name":   req.LeadName,
			"plan_name":   selected.PlanName,
			"studio_slug": s.Slug,
		},
	}

	session, err := sc.CheckoutSessions.New(params)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "stripe_error", fmt.Sprintf("failed to create checkout: %v", err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"url": session.URL})
}

// publicCreatePaymentIntent godoc
//
//	@Summary		Create a public Stripe PaymentIntent for a membership plan
//	@Description	Public endpoint that creates a Stripe PaymentIntent for embedded-Elements checkout of a selected, active, non-free membership plan. Attaches campaign metadata resolved from the lead when available. Requires the studio to have a connected Stripe secret key. No auth required.
//	@Tags			Studios (Public)
//	@Accept			json
//	@Produce		json
//	@Param			slug	path		string					true	"Studio slug"
//	@Param			body	body		map[string]interface{}	true	"Payment details (planId, leadId)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid JSON, Stripe not configured, or plan not found/inactive/free"
//	@Failure		404		{object}	httpx.ErrorResponse	"studio not found"
//	@Failure		500		{object}	httpx.ErrorResponse	"Stripe error"
//	@Router			/api/v1/public/studios/{slug}/payment-intent [post]
func (h *Handler) publicCreatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	s, err := h.svc.GetBySlug(r.Context(), slug)
	if err != nil || !s.Active {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}

	var req struct {
		PlanID string `json:"planId"`
		LeadID string `json:"leadId"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	if s.StripeSecretKey == "" {
		httpx.WriteError(w, http.StatusBadRequest, "stripe_not_configured", "Stripe not connected for this studio")
		return
	}

	plans, err := h.svc.ListPlans(r.Context(), s.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not load plans")
		return
	}
	var selected *Plan
	for i := range plans {
		if plans[i].ID.String() == req.PlanID && plans[i].IsActive {
			selected = &plans[i]
			break
		}
	}
	if selected == nil || selected.PriceSGD == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "plan_not_found", "plan not found, inactive, or free")
		return
	}

	sc := &client.API{}
	sc.Init(s.StripeSecretKey, nil)

	// Look up campaign name+slug from the lead so we can store it in PI metadata.
	var campaignName, campaignSlug string
	_ = h.svc.repo.Pool().QueryRow(r.Context(),
		`SELECT c.name, c.slug FROM leads l JOIN campaigns c ON c.id = l.campaign_id WHERE l.id = $1`,
		req.LeadID,
	).Scan(&campaignName, &campaignSlug)

	params := &stripe.PaymentIntentParams{
		Amount:      stripe.Int64(int64(selected.PriceSGD)),
		Currency:    stripe.String("sgd"),
		Description: stripe.String(selected.PlanName),
		Metadata: map[string]string{
			"studio_id":     s.ID.String(),
			"lead_id":       req.LeadID,
			"plan_id":       req.PlanID,
			"plan_name":     selected.PlanName,
			"studio_slug":   s.Slug,
			"campaign_name": campaignName,
			"campaign_slug": campaignSlug,
		},
	}
	pi, err := sc.PaymentIntents.New(params)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "stripe_error", fmt.Sprintf("failed to create payment intent: %v", err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"clientSecret": pi.ClientSecret,
		"amount":       selected.PriceSGD,
		"planName":     selected.PlanName,
		"billingCycle": selected.BillingCycle,
	})
}

// publicGetPaymentReceipt godoc
//
//	@Summary		Get a public Stripe payment receipt URL
//	@Description	Public endpoint that looks up a Stripe PaymentIntent by ID on the studio's connected Stripe account and returns the hosted receipt URL from its latest charge, if any. No auth required.
//	@Tags			Studios (Public)
//	@Produce		json
//	@Param			slug	path		string	true	"Studio slug"
//	@Param			piId	path		string	true	"Stripe PaymentIntent ID"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		404		{object}	httpx.ErrorResponse	"studio not found or payment not found"
//	@Router			/api/v1/public/studios/{slug}/payment-receipt/{piId} [get]
func (h *Handler) publicGetPaymentReceipt(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	piID := chi.URLParam(r, "piId")

	s, err := h.svc.GetBySlug(r.Context(), slug)
	if err != nil || !s.Active || s.StripeSecretKey == "" {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}

	sc := &client.API{}
	sc.Init(s.StripeSecretKey, nil)

	params := &stripe.PaymentIntentParams{}
	params.AddExpand("latest_charge")
	pi, err := sc.PaymentIntents.Get(piID, params)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "payment not found")
		return
	}

	receiptURL := ""
	if pi.LatestCharge != nil {
		receiptURL = pi.LatestCharge.ReceiptURL
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"receiptUrl": receiptURL,
	})
}

// getGoogleCredentials godoc
//
//	@Summary		Get platform Google service-account credential status
//	@Description	Returns whether platform-level Google Sheets service-account credentials are configured, along with the associated client email and project ID (never the private key). Super-admin only.
//	@Tags			Studios
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	map[string]interface{}
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/google-credentials [get]
func (h *Handler) getGoogleCredentials(w http.ResponseWriter, r *http.Request) {
	if h.credentialsPath == "" {
		httpx.JSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}

	data, err := os.ReadFile(h.credentialsPath)
	if err != nil {
		if os.IsNotExist(err) {
			httpx.JSON(w, http.StatusOK, map[string]any{"configured": false})
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	var creds struct {
		Type        string `json:"type"`
		ProjectID   string `json:"project_id"`
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		httpx.JSON(w, http.StatusOK, map[string]any{"configured": false, "error": "invalid json format"})
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"configured":  creds.Type == "service_account" && creds.ClientEmail != "",
		"clientEmail": creds.ClientEmail,
		"projectId":   creds.ProjectID,
	})
}

// uploadGoogleCredentials godoc
//
//	@Summary		Upload platform Google service-account credentials
//	@Description	Uploads a Google service-account JSON key file (max 1MB) used platform-wide for Google Sheets integration. Validates the file is a service_account credential before saving it to disk. Super-admin only.
//	@Tags			Studios
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		CookieAuth
//	@Param			file	formData	file	true	"Google service-account JSON key file"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"missing file, invalid JSON, or not a service account key"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/google-credentials [post]
func (h *Handler) uploadGoogleCredentials(w http.ResponseWriter, r *http.Request) {
	if h.credentialsPath == "" {
		httpx.WriteError(w, http.StatusBadRequest, "disabled", "Google Sheets credentials path not configured in env")
		return
	}

	// 1MB max for service account JSON
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to parse multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "file field is required")
		return
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to read file")
		return
	}

	var creds struct {
		Type        string `json:"type"`
		ProjectID   string `json:"project_id"`
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(bytes, &creds); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "file is not valid JSON")
		return
	}

	if creds.Type != "service_account" || creds.ClientEmail == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_credentials", "file is not a valid Google service account JSON key file")
		return
	}

	// Ensure the parent directory of h.credentialsPath exists
	dir := filepath.Dir(h.credentialsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to create secrets directory")
		return
	}

	// Write file
	if err := os.WriteFile(h.credentialsPath, bytes, 0600); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("failed to save file: %v", err))
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"configured":  true,
		"clientEmail": creds.ClientEmail,
		"projectId":   creds.ProjectID,
	})
}

// getPayments godoc
//
//	@Summary		Get Stripe payment configuration status
//	@Description	Returns the studio's connected Stripe account ID, publishable key, subscription tier, trial amount, and whether secret/webhook keys are configured (never the secret values themselves). Pass "global" as the id to read the platform-level Stripe configuration instead of a specific studio.
//	@Tags			Billing
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id	path		string	true	"Studio ID, or \"global\" for platform-level settings"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	httpx.ErrorResponse	"invalid studio id"
//	@Failure		404	{object}	httpx.ErrorResponse	"studio not found"
//	@Router			/api/v1/me/studios/{id}/payments [get]
func (h *Handler) getPayments(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	// If id is 'global', return platform settings
	if idStr == "global" {
		accountId, _ := h.svc.GetPlatformSetting(r.Context(), "stripe_account_id")
		publishableKey, _ := h.svc.GetPlatformSetting(r.Context(), "stripe_publishable_key")
		secretKey, _ := h.svc.GetPlatformSetting(r.Context(), "stripe_secret_key")
		webhookSecret, _ := h.svc.GetPlatformSetting(r.Context(), "stripe_webhook_secret")

		httpx.JSON(w, http.StatusOK, map[string]any{
			"stripeAccountId":        accountId,
			"stripePublishableKey":   publishableKey,
			"hasStripeSecretKey":     secretKey != "",
			"hasStripeWebhookSecret": webhookSecret != "",
			"subscriptionTier":       "platform",
			"trialAmountSgd":         0,
		})
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid studio ID")
		return
	}

	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}

	hasSecretKey := s.StripeSecretKey != ""
	hasWebhookSecret := s.StripeWebhookSecret != ""

	httpx.JSON(w, http.StatusOK, map[string]any{
		"stripeAccountId":        s.StripeAccountID,
		"stripePublishableKey":   s.StripePublishableKey,
		"hasStripeSecretKey":     hasSecretKey,
		"hasStripeWebhookSecret": hasWebhookSecret,
		"subscriptionTier":       s.SubscriptionTier,
		"trialAmountSgd":         s.TrialAmountSGD,
	})
}

// createTrialCheckout godoc
//
//	@Summary		Create a Stripe Checkout session for a trial session
//	@Description	Creates a Stripe Checkout Session (hosted payment page) for a customer to pay for a trial session at the studio, using the studio's configured trial amount (or a 25.00 SGD default). Requires the studio to have a connected Stripe secret key.
//	@Tags			Billing
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string					true	"Studio ID"
//	@Param			body	body		map[string]interface{}	true	"Customer details (customerPhone in E.164 format, customerName)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid studio id, invalid JSON, or Stripe not configured"
//	@Failure		404		{object}	httpx.ErrorResponse	"studio not found"
//	@Failure		500		{object}	httpx.ErrorResponse	"Stripe error"
//	@Router			/api/v1/me/studios/{id}/trial-checkout [post]
func (h *Handler) createTrialCheckout(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "global" {
		httpx.WriteError(w, http.StatusBadRequest, "forbidden", "cannot create checkout on global scope")
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid studio ID")
		return
	}

	var req struct {
		CustomerPhone string `json:"customerPhone"` // E.164 format e.g. 6591234567
		CustomerName  string `json:"customerName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request body")
		return
	}

	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}

	if s.StripeSecretKey == "" {
		httpx.WriteError(w, http.StatusBadRequest, "stripe_not_configured", "Stripe is not connected for this studio")
		return
	}

	// Determine amount and currency
	amount := h.svc.ResolveTrialAmountSGD(r.Context(), id, s.TrialAmountSGD)
	cur := "sgd"

	sc := &client.API{}
	sc.Init(s.StripeSecretKey, nil)

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	// Use Checkout Session with inline price_data (no pre-created product needed)
	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(cur),
					UnitAmount: stripe.Int64(amount),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String(fmt.Sprintf("%s Trial Session", s.Name)),
						Description: stripe.String("Secure your trial workout session at " + s.Name),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String("payment"),
		SuccessURL: stripe.String(fmt.Sprintf("%s/payment-success?studio=%s&session_id={CHECKOUT_SESSION_ID}", frontendURL, s.Slug)),
		CancelURL:  stripe.String(fmt.Sprintf("%s/payment-cancelled?studio=%s", frontendURL, s.Slug)),
	}
	if req.CustomerPhone != "" {
		params.Metadata = map[string]string{
			"customer_phone": req.CustomerPhone,
			"customer_name":  req.CustomerName,
			"studio_id":      id.String(),
		}
	}

	session, err := sc.CheckoutSessions.New(params)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "stripe_error", fmt.Sprintf("Failed to create checkout session: %v", err))
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"url":      session.URL,
		"amount":   amount,
		"currency": cur,
	})
}

// ----- trial payment page builder -----

// trialPageBlockCheck is deliberately minimal — just enough to validate the
// block-type invariants below. The full block JSON (position, size, content,
// styling) round-trips through storage as raw bytes; this handler doesn't
// need to understand any of it beyond "type".
type trialPageBlockCheck struct {
	Type string `json:"type"`
}

// validateTrialPageBlocks enforces the one invariant that keeps a customized
// page actually usable: exactly one card_fields block (the Stripe card
// inputs) and exactly one pay_button block. Everything else is free-form.
func validateTrialPageBlocks(raw []byte) error {
	var blocks []trialPageBlockCheck
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return fmt.Errorf("invalid layout: %w", err)
	}
	cardFields, payButtons := 0, 0
	for _, b := range blocks {
		switch b.Type {
		case "card_fields":
			cardFields++
		case "pay_button":
			payButtons++
		}
	}
	if cardFields != 1 {
		return fmt.Errorf("layout must have exactly one card_fields block (found %d)", cardFields)
	}
	if payButtons != 1 {
		return fmt.Errorf("layout must have exactly one pay_button block (found %d)", payButtons)
	}
	return nil
}

// emptyTrialPageLayout is returned when a studio has never saved a custom
// layout — the frontend falls back to its own built-in default in that case.
var emptyTrialPageLayout = map[string]any{"blocks": nil, "background": nil}

// getTrialPageLayout godoc
//
//	@Summary		Get the studio's custom trial payment page layout
//	@Description	Returns the studio's saved custom block-based layout for its trial payment page. Returns an empty layout (null blocks/background) if the studio has never saved one, so the frontend can fall back to its built-in default.
//	@Tags			Booking Page
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id	path		string	true	"Studio ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	httpx.ErrorResponse	"invalid id"
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/api/v1/me/studios/{id}/trial-page-layout [get]
func (h *Handler) getTrialPageLayout(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	layout, err := h.svc.repo.GetTrialPageLayout(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if layout == nil {
		httpx.JSON(w, http.StatusOK, emptyTrialPageLayout)
		return
	}
	httpx.JSON(w, http.StatusOK, json.RawMessage(layout))
}

// putTrialPageLayout godoc
//
//	@Summary		Save the studio's custom trial payment page layout
//	@Description	Saves a custom block-based layout for the studio's trial payment page. The layout must contain exactly one card_fields block (Stripe card inputs) and exactly one pay_button block; all other block types are free-form.
//	@Tags			Booking Page
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string					true	"Studio ID"
//	@Param			body	body		map[string]interface{}	true	"Layout blocks and background (blocks, background)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid id or invalid block layout"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/me/studios/{id}/trial-page-layout [put]
func (h *Handler) putTrialPageLayout(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req struct {
		Blocks     json.RawMessage `json:"blocks"`
		Background json.RawMessage `json:"background"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := validateTrialPageBlocks(req.Blocks); err != nil {
		httpx.WriteValidationError(w, map[string]string{"blocks": err.Error()})
		return
	}
	combined, err := json.Marshal(map[string]json.RawMessage{"blocks": req.Blocks, "background": req.Background})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := h.svc.repo.SetTrialPageLayout(r.Context(), id, combined); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, json.RawMessage(combined))
}

// GetInitialContactDelay / PutInitialContactDelay are exported (unlike most
// handlers here) because they're mounted directly under the studio-scoped
// "/studios/{studioId}" route group in main.go, not via AdminRoutes (which is
// super-admin-only) — any active studio-admin needs to edit this setting.
//
// GetInitialContactDelay godoc
//
//	@Summary		Get initial contact delay
//	@Description	Returns how many minutes the system waits before sending the first outreach message to a new lead for this studio.
//	@Tags			Messaging Settings
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid id"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/initial-contact-delay [get]
func (h *Handler) GetInitialContactDelay(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	minutes, err := h.svc.repo.GetInitialContactDelayMinutes(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"initialContactDelayMinutes": minutes})
}

type putInitialContactDelayReq struct {
	InitialContactDelayMinutes int `json:"initialContactDelayMinutes"`
}

// PutInitialContactDelay godoc
//
//	@Summary		Set initial contact delay
//	@Description	Sets how many minutes the system waits before sending the first outreach message to a new lead for this studio. Value is clamped between 0 and 1440 minutes.
//	@Tags			Messaging Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string						true	"Studio ID"
//	@Param			body		body		putInitialContactDelayReq	true	"Delay in minutes"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid id"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/initial-contact-delay [put]
func (h *Handler) PutInitialContactDelay(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req putInitialContactDelayReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	minutes := req.InitialContactDelayMinutes
	if minutes < 0 {
		minutes = 0
	}
	if minutes > 1440 {
		minutes = 1440
	}
	if err := h.svc.repo.SetInitialContactDelayMinutes(r.Context(), id, minutes); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"initialContactDelayMinutes": minutes})
}

// GetAIReplyDelay / PutAIReplyDelay control how long the AI worker waits
// before sending its auto-reply to an inbound conversation message (not the
// first outreach to a new lead — see GetInitialContactDelay for that).
// Exported and mounted the same way as GetInitialContactDelay.
//
// GetAIReplyDelay godoc
//
//	@Summary		Get AI reply delay
//	@Description	Returns how many seconds the AI worker waits before sending its automated reply to an inbound conversation message for this studio.
//	@Tags			Messaging Settings
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid id"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/ai-reply-delay [get]
func (h *Handler) GetAIReplyDelay(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	seconds, err := h.svc.repo.GetAIReplyDelaySeconds(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"aiReplyDelaySeconds": seconds})
}

type putAIReplyDelayReq struct {
	AIReplyDelaySeconds int `json:"aiReplyDelaySeconds"`
}

// PutAIReplyDelay godoc
//
//	@Summary		Set AI reply delay
//	@Description	Sets how many seconds the AI worker waits before sending its automated reply to an inbound conversation message for this studio. Value is clamped between 0 and 300 seconds.
//	@Tags			Messaging Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string				true	"Studio ID"
//	@Param			body		body		putAIReplyDelayReq	true	"Delay in seconds"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid id"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/ai-reply-delay [put]
func (h *Handler) PutAIReplyDelay(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req putAIReplyDelayReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	seconds := req.AIReplyDelaySeconds
	if seconds < 0 {
		seconds = 0
	}
	if seconds > 300 {
		seconds = 300
	}
	if err := h.svc.repo.SetAIReplyDelaySeconds(r.Context(), id, seconds); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"aiReplyDelaySeconds": seconds})
}

type putCommunicationStyleReq struct {
	CommunicationStyleProfile string `json:"communicationStyleProfile"`
}

// PutCommunicationStyle lets a studio admin manually edit the "communication
// style" profile the style worker periodically distills from their own past
// staff replies (see messaging.StyleWorker). A manual save here also resets
// the worker's drift counter, same as an automatic rebuild — so the edit
// sticks until enough new staff replies accumulate to justify a fresh build.
func (h *Handler) PutCommunicationStyle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req putCommunicationStyleReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if len(req.CommunicationStyleProfile) > 4000 {
		httpx.WriteValidationError(w, map[string]string{"communicationStyleProfile": "must be 4,000 characters or less"})
		return
	}
	if err := h.svc.repo.SetCommunicationStyleProfile(r.Context(), id, req.CommunicationStyleProfile); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"communicationStyleProfile": req.CommunicationStyleProfile})
}

// publicGetTrialPageLayout is the customer-facing read — no auth, just the
// studio slug from the link. Returns null blocks/background if the studio
// never opened the builder, so the frontend renders its own built-in default.
//
// publicGetTrialPageLayout godoc
//
//	@Summary		Get the public trial payment page layout
//	@Description	Public, customer-facing endpoint that returns the studio's saved custom trial payment page layout by slug. Returns an empty layout (null blocks/background) if the studio never customized it, so the frontend falls back to its built-in default. No auth required.
//	@Tags			Studios (Public)
//	@Produce		json
//	@Param			slug	path		string	true	"Studio slug"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		404		{object}	httpx.ErrorResponse	"studio not found"
//	@Router			/api/v1/public/studios/{slug}/trial-page-layout [get]
func (h *Handler) publicGetTrialPageLayout(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	layout, err := h.svc.repo.GetTrialPageLayoutBySlug(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}
	if layout == nil {
		httpx.JSON(w, http.StatusOK, emptyTrialPageLayout)
		return
	}
	httpx.JSON(w, http.StatusOK, json.RawMessage(layout))
}

// publicCreateTrialPaymentIntent mirrors publicCreatePaymentIntent's
// embedded-Stripe-Elements pattern, but for the trial-link flow: amount
// resolution matches messaging.Service.createTrialCheckoutSession (studio
// trial_amount_sgd → lowest active plan → 2500 fallback), and metadata
// carries lead_id/studio_id/kind="trial" for the payment_intent.succeeded
// webhook handler to pick up (see webhook_stripe.go).
//
// publicCreateTrialPaymentIntent godoc
//
//	@Summary		Create a public Stripe PaymentIntent for a trial session
//	@Description	Public endpoint that creates a Stripe PaymentIntent for a lead's trial session, using the studio's configured trial amount, falling back to the lowest-priced active plan, then a 2500 (cents) default. Attaches kind="trial" metadata for the payment_intent.succeeded webhook to pick up. No auth required.
//	@Tags			Studios (Public)
//	@Produce		json
//	@Param			leadId	path		string	true	"Lead ID"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid lead id or Stripe not configured"
//	@Failure		404		{object}	httpx.ErrorResponse	"lead not found"
//	@Failure		500		{object}	httpx.ErrorResponse	"Stripe error"
//	@Router			/api/v1/public/leads/{leadId}/trial-payment-intent [post]
func (h *Handler) publicCreateTrialPaymentIntent(w http.ResponseWriter, r *http.Request) {
	leadID, err := uuid.Parse(chi.URLParam(r, "leadId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid lead id")
		return
	}

	var studioID uuid.UUID
	if err := h.svc.repo.Pool().QueryRow(r.Context(),
		"SELECT studio_id FROM leads WHERE id = $1", leadID,
	).Scan(&studioID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "lead not found")
		return
	}

	s, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil || s.StripeSecretKey == "" {
		httpx.WriteError(w, http.StatusBadRequest, "stripe_not_configured", "Stripe not connected for this studio")
		return
	}

	amount := h.svc.ResolveTrialAmountSGD(r.Context(), studioID, s.TrialAmountSGD)

	sc := &client.API{}
	sc.Init(s.StripeSecretKey, nil)
	pi, err := sc.PaymentIntents.New(&stripe.PaymentIntentParams{
		Amount:      stripe.Int64(amount),
		Currency:    stripe.String("sgd"),
		Description: stripe.String(fmt.Sprintf("%s Trial Session", s.Name)),
		Metadata: map[string]string{
			"studio_id": studioID.String(),
			"lead_id":   leadID.String(),
			"kind":      "trial",
		},
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "stripe_error", fmt.Sprintf("failed to create payment intent: %v", err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"clientSecret": pi.ClientSecret,
		"amount":       amount,
	})
}

// CreatePlatformCheckout godoc
//
//	@Summary		Create a checkout session for a new studio signing up to the platform
//	@Description	Creates a Stripe Checkout Session for a prospective new studio purchasing a platform subscription tier (not an existing studio's own checkout). "Trial Pass" is billed as a one-time payment; other tiers are monthly subscriptions. No auth required.
//	@Tags			Platform Signup
//	@Accept			json
//	@Produce		json
//	@Param			body	body		map[string]interface{}	true	"Selected tier (tier)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid JSON, platform Stripe not configured, or invalid tier"
//	@Failure		500		{object}	httpx.ErrorResponse	"Stripe error"
//	@Router			/api/v1/public/platform/checkout [post]
func (h *Handler) CreatePlatformCheckout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tier string `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request body")
		return
	}

	secretKey, _ := h.svc.GetPlatformSetting(r.Context(), "stripe_secret_key")
	if secretKey == "" {
		httpx.WriteError(w, http.StatusBadRequest, "platform_not_configured", "Platform Stripe account is not configured")
		return
	}

	// Fetch dynamic platform plans
	val, err := h.svc.GetPlatformSetting(r.Context(), "platform_plans")
	if err != nil || val == "" {
		val = `[{"name":"Trial Pass","price":300,"cycle":"One-time","description":"Entry-level setup to validate AI integration and lead generation.","features":["1 Connected Channel","Basic AI Auto-Replies (200/mo)","1-day automated follow-up","Google Sheets contact sync"]},{"name":"Growth Tier","price":999,"cycle":"Monthly","description":"Automate workflows, payments, and client acquisition.","features":["3 Connected Channels","Full AI Auto-Replies (2,000/mo)","Dedicated Knowledge Base","Visual drag-and-drop Pipeline","Stripe account integration"]},{"name":"Pro Tier","price":1299,"cycle":"Monthly","description":"For active studios looking to scale reach via social media and paid advertising.","features":["8 Connected Channels","Extended AI Auto-Replies (10,000/mo)","Dual model routing (Gemini + Claude)","Advanced Social Planner","Google Ads Channel Integration","Studio Plan Option (Scheduling)"],"highlight":true},{"name":"Enterprise Tier","price":1599,"cycle":"Monthly","description":"Maximum scale, custom branding, and multi-location management.","features":["Unlimited Connected Channels","Unlimited AI Auto-Replies","Multi-Location Hub","Whitelabel Dashboard","Enterprise Studio Plan Option","Priority Support SLA"]}]`
	}
	var plans []map[string]any
	json.Unmarshal([]byte(val), &plans)

	var amount int64 = -1
	for _, p := range plans {
		if p["name"] == req.Tier {
			amount = int64(p["price"].(float64)) * 100 // Convert to cents
			break
		}
	}

	if amount == -1 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tier", "Invalid subscription tier")
		return
	}

	sc := &client.API{}
	sc.Init(secretKey, nil)

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String("sgd"),
					UnitAmount: stripe.Int64(amount),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(fmt.Sprintf("1herosocial.ai - %s", req.Tier)),
					},
					Recurring: &stripe.CheckoutSessionLineItemPriceDataRecurringParams{
						Interval: stripe.String("month"),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String("subscription"),
		SuccessURL: stripe.String(fmt.Sprintf("%s/onboarding?session_id={CHECKOUT_SESSION_ID}", frontendURL)),
		CancelURL:  stripe.String(fmt.Sprintf("%s/pricing", frontendURL)),
		Metadata: map[string]string{
			"plan_tier": req.Tier,
		},
	}

	// For Trial Pass, it's one-time
	if req.Tier == "Trial Pass" {
		params.Mode = stripe.String("payment")
		params.LineItems[0].PriceData.Recurring = nil
	}

	session, err := sc.CheckoutSessions.New(params)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "stripe_error", fmt.Sprintf("Failed to create checkout session: %v", err))
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"url": session.URL,
	})
}

// ProvisionPlatformStudio godoc
//
//	@Summary		Provision a new studio after platform signup payment
//	@Description	Verifies a completed Stripe Checkout Session for a platform signup, then creates the new studio and its admin account using the email captured at checkout, assigning the subscription tier recorded in the session metadata. No auth required.
//	@Tags			Platform Signup
//	@Accept			json
//	@Produce		json
//	@Param			body	body		map[string]interface{}	true	"Provisioning details (sessionId, studioName, contactPhone, adminPassword)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid session, unpaid session, missing email, or email already in use"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/public/platform/provision [post]
func (h *Handler) ProvisionPlatformStudio(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionId     string `json:"sessionId"`
		StudioName    string `json:"studioName"`
		ContactPhone  string `json:"contactPhone"`
		AdminPassword string `json:"adminPassword"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	secretKey, err := h.svc.GetPlatformSetting(r.Context(), "stripe_secret_key")
	if err != nil || secretKey == "" {
		httpx.WriteError(w, http.StatusInternalServerError, "not_configured", "Platform Stripe is not configured")
		return
	}

	sc := &client.API{}
	sc.Init(secretKey, nil)

	session, err := sc.CheckoutSessions.Get(req.SessionId, nil)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_session", "Failed to retrieve checkout session")
		return
	}

	if session.PaymentStatus != "paid" {
		httpx.WriteError(w, http.StatusBadRequest, "unpaid_session", "Checkout session is not paid")
		return
	}

	customerEmail := ""
	if session.CustomerDetails != nil && session.CustomerDetails.Email != "" {
		customerEmail = session.CustomerDetails.Email
	} else if session.CustomerEmail != "" {
		customerEmail = session.CustomerEmail
	}

	if customerEmail == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_email", "Checkout session does not have an associated email")
		return
	}

	tier := ""
	if session.Metadata != nil {
		tier = session.Metadata["plan_tier"]
	}
	if tier == "" {
		tier = "Growth Tier" // fallback
	}

	// Try to create the studio
	res, errs, err := h.svc.CreateStudioWithAdmin(r.Context(), CreateStudioInput{
		Slug:                 "", // will auto-generate from Name
		Name:                 req.StudioName,
		BrandColor:           "#7c3aed",
		LogoURL:              "",
		ContactEmail:         customerEmail,
		ContactPhone:         req.ContactPhone,
		AdminEmail:           customerEmail,
		AdminPassword:        req.AdminPassword,
		SocialPlannerEnabled: true,
	})

	if errs != nil {
		// If the admin email is already in use, or slug taken
		if _, ok := errs["adminEmail"]; ok {
			// This email is already registered. They should probably just link it, but for now we error.
			httpx.WriteError(w, http.StatusBadRequest, "email_taken", "An account with this email already exists.")
			return
		}
		httpx.WriteValidationError(w, errs)
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	// Assign the subscription tier based on the payment
	if res != nil && res.Studio != nil {
		// Just update the tier via direct DB update or service wrapper
		_ = h.svc.UpdatePayments(r.Context(), res.Studio.ID, "", "", "", "", tier)
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "studioId": res.Studio.ID})
}

// linkStripe godoc
//
//	@Summary		Link or update a Stripe account
//	@Description	Saves the studio's Stripe account ID, publishable key, secret key, and webhook secret so the studio can accept payments. Pass "global" as the id to update the platform-level Stripe configuration instead of a specific studio.
//	@Tags			Billing
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string					true	"Studio ID, or \"global\" for platform-level settings"
//	@Param			body	body		map[string]interface{}	true	"Stripe account credentials (stripeAccountId, stripePublishableKey, stripeSecretKey, stripeWebhookSecret)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid JSON or invalid studio id"
//	@Failure		404		{object}	httpx.ErrorResponse	"studio not found"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/me/studios/{id}/payments/stripe [post]
func (h *Handler) linkStripe(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	var req struct {
		StripeAccountId      string `json:"stripeAccountId"`
		StripePublishableKey string `json:"stripePublishableKey"`
		StripeSecretKey      string `json:"stripeSecretKey"`
		StripeWebhookSecret  string `json:"stripeWebhookSecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request body")
		return
	}

	if idStr == "global" {
		if err := h.svc.UpdatePlatformSetting(r.Context(), "stripe_account_id", req.StripeAccountId); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update stripe_account_id")
			return
		}
		if err := h.svc.UpdatePlatformSetting(r.Context(), "stripe_publishable_key", req.StripePublishableKey); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update stripe_publishable_key")
			return
		}
		if err := h.svc.UpdatePlatformSetting(r.Context(), "stripe_secret_key", req.StripeSecretKey); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update stripe_secret_key")
			return
		}
		if req.StripeWebhookSecret != "" {
			if err := h.svc.UpdatePlatformSetting(r.Context(), "stripe_webhook_secret", req.StripeWebhookSecret); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update stripe_webhook_secret")
				return
			}
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid studio ID")
		return
	}

	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}

	err = h.svc.UpdatePayments(r.Context(), id, req.StripeAccountId, req.StripeSecretKey, req.StripePublishableKey, req.StripeWebhookSecret, s.SubscriptionTier)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// getBillingHistory godoc
//
//	@Summary		Get Stripe billing/payment history
//	@Description	Queries Stripe for succeeded PaymentIntents (trial and plan payments) for the studio's connected Stripe account and returns them as invoice-like line items along with lifetime-paid totals. Supports optional startDate/endDate Unix-timestamp filters. Pass "global" as the id to read platform-level Stripe history instead of a specific studio.
//	@Tags			Billing
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id			path		string	true	"Studio ID, or \"global\" for platform-level settings"
//	@Param			startDate	query		string	false	"Unix timestamp (seconds) to filter payments created on or after"
//	@Param			endDate		query		string	false	"Unix timestamp (seconds) to filter payments created on or before"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studio id"
//	@Failure		404			{object}	httpx.ErrorResponse	"studio not found"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/me/studios/{id}/billing/history [get]
func (h *Handler) getBillingHistory(w http.ResponseWriter, r *http.Request) {
	var stripeSecretKey string

	idStr := chi.URLParam(r, "id")
	if idStr == "global" {
		var err error
		stripeSecretKey, err = h.svc.GetPlatformSetting(r.Context(), "stripe_secret_key")
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		if stripeSecretKey == "" {
			httpx.WriteError(w, http.StatusInternalServerError, "wtf", "key is empty for global")
			return
		}
	} else {
		id, err := uuid.Parse(idStr)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid studio ID")
			return
		}

		s, err := h.svc.GetByID(r.Context(), id)
		if err != nil {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		stripeSecretKey = s.StripeSecretKey
	}

	if stripeSecretKey == "" {
		httpx.JSON(w, http.StatusOK, map[string]any{"invoices": []any{}, "stats": map[string]any{}})
		return
	}

	sc := &client.API{}
	sc.Init(stripeSecretKey, nil)

	// Checkout Sessions create PaymentIntents (not Invoices).
	// Query PaymentIntents to show all trial booking payments.
	piParams := &stripe.PaymentIntentListParams{}

	// Read optional date filters
	startDateStr := r.URL.Query().Get("startDate")
	endDateStr := r.URL.Query().Get("endDate")

	if startDateStr != "" || endDateStr != "" {
		createdParams := &stripe.RangeQueryParams{}
		if startDateStr != "" {
			if startUnix, err := strconv.ParseInt(startDateStr, 10, 64); err == nil {
				createdParams.GreaterThanOrEqual = startUnix
			}
		}
		if endDateStr != "" {
			if endUnix, err := strconv.ParseInt(endDateStr, 10, 64); err == nil {
				createdParams.LesserThanOrEqual = endUnix
			}
		}
		piParams.CreatedRange = createdParams
		piParams.Limit = stripe.Int64(100) // Increase limit when filtering
	} else {
		piParams.Limit = stripe.Int64(20) // Default limit
	}

	piParams.AddExpand("data.latest_charge")
	piParams.AddExpand("data.invoice")
	piIter := sc.PaymentIntents.List(piParams)

	invoices := make([]map[string]any, 0)
	var lifetimePaid int64
	var lifetimePaidByCurrency = map[string]int64{}

	for piIter.Next() {
		pi := piIter.PaymentIntent()
		if pi.Status != stripe.PaymentIntentStatusSucceeded {
			continue
		}

		receiptURL := ""
		buyerName := "Guest"
		description := pi.Description

		if pi.Invoice != nil {
			if pi.Invoice.Lines != nil && len(pi.Invoice.Lines.Data) > 0 {
				description = pi.Invoice.Lines.Data[0].Description
			}
			if pi.Invoice.CustomerName != "" {
				buyerName = pi.Invoice.CustomerName
			} else if pi.Invoice.CustomerEmail != "" {
				buyerName = pi.Invoice.CustomerEmail
			}
		}

		// Prefer plan_name from metadata for description.
		if planName := pi.Metadata["plan_name"]; planName != "" {
			description = planName
		}
		if description == "" || description == "Subscription creation" {
			description = "Payment"
		}

		campaignName := pi.Metadata["campaign_name"]
		campaignSlug := pi.Metadata["campaign_slug"]

		// Try to get receipt and buyer from latest charge
		if pi.LatestCharge != nil {
			receiptURL = pi.LatestCharge.ReceiptURL
			if pi.LatestCharge.BillingDetails != nil && pi.LatestCharge.BillingDetails.Name != "" {
				buyerName = pi.LatestCharge.BillingDetails.Name
			} else if pi.LatestCharge.BillingDetails != nil && pi.LatestCharge.BillingDetails.Email != "" {
				buyerName = pi.LatestCharge.BillingDetails.Email
			}
		}

		cur := string(pi.Currency)
		invoices = append(invoices, map[string]any{
			"id":                 pi.ID,
			"number":             pi.ID[3:11], // short reference
			"amount_due":         pi.Amount,
			"amount_paid":        pi.AmountReceived,
			"currency":           cur,
			"status":             "paid",
			"created":            pi.Created,
			"hosted_invoice_url": receiptURL,
			"invoice_pdf":        receiptURL,
			"description":        description,
			"buyer_name":         buyerName,
			"campaign_name":      campaignName,
			"campaign_slug":      campaignSlug,
			"metadata":           pi.Metadata,
		})

		lifetimePaidByCurrency[cur] += pi.AmountReceived
		lifetimePaid += pi.AmountReceived
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"invoices": invoices,
		"stats": map[string]any{
			"outstandingSGD":    int64(0),
			"lifetimePaidSGD":   lifetimePaidByCurrency["sgd"],
			"lifetimePaidTotal": lifetimePaid,
		},
	})
}

// ----- Stripe Connect OAuth (Phase 4) -----

// StripeConnectRedirect godoc
//
//	@Summary		Start Stripe Connect OAuth flow
//	@Description	Redirects the caller to Stripe's Connect OAuth authorization page so the studio can link its own Stripe account, passing the studio ID as the OAuth state parameter.
//	@Tags			Stripe
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Success		307			{string}	string	"Redirect to Stripe Connect authorization page"
//	@Router			/api/v1/studios/{studioId}/stripe-oauth/login [get]
func (h *Handler) StripeConnectRedirect(w http.ResponseWriter, r *http.Request) {
	studioID := chi.URLParam(r, "studioId")
	if studioID == "" {
		studioID = chi.URLParam(r, "id") // Fallback just in case
	}
	// The client_id should come from environment variables.
	clientID := os.Getenv("STRIPE_CLIENT_ID")
	redirectURI := fmt.Sprintf("%s/api/v1/auth/stripe/callback", os.Getenv("PUBLIC_URL"))

	stripeOAuthURL := fmt.Sprintf(
		"https://connect.stripe.com/oauth/authorize?response_type=code&client_id=%s&scope=read_write&redirect_uri=%s&state=%s",
		clientID, redirectURI, studioID,
	)

	http.Redirect(w, r, stripeOAuthURL, http.StatusTemporaryRedirect)
}

// StripeConnectCallback godoc
//
//	@Summary		Stripe Connect OAuth callback
//	@Description	OAuth redirect target that Stripe calls after the studio-admin authorizes the connection. Exchanges the authorization code for the connected Stripe account ID, saves it against the studio (identified via the OAuth state parameter), and redirects back to the frontend settings page. No auth required — this is a public redirect from Stripe.
//	@Tags			Stripe
//	@Param			code	query		string	true	"Stripe OAuth authorization code"
//	@Param			state	query		string	true	"Studio ID passed through as OAuth state"
//	@Success		307		{string}	string	"Redirect to frontend studio settings page"
//	@Failure		400		{object}	httpx.ErrorResponse	"missing code/state or invalid state"
//	@Failure		500		{object}	httpx.ErrorResponse	"Stripe not configured or OAuth exchange failed"
//	@Router			/api/v1/auth/stripe/callback [get]
func (h *Handler) StripeConnectCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state") // Studio ID passed in state

	if code == "" || state == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Missing code or state")
		return
	}

	studioID, err := uuid.Parse(state)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_state", "Invalid state parameter")
		return
	}

	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	if stripe.Key == "" {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Stripe secret key not configured on platform")
		return
	}

	params := &stripe.OAuthTokenParams{
		GrantType: stripe.String("authorization_code"),
		Code:      stripe.String(code),
	}

	token, err := stripeoauth.New(params)
	if err != nil {
		slog.Error("stripe oauth failed", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "stripe_error", "Failed to authenticate with Stripe")
		return
	}

	// Update the studio's payment configuration with the connected account ID
	s, err := h.svc.GetByID(r.Context(), studioID)
	if err == nil {
		_ = h.svc.UpdatePayments(r.Context(), studioID, token.StripeUserID, "", "", "", s.SubscriptionTier)
	}

	// Redirect back to frontend
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	http.Redirect(w, r, fmt.Sprintf("%s/admin/studios/%s/settings?tab=integrations", frontendURL, studioID), http.StatusTemporaryRedirect)
}

// listPlans godoc
//
//	@Summary		List a studio's membership plans
//	@Description	Returns all membership plans (active and inactive) configured for the studio.
//	@Tags			Billing
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id	path		string	true	"Studio ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	httpx.ErrorResponse	"invalid studio id"
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/api/v1/me/studios/{id}/plans [get]
func (h *Handler) listPlans(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}
	plans, err := h.svc.ListPlans(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to list plans")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"plans": plans})
}

type updatePlanReq struct {
	PlanName     *string   `json:"planName"`
	PriceSGD     *int      `json:"priceSgd"`
	BillingCycle *string   `json:"billingCycle"`
	Features     *[]string `json:"features"`
	IsActive     *bool     `json:"isActive"`
}

// updatePlan godoc
//
//	@Summary		Update a membership plan
//	@Description	Partially updates an existing membership plan for the studio.
//	@Tags			Billing
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string			true	"Studio ID"
//	@Param			planId	path		string			true	"Plan ID"
//	@Param			body	body		updatePlanReq	true	"Fields to update"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid studio or plan id"
//	@Failure		404		{object}	httpx.ErrorResponse	"plan not found"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/me/studios/{id}/plans/{planId} [put]
func (h *Handler) updatePlan(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}
	planID, err := uuid.Parse(chi.URLParam(r, "planId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_plan_id", "invalid plan id")
		return
	}
	var req updatePlanReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	err = h.svc.UpdatePlan(r.Context(), studioID, planID, UpdatePlanInput{
		PlanName:     req.PlanName,
		PriceSGD:     req.PriceSGD,
		BillingCycle: req.BillingCycle,
		Features:     req.Features,
		IsActive:     req.IsActive,
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "plan not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update plan")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"status": "success"})
}

type createPlanReq struct {
	PlanName     string   `json:"planName"`
	PriceSGD     int      `json:"priceSgd"`
	BillingCycle string   `json:"billingCycle"`
	Features     []string `json:"features"`
	IsActive     bool     `json:"isActive"`
}

// createPlan godoc
//
//	@Summary		Create a membership plan
//	@Description	Creates a new membership plan for the studio.
//	@Tags			Billing
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string			true	"Studio ID"
//	@Param			body	body		createPlanReq	true	"Plan details"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid studio id or missing planName"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/me/studios/{id}/plans [post]
func (h *Handler) createPlan(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}
	var req createPlanReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.PlanName == "" {
		httpx.WriteError(w, http.StatusBadRequest, "validation", "planName is required")
		return
	}
	plan, err := h.svc.CreatePlan(r.Context(), studioID, CreatePlanInput{
		PlanName:     req.PlanName,
		PriceSGD:     req.PriceSGD,
		BillingCycle: req.BillingCycle,
		Features:     req.Features,
		IsActive:     req.IsActive,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to create plan")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"plan": plan})
}

// deletePlan godoc
//
//	@Summary		Delete a membership plan
//	@Description	Permanently deletes a membership plan from the studio.
//	@Tags			Billing
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string	true	"Studio ID"
//	@Param			planId	path		string	true	"Plan ID"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid studio or plan id"
//	@Failure		404		{object}	httpx.ErrorResponse	"plan not found"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/me/studios/{id}/plans/{planId} [delete]
func (h *Handler) deletePlan(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}
	planID, err := uuid.Parse(chi.URLParam(r, "planId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_plan_id", "invalid plan id")
		return
	}
	if err := h.svc.DeletePlan(r.Context(), studioID, planID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "plan not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to delete plan")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// deleteAccount godoc
//
//	@Summary		Delete a studio account
//	@Description	Permanently deletes the studio and all associated data (cascading via foreign key constraints), after confirming the caller-supplied email matches the studio's contact email. Caller must belong to the studio being deleted.
//	@Tags			Studios
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			id		path		string					true	"Studio ID"
//	@Param			body	body		map[string]interface{}	true	"Confirmation email (email)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid studio id, invalid JSON, or email mismatch"
//	@Failure		403		{object}	httpx.ErrorResponse	"caller does not belong to this studio"
//	@Failure		404		{object}	httpx.ErrorResponse	"studio not found"
//	@Failure		500		{object}	httpx.ErrorResponse	"delete failed"
//	@Router			/api/v1/me/studios/{id}/delete-account [delete]
func (h *Handler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}

	// Get authenticated user claims
	claims := identity.MustClaims(r.Context())
	if claims.StudioID == nil || claims.StudioID.String() != studioID.String() {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to delete this studio")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request")
		return
	}

	// Verify email matches
	studio, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil || studio == nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}

	if studio.ContactEmail != req.Email {
		httpx.WriteError(w, http.StatusBadRequest, "email_mismatch", "email does not match studio contact email")
		return
	}

	// Delete the studio and all associated data (cascading delete via FK constraints)
	_, err = h.svc.repo.Pool().Exec(r.Context(), `
		DELETE FROM studios WHERE id = $1
	`, studioID)

	if err != nil {
		// Log the actual error for debugging
		fmt.Fprintf(os.Stderr, "studio deletion error: %v\n", err)
		httpx.WriteError(w, http.StatusInternalServerError, "delete_failed", "failed to delete studio")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
