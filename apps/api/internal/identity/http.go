package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/logger"
	"github.com/projectx/api/internal/platform/mail"
)

type Handler struct {
	repo        *Repo
	tokens      *TokenIssuer
	cookie      config.CookieConfig
	studioBrand StudioBrandLookup
	sessions    *SessionStore
	mailer      *mail.Sender
	frontendURL string
}

// StudioBrand is a minimal projection of the studio table — just enough for
// the frontend to render branded chrome immediately after login. The `Active`
// flag drives the inactive-studio lockout in the AppShell.
type StudioBrand struct {
	Slug                 string `json:"slug"`
	Name                 string `json:"name"`
	BrandColor           string `json:"brandColor"`
	LogoURL              string `json:"logoUrl"`
	Active               bool   `json:"active"`
	SocialPlannerEnabled bool   `json:"socialPlannerEnabled"`
	SubscriptionTier     string `json:"subscriptionTier"`
}

// StudioBrandLookup resolves a studio's brand info by id. Implemented in main
// using the studios package — kept as a func so identity has no compile-time
// dependency on studios.
type StudioBrandLookup func(ctx context.Context, id uuid.UUID) (*StudioBrand, error)

func NewHandler(repo *Repo, tokens *TokenIssuer, cookie config.CookieConfig, brand StudioBrandLookup, sessions *SessionStore, mailer *mail.Sender, frontendURL string) *Handler {
	return &Handler{repo: repo, tokens: tokens, cookie: cookie, studioBrand: brand, sessions: sessions, mailer: mailer, frontendURL: frontendURL}
}

func (h *Handler) Routes(r chi.Router) {
	r.With(httpx.AuthRateLimiter).Post("/auth/login", h.login)
	r.Post("/auth/logout", h.logout)
	r.With(h.RequireAuth).Get("/auth/me", h.me)
	// Also doubles as the forced-first-login password reset for a teammate
	// created with the default password — see RequirePasswordSet's doc
	// comment for why no special-casing is needed here: this route lives
	// outside the middleware chain that check wraps.
	r.With(h.RequireAuth, httpx.AuthRateLimiter).Post("/auth/password", h.changePassword)
	r.With(httpx.AuthRateLimiter).Post("/auth/forgot-password", h.forgotPassword)
	r.With(httpx.AuthRateLimiter).Post("/auth/reset-password", h.resetPassword)
	r.With(h.RequireAuth).Get("/permissions", h.listPermissions)
}

