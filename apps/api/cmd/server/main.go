package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/decisiontree"
	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/integrations/claude"
	"github.com/projectx/api/internal/integrations/glofox"
	"github.com/projectx/api/internal/integrations/glofox/firstsession"
	"github.com/projectx/api/internal/integrations/google"
	"github.com/projectx/api/internal/integrations/sheets"
	"github.com/projectx/api/internal/integrations/sheets/inbound"
	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/messaging"
	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/db"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/logger"
	s3pkg "github.com/projectx/api/internal/platform/s3"
	"github.com/projectx/api/internal/platform/secrets"
	"github.com/projectx/api/internal/reviews"
	"github.com/projectx/api/internal/studios"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel)
	slog.SetDefault(log)

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
	dtRepo := decisiontree.NewRepo(pool)
	dtSvc := decisiontree.NewService(dtRepo)
	dtHandler := decisiontree.NewHandler(dtSvc)
	studiosRepo := studios.NewRepo(pool, cipher)
	reviewsRepo := reviews.NewRepo(pool)

	tokens := identity.NewTokenIssuer(cfg.JWT.Secret, cfg.JWT.TTL)

	glofoxClient := glofox.New(cfg.Glofox.APIKey, cfg.Glofox.APIToken, cfg.Glofox.BranchID)
	if glofoxClient != nil {
		log.Info("Glofox | Integration enabled — studio admins and lead conversions will sync to Glofox CRM",
			"component", "glofox",
			"branch_id", cfg.Glofox.BranchID,
		)
	} else {
		log.Info("Glofox | Integration disabled — set GLOFOX_API_KEY, GLOFOX_API_TOKEN, GLOFOX_BRANCH_ID to enable",
			"component", "glofox",
		)
	}
	// Wire Glofox into the repo directly so automated status changes (AI
	// worker, sheet import) sync too, not just the manual "edit lead" flow.
	leadsRepo.SetGlofoxClient(glofoxClient)

	studiosSvc := studios.NewService(studiosRepo, identityRepo, glofoxClient)
	reviewsHandler := reviews.NewHandler(reviewsRepo)

	// Initialize S3 uploader if configured
	var s3Uploader *s3pkg.Uploader
	if cfg.S3.Enabled() {
		awsCfg, err := awsconfig.LoadDefaultConfig(
			rootCtx,
			awsconfig.WithRegion(cfg.S3.Region),
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.S3.AccessKeyID, cfg.S3.SecretKey, "",
			)),
		)
		if err != nil {
			log.Error("AWS config failed (S3 uploads disabled)", "err", err)
		} else {
			s3Client := s3.NewFromConfig(awsCfg)
			s3Uploader = s3pkg.NewUploader(s3Client, cfg.S3.Bucket, cfg.S3.PublicURLBase)
			log.Info("S3 uploader initialized", "bucket", cfg.S3.Bucket)
		}
	} else {
		log.Info("S3 not configured (using disk for uploads)")
	}

	studiosHandler := studios.NewHandler(studiosSvc, cfg.Sheets.CredentialsPath, s3Uploader)

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
			SubscriptionTier:     s.SubscriptionTier,
		}, nil
	})
	identityHandler := identity.NewHandler(identityRepo, tokens, cfg.Cookie, brandLookup)

	leadsSvc := leads.NewService(leadsRepo, glofoxClient)
	leadsHandler := leads.NewHandler(leadsSvc, cfg)

	sheetsClient, err := sheets.NewClient(rootCtx, cfg.Sheets.CredentialsPath)
	if err != nil {
		log.Error("sheets init failed — leads will queue in outbox until fixed", "err", err)
	}
	sheetsWorker := sheets.NewWorker(leadsRepo, sheetsClient, cfg.Sheets.CredentialsPath, log.With("component", "sheets_worker"))
	go sheetsWorker.Run(rootCtx)

	// External leads sheet worker: polls each studio's configured, read-only
	// third-party company Google Sheet for new rows and imports them as
	// leads, which automatically enqueues the WhatsApp autocontact automation.
	externalLeadsWorker := inbound.New(sheetsClient, leadsRepo, log.With("component", "external_leads_sheet"))
	go externalLeadsWorker.Run(rootCtx)

	// --- messaging (channels + inbox) ---
	msgRepo := messaging.NewRepo(pool, cipher)
	msgBus := messaging.NewInProcBus()
	msgSvc := messaging.NewService(msgRepo, msgBus, cfg.PublicFormBaseURL, cfg.PublicAPIBaseURL)
	msgHandler := messaging.NewHandler(msgSvc, msgBus)

	// Wire DND job-cancellation callback into leads, keeping the import
	// direction one-way (leads doesn't import messaging).
	leadsSvc.SetCancelPendingMessagesFunc(msgRepo.CancelPendingJobsForLead)

	whatsappClient := channels.NewMetaWhatsApp(cfg.Meta.GraphAPIVersion)
	messengerClient := channels.NewMetaMessenger(cfg.Meta.GraphAPIVersion)
	twilioClient := channels.NewTwilioSMS()
	xClient := channels.NewXSender()
	telegramClient := channels.NewTelegramSender()
	msgWorker := messaging.NewOutboundWorker(msgRepo, msgBus, whatsappClient, messengerClient, twilioClient, xClient, telegramClient,
		log.With("component", "messaging_worker"))
	go msgWorker.Run(rootCtx)

	// Auto-contact worker: picks up lead_autocontact outbox items
	autoWorker := messaging.NewAutoContactWorker(leadsRepo, msgRepo, msgSvc, studiosRepo, log.With("component", "autocontact_worker"))
	go autoWorker.Run(rootCtx)

	// Glofox first-session worker: polls Glofox for members who purchased a plan
	// and attended at least one session, then sends a WhatsApp via the decision tree.
	// Auto-detect the studio by finding whichever studio has an active WhatsApp channel.
	var glofoxStudioID uuid.UUID
	if glofoxClient != nil {
		var sid uuid.UUID
		err := pool.QueryRow(rootCtx,
			`SELECT studio_id FROM channel_accounts
			 WHERE kind IN ('whatsapp_meta','whatsapp_web') AND status = 'active'
			 ORDER BY created_at ASC LIMIT 1`).Scan(&sid)
		if err == nil {
			glofoxStudioID = sid
			log.Info("Glofox | Auto-detected studio for first-session worker",
				"component", "glofox_first_session", "studio_id", glofoxStudioID)
		} else {
			log.Warn("Glofox | Could not auto-detect studio (no active WhatsApp channel found) — first-session worker will not start",
				"component", "glofox_first_session", "err", err)
		}
	}
	firstSessionWorker := firstsession.New(glofoxClient, pool, leadsRepo, msgSvc, msgRepo,
		glofoxStudioID, cfg.Glofox.BranchID, log.With("component", "glofox_first_session"))
	go firstSessionWorker.Run(rootCtx)

	// Claude AI worker
	claudeClient, err := claude.New(cfg.Claude.APIURL, cfg.Claude.APIKey)
	if err != nil {
		log.Error("init claude client", "err", err)
	}
	log.Info("claude config", "enabled", claudeClient != nil)
	aiWorker := messaging.NewAIWorker(msgBus, msgRepo, msgSvc, studiosRepo, leadsRepo, dtSvc, claudeClient, log.With("component", "ai_worker"))
	go aiWorker.Run(rootCtx)

	// Social Publisher worker
	socialWorker := studios.NewSocialWorker(pool, cfg.TokenEncryptionKey, log.With("component", "social_worker"))
	go socialWorker.Run(rootCtx)

	metaWebhook := messaging.NewMetaWebhookHandler(msgSvc,
		cfg.Meta.WebhookVerifyToken, cfg.Meta.AppSecret,
		log.With("component", "meta_webhook"))

	twilioWebhook := messaging.NewTwilioWebhookHandler(msgSvc, log.With("component", "twilio_webhook"))
	xWebhook := messaging.NewTwitterWebhookHandler(msgSvc, log.With("component", "x_webhook"))
	telegramWebhook := messaging.NewTelegramWebhookHandler(msgSvc, log.With("component", "telegram_webhook"))

	googleOAuth := google.NewOAuthHandler(studiosSvc, msgRepo, cfg.PublicFormBaseURL)

	// --- router ---
	r := chi.NewRouter()

	r.Use(httpx.RequestID)
	r.Use(httpx.SecurityHeaders)
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

	// Internal routes — only reachable from within the Docker network (wa-web service).
	// Not exposed through nginx; bound to the same port but path-protected.
	r.Route("/internal", func(r chi.Router) {
		msgHandler.InternalRoutes(r)
	})

	r.Route("/api/v1", func(r chi.Router) {
		identityHandler.Routes(r)

		// Public, unauthenticated endpoints
		studiosHandler.PublicRoutes(r)
		r.Post("/public/platform/checkout", studiosHandler.CreatePlatformCheckout)
		r.Post("/public/platform/provision", studiosHandler.ProvisionPlatformStudio)
		leadsHandler.PublicRoutes(r)
		msgHandler.PublicRoutes(r)

		// Reviews endpoints
		r.Post("/reviews", reviewsHandler.Create)
		r.Get("/reviews", reviewsHandler.ListAll)

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
		r.Post("/webhooks/telegram/{botID}", telegramWebhook.HandleInbound)

		stripeWebhook := studios.NewStripeWebhookHandler(studiosSvc, os.Getenv("STRIPE_WEBHOOK_SECRET"))
		r.Post("/webhooks/stripe", stripeWebhook.HandleInbound)
		r.Post("/webhooks/stripe/{studioId}", stripeWebhook.HandleInbound)

		r.Get("/webhooks/meta/instagram", metaWebhook.Verify)
		r.Post("/webhooks/meta/instagram", metaWebhook.Receive)

		// Meta data deletion (GDPR/privacy compliance)
		r.Post("/webhooks/meta/data-deletion", metaWebhook.HandleDataDeletion)

		// Authenticated
		r.Group(func(r chi.Router) {
			r.Use(identityHandler.RequireAuth)

			// Super-admin only: studio CRUD + create-with-admin
			r.Route("/admin", func(r chi.Router) {
				r.Use(identity.RequireRole(identity.RoleSuperAdmin))
				studiosHandler.AdminRoutes(r)
				leadsHandler.AdminRoutes(r)
				msgHandler.SuperAdminRoutes(r)
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
				dtHandler.AdminRoutes(r)
				r.Get("/google-oauth/login", googleOAuth.LoginHandler)
				r.Get("/stripe-oauth/login", studiosHandler.StripeConnectRedirect)
				r.Get("/initial-contact-delay", studiosHandler.GetInitialContactDelay)
				r.Put("/initial-contact-delay", studiosHandler.PutInitialContactDelay)
				leadsHandler.AdminRoutes(r)
				r.Get("/social-posts", studiosHandler.ListSocialPosts)
				r.Post("/social-posts", studiosHandler.CreateSocialPost)
				r.Put("/social-posts/{postId}", studiosHandler.UpdateSocialPost)
				r.Delete("/social-posts/{postId}", studiosHandler.DeleteSocialPost)
				r.Post("/social-posts/upload-image", studiosHandler.UploadSocialPostImage)
				identityHandler.StudioRoutes(r)
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
