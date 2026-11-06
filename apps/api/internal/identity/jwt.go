package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/secrets"
)

// numericDateOrNil converts a *time.Time to a *jwt.NumericDate,
// preserving nil — jwt.NewNumericDate(*t) would panic on a nil pointer.
func numericDateOrNil(t *time.Time) *jwt.NumericDate {
	if t == nil {
		return nil
	}
	return jwt.NewNumericDate(*t)
}

// generateCSRFToken returns a random, URL-safe 32-byte token — same
// strength as the session's own encryption key (see secrets.RequiredKeyBytes),
// plenty for an unguessable per-session CSRF value.
func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type Claims struct {
	UserID   uuid.UUID  `json:"uid"`
	StudioID *uuid.UUID `json:"sid,omitempty"` // omitted for super_admin
	Role     Role       `json:"role"`
	// Permissions is only meaningful for RoleStudioStaff — the nav-section
	// keys (identity/permissions) their role grants, as of the moment this
	// token was issued (login or password-change). Always omitted/empty for
	// super_admin/studio_admin, who bypass permission checks entirely
	// rather than being granted "all of them" — see BypassesPermissionChecks.
	//
	// By explicit request, this is trusted as-is by RequirePermission — NOT
	// re-checked against the DB on each request. Trade-off: a role's
	// permissions (and this user's own deactivation) only take effect on
	// this user's next login/password-change, not their next click. An
	// earlier version of this middleware did check the DB live on every
	// request (catching both of those instantly) but was reverted back to
	// this JWT-trusting design on request, accepting the staleness.
	Permissions []string `json:"perms,omitempty"`
	// MustResetPassword mirrors users.must_reset_password at the time this
	// token was issued — see RequirePasswordSet. It's refreshed whenever the
	// password changes (issueSessionCookie is called again then).
	MustResetPassword bool `json:"mrp,omitempty"`
	// IdleExpiresAt is this token's sliding idle deadline — the caller must
	// make another authenticated request before this time or the session
	// dies. RequireAuth pushes both this and RegisteredClaims.ExpiresAt
	// forward on every request by minting and returning a brand-new token
	// (see TokenIssuer.Refresh), so the idle timeout is enforced by the
	// token itself, not just by identity.SessionStore's Redis TTL.
	IdleExpiresAt *jwt.NumericDate `json:"idle_exp,omitempty"`
	// SessionMaxExpiresAt is a hard backstop set once at the original
	// login (TokenIssuer.Issue) and never moved by Refresh, unlike
	// IdleExpiresAt/ExpiresAt which both slide forward on every active
	// request. Even a continuously-active session is force-logged-out once
	// this passes — a security ceiling so a long-lived (or stolen) session
	// cookie can't stay usable forever, independent of how often it's used.
	SessionMaxExpiresAt *jwt.NumericDate `json:"sess_max_exp,omitempty"`
	// CSRFToken is a random value generated once at login (TokenIssuer.Issue)
	// and carried forward unchanged by every Refresh, same pattern as
	// SessionMaxExpiresAt — stable for the life of the session even though
	// the JWT itself (and its jti) rotates on every request. Mirrored into a
	// second, non-HttpOnly cookie (see issueSessionCookie) so client-side JS
	// and Next.js Server Actions can read it and echo it back as the
	// X-CSRF-Token header on every mutating request; RequireAuth compares
	// that header against this claim (which an attacker's cross-site page
	// can't read — it's sealed inside the encrypted px_session cookie, not
	// just protected by SameSite). See RequireAuth's checkCSRF call.
	CSRFToken string `json:"csrf"`
	// SessionID is the durable identity of this login session — generated
	// once at Issue and, like CSRFToken/SessionMaxExpiresAt, carried
	// forward unchanged by every Refresh even though the token's own jti
	// rotates on every request. It's what the sessions table (platform-
	// security-hardening task 1.1) is keyed on, and what
	// SessionStore.IsRevoked checks on every request — jti alone can't be
	// the revocation key here, since a stolen/old jti is simply left to
	// expire rather than revoked on rotation (see RequireAuth's doc
	// comment), which would make a per-jti revocation list useless for
	// "end this session right now" semantics.
	SessionID uuid.UUID `json:"session_id"`
	// TwoFAVerifiedAt is when a TOTP/recovery code was last actually
	// checked for this session — set only by a real code verification
	// (login's code step, or the dedicated step-up endpoint), never
	// slid forward by an ordinary Refresh the way IdleExpiresAt is.
	// RequireStepUp (platform-security-hardening task 2.6) requires this
	// to be within the last 10 minutes for sensitive actions. nil means
	// no code has ever been verified this session (no 2FA enrolled, or
	// a remember-browser login that skipped the code step entirely —
	// deliberately NOT treated as a fresh verification for step-up).
	TwoFAVerifiedAt *jwt.NumericDate `json:"totp_verified_at,omitempty"`
	jwt.RegisteredClaims
}

