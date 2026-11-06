package identity

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestTokenIssuer_RoleBasedTTLs covers platform-security-hardening task
// 1.8: super_admin gets a tighter idle/max-age ceiling than every other
// role, from the same TokenIssuer instance.
func TestTokenIssuer_RoleBasedTTLs(t *testing.T) {
	issuer := NewTokenIssuerWithSuperAdminTTLs(
		"test-secret-at-least-32-characters-long",
		time.Hour,      // ttl
		30*time.Minute, // other-role idle
		7*24*time.Hour, // other-role max age
		15*time.Minute, // super_admin idle
		8*time.Hour,    // super_admin max age
		testCipher(t),
	)

	super := &User{ID: uuid.New(), Role: RoleSuperAdmin}
	token, _, _, _, _, err := issuer.Issue(context.Background(), super, nil, nil)
	if err != nil {
		t.Fatalf("issue super_admin: %v", err)
	}
	claims, err := issuer.Parse(context.Background(), token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	now := time.Now()
	if got := claims.IdleExpiresAt.Time.Sub(now); got > 16*time.Minute || got < 14*time.Minute {
		t.Errorf("super_admin idle deadline = %v from now, want ~15m", got)
	}
	if got := claims.SessionMaxExpiresAt.Time.Sub(now); got > 8*time.Hour+time.Minute || got < 8*time.Hour-time.Minute {
		t.Errorf("super_admin max-age deadline = %v from now, want ~8h", got)
	}

	studioID := uuid.New()
	admin := &User{ID: uuid.New(), Role: RoleStudioAdmin, StudioID: &studioID}
	token, _, _, _, _, err = issuer.Issue(context.Background(), admin, nil, nil)
	if err != nil {
		t.Fatalf("issue studio_admin: %v", err)
	}
	claims, err = issuer.Parse(context.Background(), token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := claims.IdleExpiresAt.Time.Sub(now); got > 31*time.Minute || got < 29*time.Minute {
		t.Errorf("studio_admin idle deadline = %v from now, want ~30m", got)
	}
	if got := claims.SessionMaxExpiresAt.Time.Sub(now); got > 7*24*time.Hour+time.Minute || got < 7*24*time.Hour-time.Minute {
		t.Errorf("studio_admin max-age deadline = %v from now, want ~7d", got)
	}
}

func TestTokenIssuer_MaxAge_ReturnsTheLongerOfTheTwo(t *testing.T) {
	issuer := NewTokenIssuerWithSuperAdminTTLs(
		"test-secret-at-least-32-characters-long",
		time.Hour, 30*time.Minute, 7*24*time.Hour, 15*time.Minute, 8*time.Hour, testCipher(t),
	)
	if got := issuer.MaxAge(); got != 7*24*time.Hour {
		t.Errorf("MaxAge() = %v, want the longer value (7 days)", got)
	}
}
