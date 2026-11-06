package identity

import (
	"context"
	"crypto/subtle"
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
	"github.com/projectx/api/internal/platform/riskalert"
	"github.com/projectx/api/internal/platform/secaudit"
	"github.com/projectx/api/internal/platform/secrets"
)

type Handler struct {
	repo        *Repo
	tokens      *TokenIssuer
	cookie      config.CookieConfig
	studioBrand StudioBrandLookup
	sessions    *SessionStore
	mailer      *mail.Sender
	frontendURL string
	// signingKeys is optional — nil means adminEndAllSessions skips the
	// signing-key rotation layer (task 1.9) and relies solely on
	// RevokeAllSessions (task 1.6), which is still correct on its own.
	signingKeys *SigningKeyStore
	// cipher encrypts/decrypts TOTP secrets at rest (task 2.1/2.2) — the
	// same at-rest-secrets cipher used everywhere else in this app (see
	// internal/platform/secrets). identity.Repo itself holds no cipher
	// (unlike studios.Repo), so this lives on Handler and the HTTP layer
	// encrypts/decrypts before the ciphertext ever reaches the repo.
	cipher *secrets.Cipher
	// pendingAuth holds the brief state between login's password step
	// and code step for an enrolled user (task 2.3). Required once any
	// user is TOTP-enrolled; nil is only safe if nobody ever enrolls.
	pendingAuth *PendingAuthStore
	// totpLockout tracks wrong-code attempts for the code-step lockout
	// (task 2.4). nil disables lockout entirely — the code step still
	// works, just without a brute-force backstop.
	totpLockout *TwoFALockoutStore
	// secAudit is platform-security-hardening task 3.3's event log. nil
	// is safe (secaudit.Logger.Log no-ops with an error that callers
	// log-and-ignore, same nil-safety convention as
	// internal/platform/audit.Logger) but means security events go
	// unrecorded, so main.go always wires a real one.
	secAudit *secaudit.Logger
	// platformAlertEmail is where task 3.7's risk alerts go. Empty
	// disables alerting entirely (sendRiskAlert no-ops), same degrade-
	// safely convention as mailer.Enabled().
	platformAlertEmail string
	// breachChecker backs task 4.2's breached-password check. nil skips
	// the check entirely (fail-open — see checkPasswordPolicy's doc
	// comment for why a down breach API must never block a legitimate
	// password change).
	breachChecker BreachChecker
	// loginLockout backs task 4.3's per-account login lockout. nil
	// disables lockout (login still works, no brute-force backstop),
	// same nil-is-safe convention as totpLockout.
	loginLockout *LoginLockoutStore
	// captcha backs task 8.3's bot protection, beyond the existing
	// per-IP rate limiter. nil (the default — see NewTurnstileVerifier's
	// doc comment on why no real site key exists in this sandbox) skips
	// the check entirely; login behaves exactly as before.
	captcha CaptchaVerifier
}

// WithCaptchaVerifier wires task 8.3's bot-protection check into h.
func (h *Handler) WithCaptchaVerifier(v CaptchaVerifier) *Handler {
	h.captcha = v
	return h
}

// WithLoginLockoutStore wires task 4.3's per-account login lockout into h.
func (h *Handler) WithLoginLockoutStore(s *LoginLockoutStore) *Handler {
	h.loginLockout = s
	return h
}

// WithBreachChecker wires task 4.2's breached-password check into h.
func (h *Handler) WithBreachChecker(c BreachChecker) *Handler {
	h.breachChecker = c
	return h
}

// checkPasswordPolicy enforces task 4.1 (12-char minimum, no complexity
// rule) and task 4.2 (breached-password rejection) for a candidate new
// password. Returns a validation-error field map (empty if the password
// passes) to match httpx.WriteValidationError's shape at call sites.
//
// The breach check fails OPEN: if h.breachChecker is nil, or the real
// HIBP call errors (network issue, HIBP itself down), the password is
// NOT rejected — a third-party API outage must never lock a user out of
// changing/resetting their own password. This mirrors this codebase's
// existing "best-effort, never block the primary action" convention for
// non-critical side effects (see internal/platform/mail's package doc
// comment).
func (h *Handler) checkPasswordPolicy(ctx context.Context, password string) map[string]string {
	errs := map[string]string{}
	if len(password) < MinPasswordLength {
		errs["newPassword"] = fmt.Sprintf("must be at least %d characters", MinPasswordLength)
		return errs
	}
	if h.breachChecker != nil {
		breached, err := h.breachChecker.IsBreached(ctx, password)
		if err != nil {
			slog.Warn("breach check failed, failing open", "err", err)
		} else if breached {
			errs["newPassword"] = "this password has appeared in a known data breach — please choose a different one"
		}
	}
	return errs
}

