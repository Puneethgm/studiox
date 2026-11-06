// Package riskalert holds platform-security-hardening task 3.7's risk-
// trigger conditions as pure functions, evaluated against fixture data
// in tests (task 3.8) without needing to actually log in from a new
// country, exhaust login attempts, etc. Each function answers "should
// this fire" only — sending the resulting alert (via
// mail.Sender.SendSecurityAlert) and deciding who receives it is the
// caller's job, same separation already used by this codebase's other
// detect/act splits (e.g. identity.HasPriorSessionFromDevice vs. the
// email it triggers in completeLogin).
package riskalert

import "time"

// NewCountryLogin fires when an admin (super_admin/studio_admin) logs
// in from a country not in their recent-countries history.
//
// NOT WIRED LIVE in this change: this codebase has no GeoIP/IP-to-
// country lookup anywhere (Session.Country is always written as "" —
// see internal/identity/http.go's CreateSession call), so there is no
// real `country` value to evaluate here outside a test fixture. Wiring
// this live needs a GeoIP data source/service, which is genuinely
// outside this sandbox the same way AWS Secrets Manager (task 0.6) is —
// it is a new external dependency to choose and provision, not a config
// value already available. The function itself is correct and tested
// against fixture country lists.
func NewCountryLogin(isAdmin bool, country string, recentCountries []string) bool {
	if !isAdmin || country == "" {
		return false
	}
	for _, c := range recentCountries {
		if c == country {
			return false
		}
	}
	return true
}

// TwoFADisabled fires whenever a 2FA disable/reset action is recorded.
// Trivial by design — every disable is worth flagging, no threshold.
func TwoFADisabled() bool { return true }

// SecretChanged fires whenever a platform/studio secret (AI provider
// key, OCR key, etc) is set or rotated. Trivial by design, same as
// TwoFADisabled.
func SecretChanged() bool { return true }

// DataExported fires whenever a data export/import/deletion action is
// recorded. Trivial by design.
func DataExported() bool { return true }

// AdminRoleGranted fires when a role change's resulting role is
// super_admin or studio_admin and it was not already that role.
func AdminRoleGranted(oldRole, newRole string) bool {
	isAdminRole := func(r string) bool { return r == "super_admin" || r == "studio_admin" }
	return isAdminRole(newRole) && oldRole != newRole
}

// FailedLoginThreshold fires once failureCount reaches threshold (5, per
// the spec) within the counting window. The window itself is enforced
// by whatever counter the caller uses (e.g. a Redis TTL counter, the
// same shape as identity.TwoFALockoutStore) — this function only
// evaluates the count it's given.
func FailedLoginThreshold(failureCount int) bool {
	const threshold = 5
	return failureCount >= threshold
}

// OutsideStudioHours fires when an admin action timestamp falls outside
// [open, close) in the studio's own local time.
//
// NOT WIRED LIVE in this change: nothing in internal/studios currently
// models a studio's operating hours as queryable open/close times (the
// studios table has no such column at the time of writing) — this is a
// missing data model, not a missing API call, so it's listed here as
// blocked/partial rather than quietly skipped. The function itself is
// correct and tested against fixture open/close/action times.
func OutsideStudioHours(actionLocal time.Time, open, close time.Duration) bool {
	sinceMidnight := time.Duration(actionLocal.Hour())*time.Hour +
		time.Duration(actionLocal.Minute())*time.Minute +
		time.Duration(actionLocal.Second())*time.Second
	if open <= close {
		return sinceMidnight < open || sinceMidnight >= close
	}
	// Overnight hours (e.g. open 20:00, close 06:00) — outside is the
	// gap in the middle of the day.
	return sinceMidnight >= close && sinceMidnight < open
}