func (h *Handler) StudioRoutes(r chi.Router) {
	r.Get("/users", h.listStudioUsers)
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type meRes struct {
	ID       uuid.UUID    `json:"id"`
	Email    string       `json:"email"`
	Role     Role         `json:"role"`
	Username *string      `json:"username,omitempty"`
	StudioID *uuid.UUID   `json:"studioId,omitempty"`
	Studio   *StudioBrand `json:"studio,omitempty"`
	// Permissions is null for super_admin/studio_admin (full access, no
	// filtering) and a (possibly empty) list of granted nav-section keys
	// for studio_staff — see AppShell.tsx's nav filter.
	Permissions       []string `json:"permissions"`
	MustResetPassword bool     `json:"mustResetPassword"`
}

func (h *Handler) buildMeRes(ctx context.Context, u *User) meRes {
	res := meRes{
		ID: u.ID, Email: u.Email, Role: u.Role, Username: u.Username, StudioID: u.StudioID,
		MustResetPassword: u.MustResetPassword,
	}
	if u.Role == RoleStudioStaff {
		perms, err := h.repo.GetRolePermissionKeys(ctx, roleIDOrNil(u.RoleID))
		if err == nil {
			res.Permissions = perms
			if res.Permissions == nil {
				res.Permissions = []string{}
			}
		}
	}
	if u.StudioID != nil && h.studioBrand != nil {
		if b, err := h.studioBrand(ctx, *u.StudioID); err == nil && b != nil {
			res.Studio = b
		}
	}
	return res
}

func roleIDOrNil(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

// issueSessionCookie signs a fresh JWT for u (resolving studio_staff
// permissions as of right now) and sets it as the session cookie. Used by
// both login and changePassword — the latter needs this so a successful
// password change (in particular the forced-reset flow) immediately
// reflects MustResetPassword=false, rather than leaving the caller stuck
// behind RequirePasswordSet until they explicitly log out and back in.
func (h *Handler) issueSessionCookie(w http.ResponseWriter, r *http.Request, u *User) error {
	var permissions []string
	if u.Role == RoleStudioStaff && u.RoleID != nil {
		var err error
		permissions, err = h.repo.GetRolePermissionKeys(r.Context(), *u.RoleID)
		if err != nil {
			return err
		}
	}

	token, exp, jti, err := h.tokens.Issue(u, permissions)
	if err != nil {
		return err
	}
	if err := h.sessions.Create(r.Context(), jti, u.ID); err != nil {
		return err
	}
	// If this call is replacing an already-authenticated session (e.g.
	// changePassword re-issuing after a password change), revoke the old
	// jti so the previous cookie stops working the instant this one is
	// set, instead of both being valid until the old one idle-times-out.
	if old, ok := ClaimsFrom(r.Context()); ok && old.ID != jti {
		_ = h.sessions.Revoke(r.Context(), old.ID)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.cookie.Name,
		Value:    token,
		Path:     "/",
		Domain:   h.cookie.Domain,
		Expires:  exp,
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

// login godoc
//
//	@Summary		Log in with email and password
//	@Description	Verifies credentials, issues a JWT, and sets it as an HttpOnly session cookie. Rate limited to 10 requests/minute per IP.
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		loginReq	true	"Login credentials"
//	@Success		200		{object}	meRes
//	@Failure		401		{object}	httpx.ErrorResponse	"invalid email or password"
//	@Failure		422		{object}	httpx.ErrorResponse	"email or password missing"
//	@Failure		429		{object}	httpx.ErrorResponse	"too many attempts"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/auth/login [post]
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		httpx.WriteValidationError(w, map[string]string{"email": "required", "password": "required"})
		return
	}

	u, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if !VerifyPassword(u.PasswordHash, req.Password) {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}
	if !u.Active {
		httpx.WriteError(w, http.StatusUnauthorized, "account_inactive", "this account has been deactivated")
		return
	}
	if u.Role == RoleStudioStaff && u.RoleID != nil {
		roleActive, err := h.repo.IsRoleActive(r.Context(), *u.RoleID)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		if !roleActive {
			httpx.WriteError(w, http.StatusUnauthorized, "role_inactive", "your role has been deactivated")
			return
		}
	}

	if err := h.issueSessionCookie(w, r, u); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, h.buildMeRes(r.Context(), u))
}

// logout godoc
//
//	@Summary		Log out
//	@Description	Clears the session cookie by expiring it immediately. No auth required (safe to call even if no session exists).
//	@Tags			Auth
//	@Success		204	"no content"
//	@Router			/api/v1/auth/logout [post]
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	// Best-effort: revoke the session so the cookie can't keep working if
	// it's somehow replayed after this response (e.g. an old copy cached by
	// a proxy) — a parse failure here just means there's nothing to revoke.
	if cookie, err := r.Cookie(h.cookie.Name); err == nil && cookie.Value != "" {
		if claims, err := h.tokens.Parse(cookie.Value); err == nil {
			_ = h.sessions.Revoke(r.Context(), claims.ID)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.cookie.Name,
		Value:    "",
		Path:     "/",
		Domain:   h.cookie.Domain,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteStrictMode,
	})
	httpx.NoContent(w)
}

// me godoc
//
//	@Summary		Get current session user
//	@Description	Returns the authenticated user's profile and, if assigned to a studio, that studio's branding info. Clears the session cookie and returns 401 if the studio backing the session no longer exists/is accessible.
//	@Tags			Auth
//	@Security		CookieAuth
//	@Produce		json
//	@Success		200	{object}	meRes
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Router			/api/v1/auth/me [get]
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	c := MustClaims(r.Context())
	u, err := h.repo.FindByID(r.Context(), c.UserID)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "session no longer valid")
		return
	}

	// If the user has a studio, ensure it actually exists/is accessible.
	// If the database was reset but the JWT is still valid, this prevents
	// the user from getting stuck in a 403 loop on all other pages.
	if u.StudioID != nil && h.studioBrand != nil {
		b, err := h.studioBrand(r.Context(), *u.StudioID)
		if err != nil || b == nil {
			// Clear the invalid cookie
			http.SetCookie(w, &http.Cookie{
				Name:     h.cookie.Name,
				Value:    "",
				Path:     "/",
				Domain:   h.cookie.Domain,
				Expires:  time.Unix(0, 0),
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   h.cookie.Secure,
				SameSite: http.SameSiteStrictMode,
			})
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "studio no longer accessible")
			return
		}
	}

	httpx.JSON(w, http.StatusOK, h.buildMeRes(r.Context(), u))
}

type changePasswordReq struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