// WithPlatformAlertEmail wires the recipient for task 3.7's risk
// alerts.
func (h *Handler) WithPlatformAlertEmail(email string) *Handler {
	h.platformAlertEmail = email
	return h
}

// sendRiskAlert is task 3.7's trigger-to-email glue — fires the
// unconditional-by-design triggers (riskalert.TwoFADisabled,
// riskalert.SecretChanged, riskalert.DataExported all always return
// true, so callers just call this directly rather than checking a
// condition first) for the live-wireable subset. No-ops if either the
// mailer or the alert recipient isn't configured, same as every other
// best-effort email in this package.
func (h *Handler) sendRiskAlert(title, detail string) {
	if h.platformAlertEmail == "" || h.mailer == nil || !h.mailer.Enabled() {
		return
	}
	if err := h.mailer.SendSecurityAlert(h.platformAlertEmail, title, detail); err != nil {
		slog.Error("send risk alert failed", "err", err, "title", title)
	}
}

// WithSecAudit wires the security-audit-log writer into h (task 3.3).
func (h *Handler) WithSecAudit(l *secaudit.Logger) *Handler {
	h.secAudit = l
	return h
}

// logSecurityEvent is a thin, nil-safe wrapper so call sites don't each
// need to check h.secAudit != nil. actorRole may be empty (e.g. a failed
// login before we know the account's role). r may be nil for a
// background/non-HTTP caller, in which case IP/UA are left blank. Errors
// are logged by secaudit.Logger.Log itself and deliberately swallowed
// here — a failed audit write must never abort the request that
// triggered it, except for the same-transaction subset in task 3.4,
// which calls secaudit.LogTx directly instead of this helper.
func (h *Handler) logSecurityEvent(ctx context.Context, r *http.Request, actorUserID *uuid.UUID, actorRole string, studioID *uuid.UUID, action, outcome string, metadata map[string]any) {
	if h.secAudit == nil {
		return
	}
	var ip, ua string
	if r != nil {
		ip, ua = r.RemoteAddr, r.UserAgent()
	}
	_ = h.secAudit.Log(ctx, secaudit.Entry{
		ActorUserID: actorUserID,
		ActorRole:   actorRole,
		StudioID:    studioID,
		Action:      action,
		Metadata:    metadata,
		IP:          ip,
		UserAgent:   ua,
		Outcome:     outcome,
	})
}

// WithPendingAuthStore wires the login code-step's pending-token store
// into h (task 2.3). Required for login to work once any user has TOTP
// enrolled.
func (h *Handler) WithPendingAuthStore(s *PendingAuthStore) *Handler {
	h.pendingAuth = s
	return h
}

// WithTwoFALockoutStore wires the code-step lockout counter into h
// (task 2.4).
func (h *Handler) WithTwoFALockoutStore(s *TwoFALockoutStore) *Handler {
	h.totpLockout = s
	return h
}

// WithCipher wires the at-rest-secrets cipher into h, enabling the TOTP
// enrollment/verification endpoints (task 2.2+). Required for those
// endpoints to work; every other Handler method is unaffected if this
// is never called.
func (h *Handler) WithCipher(cipher *secrets.Cipher) *Handler {
	h.cipher = cipher
	return h
}

// WithSigningKeyStore wires the signing-key rotation store into h, so
// the platform-wide "end all sessions" endpoint also rotates+purges the
// JWT signing key (task 1.9). Mirrors TokenIssuer.WithSigningKeyStore —
// call with the same store passed there.
func (h *Handler) WithSigningKeyStore(keys *SigningKeyStore) *Handler {
	h.signingKeys = keys
	return h
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
	r.With(httpx.AuthRateLimiter).Post("/auth/login/2fa", h.verifyLoginTOTP)
	r.With(httpx.AuthRateLimiter).Post("/auth/login/recovery-code", h.verifyLoginRecoveryCode)
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
	r.With(h.RequireAuth).Get("/auth/sessions", h.listMySessions)
	r.With(h.RequireAuth).Delete("/auth/sessions/{sessionId}", h.endMySession)
	r.With(h.RequireAuth).Post("/auth/2fa/setup", h.setupTOTP)
	r.With(h.RequireAuth).Post("/auth/2fa/confirm", h.confirmTOTP)
	r.With(h.RequireAuth, httpx.AuthRateLimiter).Post("/auth/2fa/stepup", h.stepUp)
	r.With(h.RequireAuth).Post("/auth/sessions/end-others", h.endOtherMySessions)
}

