package riskalert

import (
	"testing"
	"time"
)

func TestNewCountryLogin(t *testing.T) {
	cases := []struct {
		name    string
		isAdmin bool
		country string
		recent  []string
		want    bool
	}{
		{"non-admin never fires", false, "FR", []string{"SG"}, false},
		{"empty country never fires", true, "", []string{"SG"}, false},
		{"known country does not fire", true, "SG", []string{"SG", "MY"}, false},
		{"new country fires", true, "FR", []string{"SG", "MY"}, true},
		{"first-ever login fires (empty history)", true, "SG", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NewCountryLogin(c.isAdmin, c.country, c.recent); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestTwoFADisabled_AlwaysTrue(t *testing.T) {
	if !TwoFADisabled() {
		t.Error("expected always true")
	}
}

func TestSecretChanged_AlwaysTrue(t *testing.T) {
	if !SecretChanged() {
		t.Error("expected always true")
	}
}

func TestDataExported_AlwaysTrue(t *testing.T) {
	if !DataExported() {
		t.Error("expected always true")
	}
}

func TestAdminRoleGranted(t *testing.T) {
	cases := []struct {
		name             string
		oldRole, newRole string
		want             bool
	}{
		{"staff to studio_admin fires", "studio_staff", "studio_admin", true},
		{"studio_admin to super_admin fires", "studio_admin", "super_admin", true},
		{"already super_admin does not fire", "super_admin", "super_admin", false},
		{"staff to staff does not fire", "studio_staff", "studio_staff", false},
		{"admin demoted to staff does not fire", "studio_admin", "studio_staff", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AdminRoleGranted(c.oldRole, c.newRole); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestFailedLoginThreshold(t *testing.T) {
	if FailedLoginThreshold(4) {
		t.Error("4 failures should not fire")
	}
	if !FailedLoginThreshold(5) {
		t.Error("5 failures should fire")
	}
	if !FailedLoginThreshold(6) {
		t.Error("6 failures should still fire")
	}
}

func TestOutsideStudioHours(t *testing.T) {
	loc := time.UTC
	open := 9 * time.Hour
	close := 18 * time.Hour

	cases := []struct {
		name   string
		hour   int
		minute int
		want   bool
	}{
		{"9am is within hours", 9, 0, false},
		{"5:59pm is within hours", 17, 59, false},
		{"6pm is outside (close is exclusive)", 18, 0, true},
		{"8am is outside", 8, 0, true},
		{"midnight is outside", 0, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ts := time.Date(2026, 10, 30, c.hour, c.minute, 0, 0, loc)
			if got := OutsideStudioHours(ts, open, close); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestOutsideStudioHours_OvernightWindow(t *testing.T) {
	// A studio open 8pm-6am (overnight).
	open := 20 * time.Hour
	close := 6 * time.Hour

	midnightAction := time.Date(2026, 10, 30, 0, 30, 0, 0, time.UTC)
	if OutsideStudioHours(midnightAction, open, close) {
		t.Error("00:30 should be within an overnight 20:00-06:00 window")
	}
	noonAction := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	if !OutsideStudioHours(noonAction, open, close) {
		t.Error("noon should be outside an overnight 20:00-06:00 window")
	}
}