// IsSuper returns true if the claims belong to a super admin.
func (c *Claims) IsSuper() bool { return c.Role == RoleSuperAdmin }

// BypassesPermissionChecks returns true for the two roles that always have
// full access, regardless of Permissions — only RoleStudioStaff is ever
// restricted by the permission catalog.
func (c *Claims) BypassesPermissionChecks() bool {
	return c.Role == RoleSuperAdmin || c.Role == RoleStudioAdmin
}

// HasPermission reports whether these claims grant key. Always true for
// super_admin/studio_admin.
func (c *Claims) HasPermission(key string) bool {
	if c.BypassesPermissionChecks() {
		return true
	}
	for _, p := range c.Permissions {
		if p == key {
			return true
		}
	}
	return false
}

// EffectiveStudioID resolves the studio scope for a request. For studio_admins
// it returns their assigned studio_id. For super_admins it returns the
// `requested` value (typically a path/query param) so they can act on any
// studio. Returns false if a super admin didn't supply one when needed.
func (c *Claims) EffectiveStudioID(requested *uuid.UUID) (uuid.UUID, bool) {
	if c.IsSuper() {
		if requested == nil {
			return uuid.Nil, false
		}
		return *requested, true
	}
	if c.StudioID == nil {
		return uuid.Nil, false
	}
	return *c.StudioID, true
}

// TokenIssuer signs, encrypts, decrypts and verifies session JWTs.
//
// The token stored in the session cookie is not a plain JWS: after signing
// with HS256 (so the claims can't be forged without secret), the whole
// compact JWS string is AES-256-GCM-encrypted with cipher (the same
// at-rest-secrets cipher used for stored credentials — see the secrets
// package) before it ever leaves the server. This is a nested
// sign-then-encrypt envelope, not a JOSE/JWE token, but it gets the property
// that was asked for: the cookie value on the wire/in the browser can't be
// base64-decoded to read uid/role/perms the way a bare JWT can.
type TokenIssuer struct {
	// secret is the static fallback signing key, used only when keys is
	// nil — every test and any deployment that hasn't wired a
	// SigningKeyStore. It signs/verifies under the fixed kid envKeyID,
	// the same kid a SigningKeyStore bootstraps from the identical env
	// secret, so a token survives a later switch from static to rotatable
	// signing without needing to be reissued.
	secret  []byte
	keys    *SigningKeyStore
	ttl     time.Duration
	idleTTL time.Duration
	// maxAge is the hard backstop (see Claims.SessionMaxExpiresAt) — set
	// once at Issue time, never extended by Refresh.
	maxAge time.Duration
	// superAdminIdleTTL/superAdminMaxAge are the same two concepts, but
	// tighter — super_admin can reach every studio's data, so its session
	// gets a shorter leash (platform-security-hardening task 1.8). Every
	// other role uses idleTTL/maxAge above.
	superAdminIdleTTL time.Duration
	superAdminMaxAge  time.Duration
	cipher            *secrets.Cipher
}

// WithSigningKeyStore wires a SigningKeyStore into t, enabling
// out-of-process signing-key rotation (platform-security-hardening task
// 1.9). Call once at startup, right after NewTokenIssuer(WithSuperAdminTTLs).
func (t *TokenIssuer) WithSigningKeyStore(keys *SigningKeyStore) *TokenIssuer {
	t.keys = keys
	return t
}

// signingSecret returns the kid/secret newly-issued tokens should be
// signed with.
func (t *TokenIssuer) signingSecret(ctx context.Context) (kid string, secret []byte, err error) {
	if t.keys != nil {
		return t.keys.Current(ctx)
	}
	return envKeyID, t.secret, nil
}

