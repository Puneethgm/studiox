package identity

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleSuperAdmin  Role = "super_admin"
	RoleStudioAdmin Role = "studio_admin"
)

func (r Role) Valid() bool {
	switch r {
	case RoleSuperAdmin, RoleStudioAdmin:
		return true
	}
	return false
}

type User struct {
	ID           uuid.UUID
	StudioID     *uuid.UUID // nil for super_admin
	Email        string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
