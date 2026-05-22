package identity

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID   uuid.UUID  `json:"uid"`
	StudioID *uuid.UUID `json:"sid,omitempty"` // omitted for super_admin
	Role     Role       `json:"role"`
	jwt.RegisteredClaims
}

// IsSuper returns true if the claims belong to a super admin.
func (c *Claims) IsSuper() bool { return c.Role == RoleSuperAdmin }

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

type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenIssuer(secret string, ttl time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), ttl: ttl}
}

func (t *TokenIssuer) Issue(u *User) (string, time.Time, error) {
	exp := time.Now().Add(t.ttl)
	c := Claims{
		UserID:   u.ID,
		StudioID: u.StudioID,
		Role:     u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(exp),
			Issuer:    "projectx-api",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := tok.SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}
	return signed, exp, nil
}

func (t *TokenIssuer) Parse(raw string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(raw, &Claims{}, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return t.secret, nil
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
