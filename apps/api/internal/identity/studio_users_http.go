package identity

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/secaudit"
)

// StudioUserMgmtRoutes mounts teammate create/deactivate (identity/studio-users).
// Studio admin (own studio) or super admin only — see roles_http.go's
// RolesRoutes doc comment for why this is a role gate, not a permission.
// Kept separate from the existing StudioRoutes (GET /users, open to any
// studio member) to avoid changing that endpoint's existing access rules.
func (h *Handler) StudioUserMgmtRoutes(r chi.Router) {
	r.Use(RequireRole(RoleStudioAdmin, RoleSuperAdmin))
	// Admin add/remove requires a fresh code (task 2.6).
	r.With(RequireStepUp).Post("/users", h.createStudioUser)
	r.With(RequireStepUp).Patch("/users/{userId}/active", h.setStudioUserActive)
	r.With(RequireStepUp).Delete("/users/{userId}", h.deactivateStudioUser)
	r.Post("/users/{userId}/sessions/end", h.forceLogoutStudioUser)
	r.With(RequireStepUp).Post("/users/{userId}/2fa/reset", h.resetStudioUserTOTP)
}

type createStudioUserReq struct {
	Username  string `json:"username"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	RoleID    string `json:"roleId"`
}

type studioUserRes struct {
	ID                uuid.UUID  `json:"id"`
	Email             string     `json:"email"`
	Role              Role       `json:"role"`
	Username          *string    `json:"username,omitempty"`
	FirstName         *string    `json:"firstName,omitempty"`
	LastName          *string    `json:"lastName,omitempty"`
	RoleID            *uuid.UUID `json:"roleId,omitempty"`
	RoleName          *string    `json:"roleName,omitempty"`
	MustResetPassword bool       `json:"mustResetPassword"`
	Active            bool       `json:"active"`
	CreatedAt         time.Time  `json:"createdAt"`
}

// createStudioUser godoc
//
//	@Summary		Add a teammate to a studio
//	@Description	Creates a studio_staff user with a fixed default password and a forced password-reset requirement. No password is accepted here — see identity/studio-users.
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string				true	"Studio ID (UUID)"
//	@Param			body		body		createStudioUserReq	true	"Teammate details"
//	@Success		201			{object}	studioUserRes
//	@Failure		409			{object}	httpx.ErrorResponse	"username or email already taken"
//	@Router			/api/v1/studios/{studioId}/users [post]
func (h *Handler) createStudioUser(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid studioId")
		return
	}
	var req createStudioUserReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	errs := map[string]string{}
	if req.Username == "" {
		errs["username"] = "required"
	}
	if req.FirstName == "" {
		errs["firstName"] = "required"
	}
	if req.LastName == "" {
		errs["lastName"] = "required"
	}
	if req.Email == "" {
		errs["email"] = "required"
	}
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		errs["roleId"] = "required and must be a valid role id"
	}
	if len(errs) > 0 {
		httpx.WriteValidationError(w, errs)
		return
	}

	// The role must belong to this studio — otherwise a studio admin could
	// assign a teammate a role scoped to a different studio entirely.
	role, err := h.repo.GetRole(r.Context(), studioID, roleID)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			httpx.WriteValidationError(w, map[string]string{"roleId": "role not found in this studio"})
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to look up role")
		return
	}

	c := MustClaims(r.Context())
	tempPassword, err := GenerateTempPassword()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to create user")
		return
	}
	userID, err := h.repo.CreateStudioUserWithPassword(r.Context(), studioID, req.Username, req.FirstName, req.LastName, req.Email, roleID, &c.UserID, tempPassword)
	if err != nil {
		switch {
		case errors.Is(err, ErrUsernameTaken):
			httpx.WriteError(w, http.StatusConflict, "username_taken", "username already taken in this studio")
		case errors.Is(err, ErrEmailTaken):
			httpx.WriteError(w, http.StatusConflict, "email_taken", "email already in use")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to create user")
		}
		return
	}

	u, err := h.repo.FindByID(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load created user")
		return
	}

	// Best-effort: a teammate who never gets this email can still be told
	// their login details in person / reset their password via "forgot
	// password" — not worth failing the whole creation over a flaky send.
	if h.mailer != nil && h.mailer.Enabled() {
		if studioName, err := h.repo.GetStudioName(r.Context(), studioID); err != nil {
			slog.Warn("create studio user: failed to look up studio name for welcome email", "err", err)
		} else {
			loginLink := h.frontendURL + "/login"
			if err := h.mailer.SendTeammateWelcome(u.Email, studioName, role.Name, req.Username, tempPassword, loginLink); err != nil {
				slog.Warn("create studio user: failed to send welcome email", "err", err)
			}
		}
	}

	h.logSecurityEvent(r.Context(), r, &c.UserID, string(c.Role), &studioID, "user.created", secaudit.OutcomeSuccess, map[string]any{"targetUserId": userID, "roleId": roleID})

	httpx.JSON(w, http.StatusCreated, studioUserToRes(u, &role.Name))
}

func studioUserToRes(u *User, roleName *string) studioUserRes {
	return studioUserRes{
		ID: u.ID, Email: u.Email, Role: u.Role, Username: u.Username,
		FirstName: u.FirstName, LastName: u.LastName, RoleID: u.RoleID, RoleName: roleName,
		MustResetPassword: u.MustResetPassword, Active: u.Active, CreatedAt: u.CreatedAt,
	}
}

type setUserActiveReq struct {
	Active bool `json:"active"`
}

// setStudioUserActive godoc
//
//	@Summary		Activate or deactivate a teammate
//	@Description	A reversible login gate, distinct from deactivateStudioUser's permanent soft-delete — deactivating blocks login immediately (checked at login time), reactivating restores it. studio_admin can't be toggled here (no role_id path applies to them; use deactivateStudioUser's soft-delete if that's genuinely needed).
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string				true	"Studio ID (UUID)"
//	@Param			userId		path		string				true	"User ID (UUID)"
//	@Param			body		body		setUserActiveReq	true	"Desired active state"
//	@Success		200			{object}	studioUserRes
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/users/{userId}/active [patch]
func (h *Handler) setStudioUserActive(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid studioId")
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid userId")
		return
	}
	var req setUserActiveReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	c := MustClaims(r.Context())
	if err := h.repo.SetUserActive(r.Context(), studioID, userID, req.Active, &c.UserID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update user")
		return
	}
	// Deactivation (not reactivation) ends this user's existing sessions
	// immediately rather than waiting for their token to idle-time-out
	// (platform-security-hardening task 1.4).
	if !req.Active {
		if err := h.RevokeSessionsForUser(r.Context(), userID, "user deactivated"); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
	}
	h.logSecurityEvent(r.Context(), r, &c.UserID, string(c.Role), &studioID, "user.active_changed", secaudit.OutcomeSuccess, map[string]any{"targetUserId": userID, "active": req.Active})

	u, err := h.repo.FindByID(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to reload user")
		return
	}
	var roleName *string
	if u.RoleID != nil {
		if role, err := h.repo.GetRole(r.Context(), studioID, *u.RoleID); err == nil {
			roleName = &role.Name
		}
	}
	httpx.JSON(w, http.StatusOK, studioUserToRes(u, roleName))
}

// deactivateStudioUser godoc
//
//	@Summary		Deactivate a teammate
//	@Description	Soft-deletes the user — the row stays (with who deactivated it and when), and they can no longer log in.
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Param			studioId	path	string	true	"Studio ID (UUID)"
//	@Param			userId		path	string	true	"User ID (UUID)"
//	@Success		204			"no content"
//	@Router			/api/v1/studios/{studioId}/users/{userId} [delete]
func (h *Handler) deactivateStudioUser(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid studioId")
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid userId")
		return
	}

	c := MustClaims(r.Context())
	if err := h.repo.DeactivateStudioUser(r.Context(), studioID, userID, c.UserID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to deactivate user")
		return
	}
	// Same reasoning as setStudioUserActive's deactivation branch — ends
	// their sessions immediately (task 1.4).
	if err := h.RevokeSessionsForUser(r.Context(), userID, "user removed"); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	h.logSecurityEvent(r.Context(), r, &c.UserID, string(c.Role), &studioID, "user.deactivated", secaudit.OutcomeSuccess, map[string]any{"targetUserId": userID})
	httpx.NoContent(w)
}

// forceLogoutStudioUser godoc
//
//	@Summary		Force-log-out a teammate
//	@Description	Ends every active session for this user immediately. studio_admin may only target a user in their own studio; super_admin may target any studio's user through this same studio-scoped path.
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Param			studioId	path	string	true	"Studio ID (UUID)"
//	@Param			userId		path	string	true	"User ID (UUID)"
//	@Success		204			"no content"
//	@Failure		404			{object}	httpx.ErrorResponse	"user not found in this studio"
//	@Router			/api/v1/studios/{studioId}/users/{userId}/sessions/end [post]
func (h *Handler) forceLogoutStudioUser(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid studioId")
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid userId")
		return
	}
	// RequirePermission/RequireActiveStudio already resolve studioId for a
	// studio_admin's own studio; this explicit membership check is what
	// stops a studio_admin reaching a DIFFERENT studio's user by guessing
	// a userId (super_admin, not scoped to any one studio, passes the
	// same check against whichever studioId they supplied).
	belongs, err := h.repo.UserBelongsToStudio(r.Context(), studioID, userID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if !belongs {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "user not found in this studio")
		return
	}
	if err := h.RevokeSessionsForUser(r.Context(), userID, "force-logout by admin"); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	h.notifySessionsEnded(r.Context(), userID, "an administrator ended your sessions")
	httpx.NoContent(w)
}