// verifyingSecret returns the secret for kid, used to verify an existing
// token's signature. An empty kid (every token issued before this field
// existed) is treated as envKeyID — both the static-secret fallback and
// a SigningKeyStore bootstrap from the very same env-sourced secret
// under that kid, so old tokens keep verifying either way.
func (t *TokenIssuer) verifyingSecret(ctx context.Context, kid string) ([]byte, error) {
	if kid == "" {
		kid = envKeyID
	}
	if t.keys != nil {
		secret, ok, err := t.keys.Lookup(ctx, kid)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("unknown signing key id")
		}
		return secret, nil
	}
	if kid != envKeyID {
		return nil, errors.New("unknown signing key id")
	}
	return t.secret, nil
}

// ttlsFor returns the idle timeout and hard max-age that apply to role —
// tighter for super_admin than every other role (task 1.8).
func (t *TokenIssuer) ttlsFor(role Role) (idleTTL, maxAge time.Duration) {
	if role == RoleSuperAdmin {
		return t.superAdminIdleTTL, t.superAdminMaxAge
	}
	return t.idleTTL, t.maxAge
}

func NewTokenIssuer(secret string, ttl, idleTTL, maxAge time.Duration, cipher *secrets.Cipher) *TokenIssuer {
	return NewTokenIssuerWithSuperAdminTTLs(secret, ttl, idleTTL, maxAge, idleTTL, maxAge, cipher)
}

// NewTokenIssuerWithSuperAdminTTLs is NewTokenIssuer plus the tighter
// super_admin idle/max-age pair (task 1.8). NewTokenIssuer itself stays
// as the common-case constructor (used by every test that doesn't care
// about the super_admin/other-roles distinction) by delegating here with
// the super_admin values equal to the regular ones.
func NewTokenIssuerWithSuperAdminTTLs(secret string, ttl, idleTTL, maxAge, superAdminIdleTTL, superAdminMaxAge time.Duration, cipher *secrets.Cipher) *TokenIssuer {
	return &TokenIssuer{
		secret: []byte(secret), ttl: ttl, idleTTL: idleTTL, maxAge: maxAge,
		superAdminIdleTTL: superAdminIdleTTL, superAdminMaxAge: superAdminMaxAge,
		cipher: cipher,
	}
}

// MaxAge returns the longer of the two possible hard session-lifetime
// backstops (super_admin vs. every other role) — the longest a token
// issued right now could possibly still verify, regardless of whose.
// Used as the Redis TTL for SessionStore.MarkRevoked, so a revocation
// record never expires before every token it must block could have
// expired on its own anyway; over-provisioning this TTL for a
// super_admin's (shorter) session is always safe.
func (t *TokenIssuer) MaxAge() time.Duration {
	if t.superAdminMaxAge > t.maxAge {
		return t.superAdminMaxAge
	}
	return t.maxAge
}

// Issue signs and encrypts a brand-new session for u — a fresh absolute
// expiry (now + ttl), a fresh idle deadline (now + idleTTL), and a fresh
// hard backstop (now + maxAge). Used only by login and password-change
// (real re-authentication events); every other authenticated request
// rotates the existing session via Refresh instead. permissions is ignored
// (and should be nil) for anything other than RoleStudioStaff — see
// Claims.Permissions. The returned sessionID is this login's durable
// identity (Claims.SessionID) — callers create the matching `sessions`
// row with it.
// twoFAVerifiedAt is nil unless the caller just checked a real code
// (login's code step) — see Claims.TwoFAVerifiedAt's doc comment.
func (t *TokenIssuer) Issue(ctx context.Context, u *User, permissions []string, twoFAVerifiedAt *time.Time) (token string, exp time.Time, jti string, csrfToken string, sessionID uuid.UUID, err error) {
	now := time.Now()
	_, maxAge := t.ttlsFor(u.Role)
	sessionMaxExp := now.Add(maxAge)
	csrfToken, err = generateCSRFToken()
	if err != nil {
		return "", time.Time{}, "", "", uuid.Nil, fmt.Errorf("generate csrf token: %w", err)
	}
	sessionID = uuid.New()
	token, exp, jti, err = t.issue(ctx, u, permissions, now.Add(t.ttl), sessionMaxExp, csrfToken, sessionID, twoFAVerifiedAt)
	return token, exp, jti, csrfToken, sessionID, err
}