// changePassword godoc
//
//	@Summary		Change the current user's password
//	@Description	Validates the new password and confirmation, verifies the current password, then updates the stored password hash. Rate limited to 10 requests/minute per IP.
//	@Tags			Auth
//	@Security		CookieAuth
//	@Accept			json
//	@Param			body	body	changePasswordReq	true	"Current and new password"
//	@Success		204		"no content"
//	@Failure		401		{object}	httpx.ErrorResponse	"session invalid or current password incorrect for auth check"
//	@Failure		422		{object}	httpx.ErrorResponse	"validation failed"
//	@Failure		429		{object}	httpx.ErrorResponse	"too many attempts"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/auth/password [post]
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	errs := map[string]string{}
	if strings.TrimSpace(req.CurrentPassword) == "" {
		errs["currentPassword"] = "required"
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		errs["newPassword"] = "required"
	} else if len(req.NewPassword) < 8 {
		errs["newPassword"] = "must be at least 8 characters"
	}
	if strings.TrimSpace(req.ConfirmPassword) == "" {
		errs["confirmPassword"] = "required"
	} else if req.NewPassword != req.ConfirmPassword {
		errs["confirmPassword"] = "passwords do not match"
	}
	if len(errs) > 0 {
		httpx.WriteValidationError(w, errs)
		return
	}

	c := MustClaims(r.Context())
	u, err := h.repo.FindByID(r.Context(), c.UserID)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "session no longer valid")
		return
	}
	if !VerifyPassword(u.PasswordHash, req.CurrentPassword) {
		httpx.WriteValidationError(w, map[string]string{"currentPassword": "incorrect current password"})
		return
	}
	if req.CurrentPassword == req.NewPassword {
		httpx.WriteValidationError(w, map[string]string{"newPassword": "must be different from current password"})
		return
	}

	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := h.repo.UpdatePasswordHash(r.Context(), c.UserID, hash, &c.UserID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "session no longer valid")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	// Re-issue the session cookie so MustResetPassword=false (and any
	// current permissions) take effect immediately — see
	// issueSessionCookie's doc comment for why this matters for the forced-
	// reset flow specifically.
	u.MustResetPassword = false
	u.PasswordHash = hash
	if err := h.issueSessionCookie(w, r, u); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	httpx.NoContent(w)
}

type forgotPasswordReq struct {
	Email string `json:"email"`
}

