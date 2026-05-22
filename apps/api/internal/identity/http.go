package identity

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/logger"
)

type Handler struct {
	repo        *Repo
	tokens      *TokenIssuer
	cookie      config.CookieConfig
	studioBrand StudioBrandLookup
}

// StudioBrand is a minimal projection of the studio table — just enough for
// the frontend to render branded chrome immediately after login. The `Active`
// flag drives the inactive-studio lockout in the AppShell.
type StudioBrand struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	BrandColor string `json:"brandColor"`
	LogoURL    string `json:"logoUrl"`
	Active     bool   `json:"active"`
}

// StudioBrandLookup resolves a studio's brand info by id. Implemented in main
// using the studios package — kept as a func so identity has no compile-time
// dependency on studios.
type StudioBrandLookup func(ctx context.Context, id uuid.UUID) (*StudioBrand, error)

func NewHandler(repo *Repo, tokens *TokenIssuer, cookie config.CookieConfig, brand StudioBrandLookup) *Handler {
	return &Handler{repo: repo, tokens: tokens, cookie: cookie, studioBrand: brand}
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/auth/login", h.login)
	r.Post("/auth/logout", h.logout)
	r.With(h.RequireAuth).Get("/auth/me", h.me)
	r.With(h.RequireAuth).Post("/auth/password", h.changePassword)
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type meRes struct {
	ID       uuid.UUID    `json:"id"`
	Email    string       `json:"email"`
	Role     Role         `json:"role"`
	StudioID *uuid.UUID   `json:"studioId,omitempty"`
	Studio   *StudioBrand `json:"studio,omitempty"`
}

func (h *Handler) buildMeRes(ctx context.Context, u *User) meRes {
	res := meRes{ID: u.ID, Email: u.Email, Role: u.Role, StudioID: u.StudioID}
	if u.StudioID != nil && h.studioBrand != nil {
		if b, err := h.studioBrand(ctx, *u.StudioID); err == nil && b != nil {
			res.Studio = b
		}
	}
	return res
}

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

	token, exp, err := h.tokens.Issue(u)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.cookie.Name,
		Value:    token,
		Path:     "/",
		Domain:   h.cookie.Domain,
		Expires:  exp,
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	httpx.JSON(w, http.StatusOK, h.buildMeRes(r.Context(), u))
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookie.Name,
		Value:    "",
		Path:     "/",
		Domain:   h.cookie.Domain,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	httpx.NoContent(w)
}

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
				SameSite: http.SameSiteLaxMode,
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
	if err := h.repo.UpdatePasswordHash(r.Context(), c.UserID, hash); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "session no longer valid")
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

// RequireAuth verifies the session cookie and injects claims into the context.
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read and log the incoming body so we can capture payloads even when
		// the request is unauthenticated. Restore the body for downstream
		// handlers.
		if r.Body != nil {
			if b, err := io.ReadAll(r.Body); err == nil {
				logger.FromCtx(r.Context(), slog.Default()).Info("auth_incoming_body",
					"method", r.Method,
					"path", r.URL.Path,
					"content_type", r.Header.Get("Content-Type"),
					"body", string(b),
				)
				r.Body = io.NopCloser(bytes.NewReader(b))
			}
		}

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