func (h *Handler) StudioRoutes(r chi.Router) {
	r.Get("/users", h.listStudioUsers)
	r.Get("/security-audit-log", h.listStudioSecurityAuditLog)
}

// listStudioSecurityAuditLog godoc
//
//	@Summary		Security audit log for this studio (studio_admin)
//	@Description	The studio_admin read-access path from task 3.6 — scoped to the caller's own studio only. studio_staff has no route to this handler at all (not mounted under their permission set), matching the spec's "staff sees none".
//	@Tags			Auth
//	@Security		CookieAuth
//	@Param			studioId	path	string	true	"Studio ID (UUID)"
//	@Success		200	{array}	secaudit.LogEntry
//	@Router			/api/v1/studios/{studioId}/security-audit-log [get]
func (h *Handler) listStudioSecurityAuditLog(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid studioId")
		return
	}
	entries, err := secaudit.ListForStudio(r.Context(), h.repo.pool, studioID, 100)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, entries)
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	// CaptchaToken is only checked when h.captcha is configured (task
	// 8.3) — nil in this sandbox (see CaptchaVerifier's doc comment), so
	// this field is currently always ignored in practice.
	CaptchaToken string `json:"captchaToken,omitempty"`
}

// twoFARequiredRes is returned by login instead of meRes when the
// password was correct but the account has TOTP enrolled — no session
// cookie is set yet (platform-security-hardening task 2.3). pendingToken
// must be presented to verifyLoginTOTP within 5 minutes.
type twoFARequiredRes struct {
	TwoFactorRequired bool   `json:"twoFactorRequired"`
	PendingToken      string `json:"pendingToken"`
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
//
// twoFAPassed is session metadata (does this session not need a code
// step at all, including via remember-browser) stored on the `sessions`
// row. twoFAVerifiedAt is the separate, stricter signal RequireStepUp
// checks (task 2.6): non-nil only when a code was actually, freshly
// checked in this very call — nil for a remember-browser login, even
// though that login's twoFAPassed is true.
func (h *Handler) issueSessionCookie(w http.ResponseWriter, r *http.Request, u *User, twoFAPassed bool, twoFAVerifiedAt *time.Time) error {
	var permissions []string
	if u.Role == RoleStudioStaff && u.RoleID != nil {
		var err error
		permissions, err = h.repo.GetRolePermissionKeys(r.Context(), *u.RoleID)
		if err != nil {
			return err
		}
	}

	token, exp, jti, csrfToken, sessionID, err := h.tokens.Issue(r.Context(), u, permissions, twoFAVerifiedAt)
	if err != nil {
		return err
	}
	if err := h.sessions.Create(r.Context(), jti, u.ID); err != nil {
		return err
	}
	if err := h.repo.CreateSession(r.Context(), sessionID, u.ID, "", r.Header.Get("User-Agent"), httpx.ClientIP(r), "", twoFAPassed); err != nil {
		return err
	}
	// If this call is replacing an already-authenticated session (e.g.
	// changePassword re-issuing after a password change), revoke the old
	// jti so the previous cookie stops working the instant this one is
	// set, instead of both being valid until the old one idle-times-out.
	if old, ok := ClaimsFrom(r.Context()); ok && old.ID != jti {
		_ = h.sessions.Revoke(r.Context(), old.ID)
	}

	// #nosec G124 -- Secure/SameSite are set via h.cookie.Secure/SameSiteStrictMode below; config.Load() refuses to start outside API_ENV=local unless COOKIE_SECURE=true (config.go), so this is not a static true/false gosec can see but is enforced at boot.
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
	setCSRFCookie(w, h.cookie, csrfToken, exp)
	return nil
}

// csrfCookieName is deliberately NOT HttpOnly — client-side JS (lib/api.ts)
// and Next.js Server Actions (reading it via next/headers' cookies()) both
// need to read its value to echo it back as the X-CSRF-Token header.
// SameSite=Strict still applies, same as the session cookie — it's not the
// thing keeping this safe to expose (the whole point of the pattern is that
// an attacker's cross-site page can trigger the cookie to be auto-sent but
// can never read its value to forge a matching header), but there's no
// reason to loosen it.
const csrfCookieName = "px_csrf"

func setCSRFCookie(w http.ResponseWriter, cookie config.CookieConfig, csrfToken string, exp time.Time) {
	// #nosec G124 -- Secure/SameSite are set via h.cookie.Secure/SameSiteStrictMode below; config.Load() refuses to start outside API_ENV=local unless COOKIE_SECURE=true (config.go), so this is not a static true/false gosec can see but is enforced at boot.
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/",
		Domain:   cookie.Domain,
		Expires:  exp,
		HttpOnly: false,
		Secure:   cookie.Secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// checkCSRF compares the X-CSRF-Token request header against the session's
// own CSRFToken claim — constant-time, so response timing can't leak
// whether/how much of the token matched. Safe methods (GET/HEAD/OPTIONS)
// never mutate state, so they're exempt by convention, same as every other
// CSRF scheme. Called from RequireAuth (covers every authenticated mutating
// route in one place) and explicitly from logout (the one mutating auth
// route that doesn't go through RequireAuth — see its own call site).
func checkCSRF(r *http.Request, csrfToken string) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	header := r.Header.Get("X-CSRF-Token")
	if header == "" || csrfToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(header), []byte(csrfToken)) == 1
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

	// Bot protection (task 8.3), beyond the existing per-IP rate limiter
	// (httpx.AuthRateLimiter already wraps this route). Only enforced
	// once a real verifier is configured — see WithCaptchaVerifier's and
	// CaptchaVerifier's doc comments for why none is wired in this
	// sandbox.
	if h.captcha != nil {
		ok, err := h.captcha.Verify(r.Context(), req.CaptchaToken, httpx.ClientIP(r))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		if !ok {
			httpx.WriteError(w, http.StatusForbidden, "captcha_required", "bot verification failed")
			return
		}
	}

	// Per-account lockout (task 4.3) — checked before touching the DB at
	// all, independent of which IP the request came from, unlike the
	// existing per-request-IP rate limiter.
	if h.loginLockout != nil {
		locked, err := h.loginLockout.IsLocked(r.Context(), req.Email)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		if locked {
			httpx.WriteError(w, http.StatusTooManyRequests, "account_locked", "too many failed attempts — try again later")
			return
		}
	}

	u, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.logSecurityEvent(r.Context(), r, nil, "", nil, "login", secaudit.OutcomeFailure, map[string]any{"email": req.Email, "reason": "not_found"})
			if h.loginLockout != nil {
				_, _ = h.loginLockout.RecordFailure(r.Context(), req.Email)
			}
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if !VerifyPassword(u.PasswordHash, req.Password) {
		h.logSecurityEvent(r.Context(), r, &u.ID, string(u.Role), u.StudioID, "login", secaudit.OutcomeFailure, map[string]any{"reason": "bad_password"})
		if h.loginLockout != nil {
			if justLocked, lockErr := h.loginLockout.RecordFailure(r.Context(), req.Email); lockErr == nil && justLocked && riskalert.FailedLoginThreshold(loginLockoutThreshold) {
				h.sendRiskAlert("5+ failed logins", fmt.Sprintf("Account %s had 5 failed login attempts in a row and is now locked for %s.", req.Email, loginLockoutWindow))
			}
		}
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}
	if h.loginLockout != nil {
		_ = h.loginLockout.Reset(r.Context(), req.Email)
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

	// Password step passed — if TOTP is enrolled, a full session isn't
	// issued yet; the caller must still complete the code step
	// (platform-security-hardening task 2.3) — UNLESS this exact
	// browser was remembered (task 2.5), and never for super_admin,
	// regardless of any cookie presented.
	totpEnabled, err := h.repo.IsTOTPEnabled(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if totpEnabled && u.Role != RoleSuperAdmin {
		if cookie, err := r.Cookie(rememberBrowserCookieName); err == nil {
			remembered, err := h.repo.IsRememberBrowserTokenValid(r.Context(), u.ID, cookie.Value)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
				return
			}
			if remembered {
				// twoFAVerifiedAt stays nil — a remembered browser skips
				// the code step entirely, so no fresh check happened
				// (task 2.6's RequireStepUp must still prompt for one).
				h.completeLogin(w, r, u, true, nil)
				return
			}
		}
	}
	if totpEnabled {
		pendingToken, err := h.pendingAuth.Create(r.Context(), u.ID)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		httpx.JSON(w, http.StatusOK, twoFARequiredRes{TwoFactorRequired: true, PendingToken: pendingToken})
		return
	}

	h.completeLogin(w, r, u, false, nil)
}

// completeLogin issues the real session cookie and does the post-login
// side effects (new-device email) shared by both the no-2FA path in
// login and the code-step path in verifyLoginTOTP (task 2.3).
// twoFAVerifiedAt should be &now if the caller just checked a real code
// (verifyLoginTOTP/verifyLoginRecoveryCode) or nil otherwise (no 2FA
// enrolled, or a remember-browser login) — see issueSessionCookie's doc
// comment for why this is tracked separately from twoFAPassed.
func (h *Handler) completeLogin(w http.ResponseWriter, r *http.Request, u *User, twoFAPassed bool, twoFAVerifiedAt *time.Time) {
	// Checked BEFORE issueSessionCookie's CreateSession call records this
	// very login — otherwise every login would trivially "recognize"
	// itself (platform-security-hardening task 1.7).
	browser := r.Header.Get("User-Agent")
	seenBefore, err := h.repo.HasPriorSessionFromDevice(r.Context(), u.ID, browser)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	if err := h.issueSessionCookie(w, r, u, twoFAPassed, twoFAVerifiedAt); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	h.logSecurityEvent(r.Context(), r, &u.ID, string(u.Role), u.StudioID, "login", secaudit.OutcomeSuccess, map[string]any{"newDevice": !seenBefore, "twoFaPassed": twoFAPassed})

	if !seenBefore && h.mailer != nil && h.mailer.Enabled() {
		// Best-effort: a failed send must never block a successful login.
		name := ""
		if u.Username != nil {
			name = *u.Username
		}
		if err := h.mailer.SendNewDeviceLogin(u.Email, name, browser, httpx.ClientIP(r), time.Now().UTC().Format(time.RFC1123)); err != nil {
			slog.Error("send new-device login email failed", "err", err, "user_id", u.ID)
		}
	}

	httpx.JSON(w, http.StatusOK, h.buildMeRes(r.Context(), u))
}

type verifyLoginTOTPReq struct {
	PendingToken string `json:"pendingToken"`
	Code         string `json:"code"`
	// RememberBrowser requests a 14-day skip-the-code-step grant for
	// this browser (task 2.5) — ignored (never granted) for super_admin.
	RememberBrowser bool `json:"rememberBrowser,omitempty"`
}

// maybeSetRememberBrowserCookie grants a 14-day remember-browser cookie
// when requested and the role is eligible (never super_admin) —
// task 2.5. Best-effort: a failure to mint the token just means the
// next login asks for a code again, not a failed login right now.
func (h *Handler) maybeSetRememberBrowserCookie(w http.ResponseWriter, r *http.Request, u *User, requested bool) {
	if !requested || u.Role == RoleSuperAdmin {
		return
	}
	token, err := h.repo.CreateRememberBrowserToken(r.Context(), u.ID)
	if err != nil {
		slog.Error("create remember-browser token failed", "err", err, "user_id", u.ID)
		return
	}
	// #nosec G124 -- Secure/SameSite are set via h.cookie.Secure/SameSiteStrictMode below; config.Load() refuses to start outside API_ENV=local unless COOKIE_SECURE=true (config.go), so this is not a static true/false gosec can see but is enforced at boot.
	http.SetCookie(w, &http.Cookie{
		Name:     rememberBrowserCookieName,
		Value:    token,
		Path:     "/",
		Domain:   h.cookie.Domain,
		Expires:  time.Now().Add(rememberBrowserTTL),
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// verifyLoginTOTP godoc
//
//	@Summary		Complete login's code step
//	@Description	Exchanges a pendingToken from login (password step) plus a TOTP code for a real session. The pending token is single-use and expires after 5 minutes.
//	@Tags			Auth
//	@Accept			json
//	@Param			body	body	verifyLoginTOTPReq	true	"Pending token and 6-digit code"
//	@Success		200		{object}	meRes
//	@Failure		401		{object}	httpx.ErrorResponse	"pending token expired or already used"
//	@Failure		422		{object}	httpx.ErrorResponse	"wrong code"
//	@Router			/api/v1/auth/login/2fa [post]
func (h *Handler) verifyLoginTOTP(w http.ResponseWriter, r *http.Request) {
	var req verifyLoginTOTPReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	// Resolve, not Consume: a wrong code must leave the pending token
	// alive so the user can retry within the same 5-minute window
	// (task 2.3) — only a CORRECT code consumes it, just below.
	userID, err := h.pendingAuth.Resolve(r.Context(), req.PendingToken)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "sign in again")
		return
	}
	u, err := h.repo.FindByID(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "sign in again")
		return
	}

	// Locked accounts are rejected before even checking the code
	// (task 2.4) — checking first would let a locked-out attacker keep
	// learning whether a given code happens to be right.
	if h.totpLockout != nil {
		locked, err := h.totpLockout.IsLocked(r.Context(), userID)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		if locked {
			httpx.WriteError(w, http.StatusLocked, "locked", "too many wrong codes — try again in 15 minutes")
			return
		}
	}

	ok, err := h.verifyEnabledTOTPCode(r.Context(), userID, req.Code)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if !ok {
		if h.totpLockout != nil {
			justLocked, lockErr := h.totpLockout.RecordFailure(r.Context(), userID)
			if lockErr != nil {
				slog.Error("record totp failure failed", "err", lockErr, "user_id", userID)
			} else if justLocked && h.mailer != nil && h.mailer.Enabled() {
				name := ""
				if u.Username != nil {
					name = *u.Username
				}
				if err := h.mailer.SendTwoFactorLockout(u.Email, name); err != nil {
					slog.Error("send 2fa lockout email failed", "err", err, "user_id", userID)
				}
			}
		}
		httpx.WriteValidationError(w, map[string]string{"code": "invalid code"})
		return
	}
	if h.totpLockout != nil {
		if err := h.totpLockout.Reset(r.Context(), userID); err != nil {
			slog.Error("reset totp lockout failed", "err", err, "user_id", userID)
		}
	}
	if _, err := h.pendingAuth.Consume(r.Context(), req.PendingToken); err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "sign in again")
		return
	}

	h.maybeSetRememberBrowserCookie(w, r, u, req.RememberBrowser)
	now := time.Now()
	h.completeLogin(w, r, u, true, &now)
}