// forgotPassword godoc
//
//	@Summary		Request a password reset email
//	@Description	Checks the email against the user list first — returns 404 if there's no active account for it, and only then sends the reset link. By design this reveals whether an email has an account (an accepted trade-off for this internal admin tool). Rate limited to 10 requests/minute per IP.
//	@Tags			Auth
//	@Accept			json
//	@Param			body	body	forgotPasswordReq	true	"Email"
//	@Success		204		"no content"
//	@Failure		404		{object}	httpx.ErrorResponse	"no account found for that email"
//	@Router			/api/v1/auth/forgot-password [post]
func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		httpx.WriteValidationError(w, map[string]string{"email": "required"})
		return
	}

	u, err := h.repo.FindByEmail(r.Context(), email)
	if err != nil || !u.Active {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "no account found for that email")
		return
	}

	if h.mailer != nil && h.mailer.Enabled() {
		token, err := h.repo.CreatePasswordResetToken(r.Context(), u.ID, time.Hour)
		if err != nil {
			slog.Error("forgot password: failed to create reset token", "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		link := fmt.Sprintf("%s/reset-password?token=%s", h.frontendURL, token)

		var studioName string
		if u.StudioID != nil {
			if name, err := h.repo.GetStudioName(r.Context(), *u.StudioID); err == nil {
				studioName = name
			}
		}

		if err := h.mailer.SendPasswordReset(u.Email, fullName(u.FirstName, u.LastName), studioName, link); err != nil {
			slog.Error("forgot password: failed to send email", "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "email_failed", "could not send the reset email")
			return
		}
	}

	httpx.NoContent(w)
}

type resetPasswordReq struct {
	Token           string `json:"token"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

// resetPassword godoc
//
//	@Summary		Consume a password reset token
//	@Description	Sets a new password using a single-use token from the forgot-password email. The token is invalidated whether or not this call succeeds past that point — it cannot be reused. Rate limited to 10 requests/minute per IP.
//	@Tags			Auth
//	@Accept			json
//	@Param			body	body	resetPasswordReq	true	"Token and new password"
//	@Success		204		"no content"
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid or expired token"
//	@Failure		422		{object}	httpx.ErrorResponse	"validation failed"
//	@Router			/api/v1/auth/reset-password [post]
func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	errs := map[string]string{}
	if strings.TrimSpace(req.Token) == "" {
		errs["token"] = "required"
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		errs["newPassword"] = "required"
	} else if len(req.NewPassword) < 8 {
		errs["newPassword"] = "must be at least 8 characters"
	}
	if req.NewPassword != req.ConfirmPassword {
		errs["confirmPassword"] = "passwords do not match"
	}
	if len(errs) > 0 {
		httpx.WriteValidationError(w, errs)
		return
	}

	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := h.repo.ConsumePasswordResetToken(r.Context(), req.Token, hash); err != nil {
		if errors.Is(err, ErrResetTokenInvalid) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_token", "this reset link is invalid or has expired")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	httpx.NoContent(w)
}

// ----- middleware / context -----

type ctxKey int

const claimsKey ctxKey = iota

func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

func ClaimsFrom(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*Claims)
	return c, ok
}

func MustClaims(ctx context.Context) *Claims {
	c, ok := ClaimsFrom(ctx)
	if !ok {
		panic("identity: claims missing from context (RequireAuth not applied?)")
	}
	return c
}

// RequireAuth verifies the session cookie, rejects it if the token's own
// sliding idle deadline (Claims.IdleExpiresAt) has passed or if its jti has
// been revoked (identity.SessionStore), and otherwise rotates the session:
// mints a brand-new token — same absolute expiry, a freshly pushed-forward
// idle deadline, a new jti — and sets it as the response's session cookie.
// This is what bakes the idle timeout into the token itself rather than
// relying solely on a Redis TTL: even without Redis, the token would
// eventually stop working on its own past IdleExpiresAt. Redis is still
// consulted (and still fails closed on an outage) purely for revocation,
// since a stateless JWT can't otherwise be killed before its own expiry.
//
// Deliberately does NOT revoke the old jti here — only logout and
// password-change do that (real, explicit security events). A browser
// page load routinely fires more than one authenticated request
// concurrently (parallel data fetching, Next.js prefetch, retries), all
// carrying whatever cookie the browser had at that moment; if rotation
// revoked the old jti immediately, whichever of those concurrent requests
// reached the server first would invalidate the cookie the others are
// still using, 401-ing them even though nothing was actually wrong. The
// old jti is simply left to expire on its own Redis TTL instead, so any
// request already in flight with it still succeeds.
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(h.cookie.Name)
		if err != nil || cookie.Value == "" {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		claims, err := h.tokens.Parse(cookie.Value)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired session")
			return
		}
		if claims.IdleExpiresAt != nil && time.Now().After(claims.IdleExpiresAt.Time) {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "session expired due to inactivity")
			return
		}
		if err := h.sessions.Exists(r.Context(), claims.ID); err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "session expired due to inactivity")
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
			return
		}

		u := &User{ID: claims.UserID, StudioID: claims.StudioID, Role: claims.Role, MustResetPassword: claims.MustResetPassword}
		newToken, newJTI, err := h.tokens.Refresh(u, claims.Permissions, claims.ExpiresAt.Time)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		if err := h.sessions.Create(r.Context(), newJTI, u.ID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     h.cookie.Name,
			Value:    newToken,
			Path:     "/",
			Domain:   h.cookie.Domain,
			Expires:  claims.ExpiresAt.Time,
			HttpOnly: true,
			Secure:   h.cookie.Secure,
			SameSite: http.SameSiteStrictMode,
		})

		ctx := WithClaims(r.Context(), claims)
		ctx = logger.WithUserID(ctx, claims.UserID.String())
		if claims.StudioID != nil {
			ctx = logger.WithTenantID(ctx, claims.StudioID.String())
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole gates handlers to one of the listed roles.
func RequireRole(allowed ...Role) func(http.Handler) http.Handler {
	set := make(map[Role]struct{}, len(allowed))
	for _, r := range allowed {
		set[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := ClaimsFrom(r.Context())
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}
			if _, allowed := set[c.Role]; !allowed {
				httpx.WriteError(w, http.StatusForbidden, "forbidden", "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// listStudioUsers godoc
//
//	@Summary		List users in a studio
//	@Description	Returns the id, email, and role of every user belonging to the given studio. Callers must be a super user or belong to the studio themselves.
//	@Tags			Auth
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID (UUID)"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"missing or invalid studioId"
//	@Failure		403			{object}	httpx.ErrorResponse	"cannot access this studio"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/users [get]
func (h *Handler) listStudioUsers(w http.ResponseWriter, r *http.Request) {
	studioIDStr := chi.URLParam(r, "studioId")
	if studioIDStr == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "studioId parameter required")
		return
	}
	studioID, err := uuid.Parse(studioIDStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid studioId format")
		return
	}

	c := MustClaims(r.Context())
	if !c.IsSuper() && (c.StudioID == nil || *c.StudioID != studioID) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "cannot access this studio")
		return
	}

	users, err := h.repo.ListStudioUsers(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to list users")
		return
	}

	res := make([]studioUserRes, len(users))
	for i, u := range users {
		res[i] = studioUserRes{
			ID: u.ID, Email: u.Email, Role: u.Role, Username: u.Username,
			FirstName: u.FirstName, LastName: u.LastName, RoleID: u.RoleID, RoleName: u.RoleName,
			MustResetPassword: u.MustResetPassword, Active: u.Active, CreatedAt: u.CreatedAt,
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"users": res})
}
