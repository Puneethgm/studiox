package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/integrations/claude"
	"github.com/projectx/api/internal/integrations/google"
	"github.com/projectx/api/internal/integrations/sheets"
	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/messaging"
	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/db"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/logger"
	"github.com/projectx/api/internal/platform/secrets"
	"github.com/projectx/api/internal/studios"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel)

	rootCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.Connect(rootCtx, cfg.DB.DSN())
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	cipher, err := secrets.New(cfg.TokenEncryptionKey)
	if err != nil {
		log.Error("token encryption init", "err", err)
		os.Exit(1)
	}

	// --- repos / services / handlers ---
	identityRepo := identity.NewRepo(pool)
	leadsRepo := leads.NewRepo(pool)
	studiosRepo := studios.NewRepo(pool, cipher)

	tokens := identity.NewTokenIssuer(cfg.JWT.Secret, cfg.JWT.TTL)

	studiosSvc := studios.NewService(studiosRepo, identityRepo)
	studiosHandler := studios.NewHandler(studiosSvc, cfg.Sheets.CredentialsPath)

	// Identity needs to enrich /me + /login responses with the user's studio
	// brand info. Wire studios in via a callback to keep the import direction one-way.
	brandLookup := identity.StudioBrandLookup(func(ctx context.Context, id uuid.UUID) (*identity.StudioBrand, error) {
		s, err := studiosSvc.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		return &identity.StudioBrand{
			Slug:                 s.Slug,
			Name:                 s.Name,
			BrandColor:           s.BrandColor,
			LogoURL:              s.LogoURL,
			Active:               s.Active,
			SocialPlannerEnabled: s.SocialPlannerEnabled,
		}, nil
	})
	identityHandler := identity.NewHandler(identityRepo, tokens, cfg.Cookie, brandLookup)

	leadsSvc := leads.NewService(leadsRepo)
	leadsHandler := leads.NewHandler(leadsSvc, cfg)

	sheetsClient, err := sheets.NewClient(rootCtx, cfg.Sheets.CredentialsPath)
	if err != nil {
		log.Error("sheets init failed — leads will queue in outbox until fixed", "err", err)
	}
	sheetsWorker := sheets.NewWorker(leadsRepo, sheetsClient, cfg.Sheets.CredentialsPath, log.With("component", "sheets_worker"))
	go sheetsWorker.Run(rootCtx)

	// --- messaging (channels + inbox) ---
	msgRepo := messaging.NewRepo(pool, cipher)
	msgBus := messaging.NewInProcBus()
	msgSvc := messaging.NewService(msgRepo, msgBus, cfg.PublicFormBaseURL)
	msgHandler := messaging.NewHandler(msgSvc, msgBus)

	whatsappClient := channels.NewMetaWhatsApp(cfg.Meta.GraphAPIVersion)
	messengerClient := channels.NewMetaMessenger(cfg.Meta.GraphAPIVersion)
	twilioClient := channels.NewTwilioSMS()
	xClient := channels.NewXSender()
	msgWorker := messaging.NewOutboundWorker(msgRepo, msgBus, whatsappClient, messengerClient, twilioClient, xClient,
		log.With("component", "messaging_worker"))
	go msgWorker.Run(rootCtx)

	// Auto-contact worker: picks up lead_autocontact outbox items
	autoWorker := messaging.NewAutoContactWorker(leadsRepo, msgRepo, msgSvc, log.With("component", "autocontact_worker"))
	go autoWorker.Run(rootCtx)

	// Claude AI worker
	claudeClient, err := claude.New(cfg.Claude.APIURL, cfg.Claude.APIKey)
	if err != nil {
		log.Error("init claude client", "err", err)
	}
	log.Info("claude config", "enabled", claudeClient != nil)
	aiWorker := messaging.NewAIWorker(msgBus, msgRepo, msgSvc, studiosRepo, leadsRepo, claudeClient, log.With("component", "ai_worker"))
	go aiWorker.Run(rootCtx)

	// Social Publisher worker
	socialWorker := studios.NewSocialWorker(pool, cfg.TokenEncryptionKey, log.With("component", "social_worker"))
	go socialWorker.Run(rootCtx)

	metaWebhook := messaging.NewMetaWebhookHandler(msgSvc,
		cfg.Meta.WebhookVerifyToken, cfg.Meta.AppSecret,
		log.With("component", "meta_webhook"))

	twilioWebhook := messaging.NewTwilioWebhookHandler(msgSvc, log.With("component", "twilio_webhook"))
	xWebhook := messaging.NewTwitterWebhookHandler(msgSvc, log.With("component", "x_webhook"))

	googleOAuth := google.NewOAuthHandler(studiosSvc, msgRepo, cfg.PublicFormBaseURL)

	// --- router ---
	r := chi.NewRouter()

	r.Use(httpx.RequestID)
	r.Use(httpx.RateLimiter)
	r.Use(httpx.Recoverer(log))
	r.Use(httpx.AccessLog(log))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Serve uploaded media files (created by the /messaging/upload endpoint)
	uploadsDir := "./uploads"
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		log.Error("create uploads dir", "err", err)
	}
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadsDir))))

	r.Route("/api/v1", func(r chi.Router) {
		identityHandler.Routes(r)

		// Public, unauthenticated endpoints
		studiosHandler.PublicRoutes(r)
		leadsHandler.PublicRoutes(r)
		msgHandler.PublicRoutes(r)

		// Google OAuth endpoints (unauthenticated callbacks)
		r.Get("/auth/google/callback", googleOAuth.CallbackHandler)
		r.Get("/auth/stripe/callback", studiosHandler.StripeConnectCallback)

		// Meta webhooks (WA, FB, IG)
		// We provide separate URLs for clarity, though the handler logic handles all types.
		r.Get("/webhooks/meta/whatsapp", metaWebhook.Verify)
		r.Post("/webhooks/meta/whatsapp", metaWebhook.Receive)

		r.Get("/webhooks/meta/messenger", metaWebhook.Verify)
		r.Get("/webhooks/meta/messenger/", metaWebhook.Verify)
		r.Post("/webhooks/meta/messenger", metaWebhook.Receive)
		r.Post("/webhooks/meta/messenger/", metaWebhook.Receive)
		r.Post("/webhooks/twilio", twilioWebhook.HandleInbound)
		r.Get("/webhooks/x", xWebhook.HandleInbound)
		r.Post("/webhooks/x", xWebhook.HandleInbound)
		
		stripeWebhook := studios.NewStripeWebhookHandler(studiosSvc, os.Getenv("STRIPE_WEBHOOK_SECRET"))
		r.Post("/webhooks/stripe", stripeWebhook.HandleInbound)

		r.Get("/webhooks/meta/instagram", metaWebhook.Verify)
		r.Post("/webhooks/meta/instagram", metaWebhook.Receive)

		// Authenticated
		r.Group(func(r chi.Router) {
			r.Use(identityHandler.RequireAuth)

			// Super-admin only: studio CRUD + create-with-admin
			r.Route("/admin", func(r chi.Router) {
				r.Use(identity.RequireRole(identity.RoleSuperAdmin))
				studiosHandler.AdminRoutes(r)
				leadsHandler.AdminRoutes(r)
			})

			// Any authenticated user: read/update OWN studio (studio_admin) or
			// any studio (super_admin). Path: /me/studios/{id}.
			// Studio-admins of inactive studios are blocked here too (super passes through).
			r.Route("/me", func(r chi.Router) {
				r.Use(studiosHandler.RequireActiveStudio)
				studiosHandler.SelfRoutes(r)
			})

			// Studio-scoped campaigns + leads + messaging. Path: /studios/{studioId}/...
			// Authorization is per-handler via resolveStudioID; the middleware
			// gates against inactive studios for non-super-admins.
			r.Route("/studios/{studioId}", func(r chi.Router) {
				r.Use(studiosHandler.RequireActiveStudio)
				r.Get("/google-oauth/login", googleOAuth.LoginHandler)
				r.Get("/stripe-oauth/login", studiosHandler.StripeConnectRedirect)
				leadsHandler.AdminRoutes(r)
				r.Get("/social-posts", studiosHandler.ListSocialPosts)
				r.Post("/social-posts", studiosHandler.CreateSocialPost)
				r.Delete("/social-posts/{postId}", studiosHandler.DeleteSocialPost)
				r.Route("/messaging", func(r chi.Router) {
					msgHandler.AdminRoutes(r)
				})
			})
		})
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Env, "sheets_enabled", sheetsClient != nil)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen", "err", err)
			cancel()
		}
	}()

	<-rootCtx.Done()
	log.Info("shutdown initiated")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
	log.Info("bye")
}
