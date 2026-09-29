package identity

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/httpx"
)

// RolesRoutes mounts the studio-scoped role endpoints (identity/studio-roles).
// Callers must be studio_admin (their own studio) or super_admin — a
// teammate managing other teammates' access isn't itself a grantable
// permission, so this is gated by role, not the permission catalog (see
// rbac_middleware.go's doc comment).
func (h *Handler) RolesRoutes(r chi.Router) {
	r.Use(RequireRole(RoleStudioAdmin, RoleSuperAdmin))
	r.Get("/roles", h.listRoles)
	r.Post("/roles", h.createRole)
	r.Get("/roles/{roleId}", h.getRole)
	r.Patch("/roles/{roleId}", h.updateRole)
	r.Patch("/roles/{roleId}/active", h.setRoleActive)
	r.Delete("/roles/{roleId}", h.deleteRole)
}

type permissionRes struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// listPermissions godoc
//
//	@Summary		List the permission catalog
//	@Description	Returns the fixed, platform-wide set of grantable permissions (one per left-nav section).
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Router			/api/v1/permissions [get]
func (h *Handler) listPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.repo.ListPermissions(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to list permissions")
		return
	}
	res := make([]permissionRes, len(perms))
	for i, p := range perms {
		res[i] = permissionRes{Key: p.Key, Label: p.Label}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"permissions": res})
}

type roleReq struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	PermissionKeys []string `json:"permissionKeys"`
}

type roleRes struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	PermissionKeys []string  `json:"permissionKeys"`
	Active         bool      `json:"active"`
}

func (h *Handler) buildRoleRes(r *http.Request, role *StudioRole) (roleRes, error) {
	keys, err := h.repo.GetRolePermissionKeys(r.Context(), role.ID)
	if err != nil {
		return roleRes{}, err
	}
	if keys == nil {
		keys = []string{}
	}
	return roleRes{ID: role.ID, Name: role.Name, Description: role.Description, PermissionKeys: keys, Active: role.Active}, nil
}

// createRole godoc
//
//	@Summary		Create a studio role
//	@Description	Creates a role scoped to this studio, with the given permission grants. Studio admin (own studio) or super admin only.
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID (UUID)"
//	@Param			body		body		roleReq	true	"Role name, description, permission keys"
//	@Success		201			{object}	roleRes
//	@Failure		409			{object}	httpx.ErrorResponse	"role name already exists in this studio"
//	@Router			/api/v1/studios/{studioId}/roles [post]
func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid studioId")
		return
	}
	var req roleReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httpx.WriteValidationError(w, map[string]string{"name": "required"})
		return
	}

	c := MustClaims(r.Context())
	role, err := h.repo.CreateRole(r.Context(), studioID, req.Name, req.Description, &c.UserID)
	if err != nil {
		if errors.Is(err, ErrRoleNameTaken) {
			httpx.WriteError(w, http.StatusConflict, "role_name_taken", "role name already exists in this studio")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to create role")
		return
	}

	if len(req.PermissionKeys) > 0 {
		if err := h.repo.SetRolePermissions(r.Context(), role.ID, req.PermissionKeys, &c.UserID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to set role permissions")
			return
		}
	}

	res, err := h.buildRoleRes(r, role)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load role")
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

// listRoles godoc
//
//	@Summary		List a studio's roles
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path	string	true	"Studio ID (UUID)"
//	@Success		200			{object}	map[string]interface{}
//	@Router			/api/v1/studios/{studioId}/roles [get]
func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid studioId")
		return
	}
	roles, err := h.repo.ListRoles(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to list roles")
		return
	}
	res := make([]roleRes, len(roles))
	for i, role := range roles {
		rr, err := h.buildRoleRes(r, &role)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load role permissions")
			return
		}
		res[i] = rr
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"roles": res})
}

// getRole godoc
//
//	@Summary		Get a studio role
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID (UUID)"
//	@Param			roleId		path		string	true	"Role ID (UUID)"
//	@Success		200			{object}	roleRes
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/roles/{roleId} [get]
func (h *Handler) getRole(w http.ResponseWriter, r *http.Request) {
	studioID, roleID, ok := parseStudioAndRoleID(w, r)
	if !ok {
		return
	}
	role, err := h.repo.GetRole(r.Context(), studioID, roleID)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "role not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load role")
		return
	}
	res, err := h.buildRoleRes(r, role)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load role permissions")
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

