package identity

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleSuperAdmin  Role = "super_admin"
	RoleStudioAdmin Role = "studio_admin"
	// RoleStudioStaff is a teammate a studio_admin created — the only role
	// subject to permission checks (see RequirePermission). super_admin and
	// studio_admin always bypass them.
	RoleStudioStaff Role = "studio_staff"
)

func (r Role) Valid() bool {
	switch r {
	case RoleSuperAdmin, RoleStudioAdmin, RoleStudioStaff:
		return true
	}
	return false
}

// DefaultTeammatePassword is set on every studio_staff user created via
// CreateStudioUser — never chosen by the creating admin. must_reset_password
// is always true alongside it, and every request except logout/me/password
// is blocked until the teammate changes it (see RequirePasswordSet).
const DefaultTeammatePassword = "password123"

type User struct {
	ID                uuid.UUID
	StudioID          *uuid.UUID // nil for super_admin
	Email             string
	PasswordHash      string
	Role              Role
	Username          *string // set only for studio_staff; unique per studio_id
	FirstName         *string
	LastName          *string
	RoleID            *uuid.UUID // FK to StudioRole; set only for studio_staff
	MustResetPassword bool
	// Active is a reversible login gate, distinct from DeletedAt (a
	// permanent, one-way removal) — toggled from the Users tab; a deactivated
	// user simply can't log in until reactivated. Defaults true.
	Active    bool
	DeletedAt *time.Time
	DeletedBy *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// StudioRole is a studio-defined bundle of permissions (identity/studio-roles).
type StudioRole struct {
	ID          uuid.UUID
	StudioID    uuid.UUID
	Name        string
	Description string
	// Active is a reversible gate on the whole role — deactivating it blocks
	// login for every user currently assigned to it (checked at login time,
	// same staleness trade-off as everything else in Claims — see jwt.go).
	// Defaults true.
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy *uuid.UUID
	UpdatedBy *uuid.UUID
}

// Permission is one entry in the fixed, platform-wide catalog — one per
// left-nav section. Studios cannot define their own.
type Permission struct {
	ID    uuid.UUID
	Key   string
	Label string
}