// Refresh re-issues claims for the same session, sliding BOTH the absolute
// expiry and the idle deadline forward from now, while preserving
// sessionMaxExp — the hard backstop from the original login, which
// activity never extends (pass the existing token's
// Claims.SessionMaxExpiresAt here). Used by RequireAuth on every
// authenticated request: as long as activity (any authenticated request)
// keeps arriving within the idle window, the session never hits a fixed
// ceiling from the original login except that one outer backstop — only
// idleTTL of genuine inactivity (no requests at all), or sessionMaxExp
// itself passing, ever ends it. sessionID must be the existing token's
// Claims.SessionID — carried forward unchanged, same as csrfToken.
// twoFAVerifiedAt must be the existing token's Claims.TwoFAVerifiedAt
// to leave it unchanged (ordinary request rotation), or a fresh
// &time.Now() to refresh it (the step-up endpoint re-issuing after a
// just-checked code).
func (t *TokenIssuer) Refresh(ctx context.Context, u *User, permissions []string, sessionMaxExp time.Time, csrfToken string, sessionID uuid.UUID, twoFAVerifiedAt *time.Time) (token string, exp time.Time, jti string, err error) {
	exp = time.Now().Add(t.ttl)
	if exp.After(sessionMaxExp) {
		exp = sessionMaxExp // sliding never reaches past the hard backstop
	}
	return t.issue(ctx, u, permissions, exp, sessionMaxExp, csrfToken, sessionID, twoFAVerifiedAt)
}

func (t *TokenIssuer) issue(ctx context.Context, u *User, permissions []string, exp, sessionMaxExp time.Time, csrfToken string, sessionID uuid.UUID, twoFAVerifiedAt *time.Time) (token string, retExp time.Time, jti string, err error) {
	now := time.Now()
	idleTTL, _ := t.ttlsFor(u.Role)
	idleExp := now.Add(idleTTL)
	if idleExp.After(exp) {
		idleExp = exp // the idle window never reaches past the absolute cap
	}
	jti = uuid.NewString()
	c := Claims{
		UserID:              u.ID,
		StudioID:            u.StudioID,
		Role:                u.Role,
		Permissions:         permissions,
		MustResetPassword:   u.MustResetPassword,
		IdleExpiresAt:       jwt.NewNumericDate(idleExp),
		SessionMaxExpiresAt: jwt.NewNumericDate(sessionMaxExp),
		CSRFToken:           csrfToken,
		SessionID:           sessionID,
		TwoFAVerifiedAt:     numericDateOrNil(twoFAVerifiedAt),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   u.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			Issuer:    "projectx-api",
		},
	}
	kid, signingSecret, err := t.signingSecret(ctx)
	if err != nil {
		return "", time.Time{}, "", fmt.Errorf("resolve signing key: %w", err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(signingSecret)
	if err != nil {
		return "", time.Time{}, "", fmt.Errorf("sign jwt: %w", err)
	}
	encrypted, err := t.cipher.Encrypt(signed)
	if err != nil {
		return "", time.Time{}, "", fmt.Errorf("encrypt jwt: %w", err)
	}
	return encrypted, exp, jti, nil
}

// Parse decrypts raw (the cookie value) and verifies the signed JWT inside
// it, returning the claims. Callers still need to check claims.IdleExpiresAt
// (self-contained — no I/O needed) and the jti (claims.ID) against
// identity.SessionStore (revocation — see SessionStore.Exists) themselves —
// a token that decrypts and verifies cleanly can still belong to an
// idle-timed-out or revoked session.
func (t *TokenIssuer) Parse(ctx context.Context, raw string) (*Claims, error) {
	signed, err := t.cipher.Decrypt(raw)
	if err != nil {
		return nil, errors.New("invalid token")
	}
	parsed, err := jwt.ParseWithClaims(signed, &Claims{}, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		kid, _ := tok.Header["kid"].(string)
		return t.verifyingSecret(ctx, kid)
	})
	if err != nil {
		return nil, err
	}
	c, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}