// updateRole godoc
//
//	@Summary		Update a studio role
//	@Description	Updates the role's name/description and replaces its entire permission grant with the given keys.
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID (UUID)"
//	@Param			roleId		path		string	true	"Role ID (UUID)"
//	@Param			body		body		roleReq	true	"Role name, description, permission keys"
//	@Success		200			{object}	roleRes
//	@Router			/api/v1/studios/{studioId}/roles/{roleId} [patch]
func (h *Handler) updateRole(w http.ResponseWriter, r *http.Request) {
	studioID, roleID, ok := parseStudioAndRoleID(w, r)
	if !ok {
		return
	}
	var req roleReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httpx.WriteValidationError(w, map[string]string{"name": "required"})
		return
	}

	role, err := h.repo.GetRole(r.Context(), studioID, roleID)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "role not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load role")
		return
	}

	c := MustClaims(r.Context())
	if err := h.repo.UpdateRole(r.Context(), studioID, roleID, req.Name, req.Description, &c.UserID); err != nil {
		if errors.Is(err, ErrRoleNameTaken) {
			httpx.WriteError(w, http.StatusConflict, "role_name_taken", "role name already exists in this studio")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update role")
		return
	}
	if err := h.repo.SetRolePermissions(r.Context(), role.ID, req.PermissionKeys, &c.UserID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to set role permissions")
		return
	}

	updated, err := h.repo.GetRole(r.Context(), studioID, roleID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to reload role")
		return
	}
	res, err := h.buildRoleRes(r, updated)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load role permissions")
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

type setRoleActiveReq struct {
	Active bool `json:"active"`
}

// setRoleActive godoc
//
//	@Summary		Activate or deactivate a studio role
//	@Description	Deactivating a role immediately blocks login for every user currently assigned to it (checked at login time, not live-enforced on existing sessions). Reversible — distinct from deleting the role.
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string				true	"Studio ID (UUID)"
//	@Param			roleId		path		string				true	"Role ID (UUID)"
//	@Param			body		body		setRoleActiveReq	true	"Desired active state"
//	@Success		200			{object}	roleRes
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/roles/{roleId}/active [patch]
func (h *Handler) setRoleActive(w http.ResponseWriter, r *http.Request) {
	studioID, roleID, ok := parseStudioAndRoleID(w, r)
	if !ok {
		return
	}
	var req setRoleActiveReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	c := MustClaims(r.Context())
	if err := h.repo.SetRoleActive(r.Context(), studioID, roleID, req.Active, &c.UserID); err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "role not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update role")
		return
	}

	updated, err := h.repo.GetRole(r.Context(), studioID, roleID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to reload role")
		return
	}
	res, err := h.buildRoleRes(r, updated)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load role permissions")
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

// deleteRole godoc
//
//	@Summary		Delete a studio role
//	@Description	Rejected with 409 if any user still has this role assigned (spec: identity/studio-roles).
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Param			studioId	path	string	true	"Studio ID (UUID)"
//	@Param			roleId		path	string	true	"Role ID (UUID)"
//	@Success		204			"no content"
//	@Failure		409			{object}	httpx.ErrorResponse	"role still assigned to one or more users"
//	@Router			/api/v1/studios/{studioId}/roles/{roleId} [delete]
func (h *Handler) deleteRole(w http.ResponseWriter, r *http.Request) {
	studioID, roleID, ok := parseStudioAndRoleID(w, r)
	if !ok {
		return
	}
	if err := h.repo.DeleteRole(r.Context(), studioID, roleID); err != nil {
		switch {
		case errors.Is(err, ErrRoleInUse):
			httpx.WriteError(w, http.StatusConflict, "role_in_use", "this role is still assigned to one or more users — reassign them first")
		case errors.Is(err, ErrRoleNotFound):
			httpx.WriteError(w, http.StatusNotFound, "not_found", "role not found")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to delete role")
		}
		return
	}
	httpx.NoContent(w)
}

func parseStudioAndRoleID(w http.ResponseWriter, r *http.Request) (studioID, roleID uuid.UUID, ok bool) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid studioId")
		return uuid.Nil, uuid.Nil, false
	}
	roleID, err = uuid.Parse(chi.URLParam(r, "roleId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid roleId")
		return uuid.Nil, uuid.Nil, false
	}
	return studioID, roleID, true
}