type verifyLoginRecoveryCodeReq struct {
	PendingToken    string `json:"pendingToken"`
	Code            string `json:"code"`
	RememberBrowser bool   `json:"rememberBrowser,omitempty"`
}

// verifyLoginRecoveryCode godoc
//
//	@Summary		Complete login with a recovery code
//	@Description	Lost-phone path: exchanges a pendingToken from login (password step) plus an unused recovery code for a real session. The code is marked used and can never be presented again.
//	@Tags			Auth
//	@Accept			json
//	@Param			body	body	verifyLoginRecoveryCodeReq	true	"Pending token and recovery code"
//	@Success		200		{object}	meRes
//	@Failure		401		{object}	httpx.ErrorResponse	"pending token expired or already used"
//	@Failure		422		{object}	httpx.ErrorResponse	"wrong or already-used code"
//	@Router			/api/v1/auth/login/recovery-code [post]
func (h *Handler) verifyLoginRecoveryCode(w http.ResponseWriter, r *http.Request) {
	var req verifyLoginRecoveryCodeReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	// Resolve, not Consume — same reasoning as verifyLoginTOTP: a wrong
	// code must leave the pending token alive to retry.
	userID, err := h.pendingAuth.Resolve(r.Context(), req.PendingToken)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "sign in again")
		return
	}
	u, err := h.repo.FindByID(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "sign in again")
		return
	}

	ok, err := h.repo.ConsumeRecoveryCode(r.Context(), userID, normalizeRecoveryCode(req.Code))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if !ok {
		httpx.WriteValidationError(w, map[string]string{"code": "invalid or already-used recovery code"})
		return
	}
	if _, err := h.pendingAuth.Consume(r.Context(), req.PendingToken); err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "sign in again")
		return
	}

	// A recovery code is itself proof of possessing the second factor
	// (it was only ever handed out once, at enrollment) — it satisfies
	// the code step the same way a TOTP code does.
	h.maybeSetRememberBrowserCookie(w, r, u, req.RememberBrowser)
	now := time.Now()
	h.completeLogin(w, r, u, true, &now)
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
	// logout doesn't go through RequireAuth (see Routes — it's intentionally
	// reachable without a valid session, since a stale/broken cookie should
	// still be clearable), so it checks CSRF inline instead: only enforced
	// when there's an actual valid session to protect — a request with no
	// (or an already-broken) cookie has nothing a forged request could
	// meaningfully hijack, so it still falls through to just clearing
	// whatever cookie is present.
	if cookie, err := r.Cookie(h.cookie.Name); err == nil && cookie.Value != "" {
		if claims, err := h.tokens.Parse(r.Context(), cookie.Value); err == nil {
			if !checkCSRF(r, claims.CSRFToken) {
				httpx.WriteError(w, http.StatusForbidden, "csrf_mismatch", "missing or invalid CSRF token")
				return
			}
			_ = h.sessions.Revoke(r.Context(), claims.ID)
			h.logSecurityEvent(r.Context(), r, &claims.UserID, string(claims.Role), nil, "logout", secaudit.OutcomeSuccess, nil)
		}
	}

	// #nosec G124 -- Secure/SameSite are set via h.cookie.Secure/SameSiteStrictMode below; config.Load() refuses to start outside API_ENV=local unless COOKIE_SECURE=true (config.go), so this is not a static true/false gosec can see but is enforced at boot.
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
			// #nosec G124 -- Secure/SameSite are set via h.cookie.Secure/SameSiteStrictMode below; config.Load() refuses to start outside API_ENV=local unless COOKIE_SECURE=true (config.go), so this is not a static true/false gosec can see but is enforced at boot.
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
	} else {
		for k, v := range h.checkPasswordPolicy(r.Context(), req.NewPassword) {
			errs[k] = v
		}
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

	// End every session this user has — on every device, not just this
	// one (platform-security-hardening task 1.4). Must run BEFORE
	// issueSessionCookie below, which creates a brand-new session for
	// this request/device; revoking after would immediately kill the one
	// we're about to issue.
	if err := h.RevokeSessionsForUser(r.Context(), c.UserID, "password changed"); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	h.logSecurityEvent(r.Context(), r, &c.UserID, string(c.Role), u.StudioID, "password.change", secaudit.OutcomeSuccess, nil)

	// Re-issue the session cookie so MustResetPassword=false (and any
	// current permissions) take effect immediately — see
	// issueSessionCookie's doc comment for why this matters for the forced-
	// reset flow specifically.
	u.MustResetPassword = false
	u.PasswordHash = hash
	// twoFAPassed=false here is a cosmetic simplification, not a security
	// gap: this re-issue only follows an already-authenticated request
	// (RequireAuth ran first), so whatever factor(s) got them here were
	// already enforced before this handler ran at all. TwoFAVerifiedAt
	// is carried forward from the existing session rather than cleared,
	// so a recent step-up isn't lost just because the password changed
	// moments later.
	var twoFAVerifiedAt *time.Time
	if old, ok := ClaimsFrom(r.Context()); ok && old.TwoFAVerifiedAt != nil {
		t := old.TwoFAVerifiedAt.Time
		twoFAVerifiedAt = &t
	}
	if err := h.issueSessionCookie(w, r, u, false, twoFAVerifiedAt); err != nil {
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
	} else {
		for k, v := range h.checkPasswordPolicy(r.Context(), req.NewPassword) {
			errs[k] = v
		}
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
	userID, err := h.repo.ConsumePasswordResetToken(r.Context(), req.Token, hash)
	if err != nil {
		if errors.Is(err, ErrResetTokenInvalid) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_token", "this reset link is invalid or has expired")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	// A reset via emailed link typically means the previous credential may
	// have been compromised — end every session, not just let the old one
	// linger until it idle-times-out (platform-security-hardening task 1.4).
	if err := h.RevokeSessionsForUser(r.Context(), userID, "password reset"); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	h.logSecurityEvent(r.Context(), r, &userID, "", nil, "password.reset", secaudit.OutcomeSuccess, nil)

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
// mints a brand-new token — both the absolute expiry and the idle deadline
// pushed forward from now, a new jti — and sets it as the response's
// session cookie. This is what bakes the idle timeout into the token itself
// rather than relying solely on a Redis TTL: even without Redis, the token
// would eventually stop working on its own past IdleExpiresAt. Redis is
// still consulted (and still fails closed on an outage) purely for
// revocation, since a stateless JWT can't otherwise be killed before its
// own expiry.
//
// Sliding the absolute expiry forward on every request (not just the idle
// deadline) is deliberate: as long as the user keeps making authenticated
// requests, the session should never hit a fixed login-time ceiling on a
// timer — only genuine inactivity (no request at all within the idle
// window), or Claims.SessionMaxExpiresAt (a hard backstop that sliding
// activity never extends — see its doc comment), should ever force a
// logout.
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
		claims, err := h.tokens.Parse(r.Context(), cookie.Value)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired session")
			return
		}
		if claims.IdleExpiresAt != nil && time.Now().After(claims.IdleExpiresAt.Time) {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "session expired due to inactivity")
			return
		}
		// SessionMaxExpiresAt (the hard backstop — see its doc comment) is
		// nil only for a token issued before this field existed; treat that
		// as already expired rather than letting it through unbounded.
		if claims.SessionMaxExpiresAt == nil || time.Now().After(claims.SessionMaxExpiresAt.Time) {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "session expired, please sign in again")
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
		// Revocation takes effect on the very next request, not just the
		// next login — see RevokeSessionsForUser's doc comment
		// (platform-security-hardening task 1.4). A zero-value SessionID
		// means this token predates the sessions feature; treat it the same
		// rollout-grace way CSRFToken is handled just below rather than
		// hard-failing every pre-existing session.
		if claims.SessionID != uuid.Nil {
			revoked, err := h.sessions.IsRevoked(r.Context(), claims.SessionID)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
				return
			}
			if revoked {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "session has been ended")
				return
			}
		}

		// Rollout grace: a session issued before CSRF protection existed has
		// no CSRFToken claim at all (empty string) — without this, every
		// admin already logged in at deploy time would get silently 403'd
		// on every mutating action for the rest of their session (hours),
		// with no path to recover except manually logging out and back in.
		// Minting one now instead self-heals within a single request: this
		// one request (if mutating) still can't present a header matching a
		// token it was never given, so it's let through once un-checked,
		// but the refreshed cookie below carries the new token forward, so
		// every subsequent request — including ones fired moments later —
		// has a real one to check. Only reachable for a pre-existing
		// session; Issue() always sets a real token from here on, so this
		// path goes permanently cold once every live session has rotated at
		// least once (bounded by SessionMaxExpiresAt).
		csrfToken := claims.CSRFToken
		if csrfToken == "" {
			csrfToken, err = generateCSRFToken()
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
				return
			}
		} else if !checkCSRF(r, csrfToken) {
			// CSRF check before any mutation happens — claims.CSRFToken is
			// stable across the Refresh below (carried forward unchanged),
			// so this check and the cookie the client already has from
			// login/last refresh always agree. See checkCSRF's doc comment
			// for why this is the one place that covers every authenticated
			// mutating route.
			httpx.WriteError(w, http.StatusForbidden, "csrf_mismatch", "missing or invalid CSRF token")
			return
		}

		u := &User{ID: claims.UserID, StudioID: claims.StudioID, Role: claims.Role, MustResetPassword: claims.MustResetPassword}
		var twoFAVerifiedAt *time.Time
		if claims.TwoFAVerifiedAt != nil {
			t := claims.TwoFAVerifiedAt.Time
			twoFAVerifiedAt = &t
		}
		newToken, newExp, newJTI, err := h.tokens.Refresh(r.Context(), u, claims.Permissions, claims.SessionMaxExpiresAt.Time, csrfToken, claims.SessionID, twoFAVerifiedAt)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		if err := h.sessions.Create(r.Context(), newJTI, u.ID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		// Advances last_seen_at for the self-service session list (task
		// 1.5). Best-effort: a pre-sessions-feature token (SessionID ==
		// uuid.Nil) has no row to touch, h.repo is nil in tests that only
		// exercise the Redis-backed half of RequireAuth, and a touch
		// failure shouldn't 500 an otherwise-successful request.
		if claims.SessionID != uuid.Nil && h.repo != nil {
			_ = h.repo.TouchSession(r.Context(), claims.SessionID)
		}
		// #nosec G124 -- Secure/SameSite are set via h.cookie.Secure/SameSiteStrictMode below; config.Load() refuses to start outside API_ENV=local unless COOKIE_SECURE=true (config.go), so this is not a static true/false gosec can see but is enforced at boot.
		http.SetCookie(w, &http.Cookie{
			Name:     h.cookie.Name,
			Value:    newToken,
			Path:     "/",
			Domain:   h.cookie.Domain,
			Expires:  newExp,
			HttpOnly: true,
			Secure:   h.cookie.Secure,
			SameSite: http.SameSiteStrictMode,
		})
		setCSRFCookie(w, h.cookie, csrfToken, newExp)

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
