package identity

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/projectx/api/internal/platform/httpx"
)

// permissionPrefixes maps a request path (relative to /studios/{studioId})
// to the permission key(s) that grant it, via longest-prefix match. See
// design.md decision 5's correction: a single handler package's routes
// often span more than one nav section (e.g. messaging.Handler.AdminRoutes
// alone covers Channels, Inbox, Jobs, and Leads), so this has to key off
// the URL, not the Go package serving it.
//
// Every entry below is grounded in an actual grep of which frontend page
// calls it (`grep -rohE "/api/v1/studios/\$\{studioId\}/..." <page-dir>`),
// not guessed from the endpoint's name — a guessed first pass got several
// of these wrong (see the corrections noted inline) and broke real usage:
// "Templates" (the nav item) and "/messaging/templates" (message-template
// entities used only by the Inbox composer) sound related but are two
// unrelated resources; "/communication-style" sounds like Settings but is
// only ever called from the Knowledge Base page.
//
// Some paths are used by more than one page and need more than one valid
// key — e.g. /messaging/upload (Inbox's attachment upload, but also used
// by the Settings page). Any one of a path's keys is sufficient; there's
// no method-aware (GET vs POST/PUT/DELETE) distinction. Acceptable for now
// (this is internal studio staff, not external users) — tighten later if
// that ever matters.
var permissionPrefixes = []struct {
	prefix string
	keys   []string
}{
	{"/messaging/channels", []string{"channels"}},
	{"/messaging/conversations", []string{"inbox"}},
	{"/messaging/stream", []string{"inbox"}},                        // SSE — live updates for the inbox UI
	{"/messaging/jobs", []string{"inbox"}},                          // pending/scheduled outbound messages, shown inside Inbox
	{"/messaging/upload", []string{"inbox", "settings"}},            // attachment upload — used by both Inbox and Settings
	{"/messaging/ai/generate", []string{"inbox", "social-planner"}}, // AI-assisted copy — used by both Inbox and Social Planner
	{"/messaging/followup-steps", []string{"decision-trees"}},       // only used by decision-trees/follow-ups
	{"/messaging/templates", []string{"inbox"}},                     // message-template entities, used only by Inbox's composer — NOT the "Templates" nav item (see above)
	{"/messaging/trigger-links", []string{"inbox"}},                 // same: used only by Inbox's composer
	{"/messaging/leads", []string{"leads"}},
	{"/messaging/settings", []string{"settings"}},
	{"/messaging", []string{"settings"}}, // fallback for any other /messaging/* sub-route
	{"/campaigns", []string{"campaigns"}},
	{"/leads", []string{"leads"}},
	{"/analytics", []string{"leads"}},
	{"/decision-trees", []string{"decision-trees"}},
	{"/social-posts", []string{"social-planner"}},
	{"/knowledge-base", []string{"knowledge-base"}},
	{"/communication-style", []string{"knowledge-base"}},    // corrected: only called from the Knowledge Base page, not Settings
	{"/style-refresh-interval", []string{"knowledge-base"}}, // same
	{"/program-start-date", []string{"knowledge-base"}},     // same
	{"/google-oauth", []string{"channels"}},                 // corrected: only called from the Channels page (Connect Google Ads), not Settings
	{"/ai-models", []string{"settings"}},
	{"/stripe-oauth", []string{"settings"}}, // no frontend caller found at all currently — kept as a safe default, unverified
	{"/initial-contact-delay", []string{"settings"}},
	{"/ai-reply-delay", []string{"settings"}},
}

// exemptPrefixes are paths that predate the permission catalog and already
// have their own, broader access rule — RequirePermission passes them
// through unconditionally rather than fail-closed-denying an unmapped path,
// leaving enforcement entirely to the route's own handler/middleware.
// `/users` (GET) predates this change and was already open to any
// authenticated member of the studio (checked inside listStudioUsers
// itself) — found in live testing when it broke the Inbox page's own
// assignee list for a studio_staff caller, which is not new gating this
// change intended to add. `/roles` and the write side of `/users`
// (POST/DELETE, this change's own new endpoints) are also exempt here
// because they're fully gated by RequireRole(studio_admin, super_admin)
// already (see RolesRoutes/StudioUserMgmtRoutes) — a studio_staff caller
// is rejected there, just not by this permission-catalog check.
var exemptPrefixes = []string{"/users", "/roles"}

// permissionKeyForPath returns the permission key(s) that grant path
// (already stripped of the /studios/{studioId} prefix — see
// requestPathAfterStudioID); any one of them is sufficient. ok is false if
// nothing in the table matches.
func permissionKeyForPath(path string) (keys []string, ok bool) {
	bestLen := -1
	for _, e := range permissionPrefixes {
		if strings.HasPrefix(path, e.prefix) && len(e.prefix) > bestLen {
			bestLen = len(e.prefix)
			keys = e.keys
			ok = true
		}
	}
	return keys, ok
}

// isExemptPath reports whether path is in exemptPrefixes (see its doc
// comment) and should bypass the permission catalog entirely.
func isExemptPath(path string) bool {
	for _, p := range exemptPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// requestPathAfterStudioID strips everything up through /studios/{studioId}
// from the raw request path, leaving e.g. "/messaging/channels".
func requestPathAfterStudioID(r *http.Request) string {
	studioID := chi.URLParam(r, "studioId")
	marker := "/studios/" + studioID
	idx := strings.Index(r.URL.Path, marker)
	if idx < 0 {
		return r.URL.Path
	}
	return r.URL.Path[idx+len(marker):]
}

// RequirePermission enforces the permission catalog for studio_staff
// callers, trusting the Permissions list embedded in the JWT at
// login/password-change — by explicit request, NOT checked live against
// the DB. Trade-off, accepted on request: a role's permissions (and this
// user's own deactivation, since neither is re-checked here) only take
// effect on this caller's next login/password-change, not their next
// click. (An earlier version of this middleware did check both live on
// every request — see git history / design.md decision 5 if that's ever
// needed again; reverted back to this JWT-trusting design on request.)
//
// A path with no table entry and no exemptPrefixes match is denied by
// default (fail closed) — the safety net for a genuinely new, unmapped
// endpoint, not for a pre-existing one that just needs listing above.
func RequirePermission(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		c, ok := ClaimsFrom(req.Context())
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		if c.BypassesPermissionChecks() {
			next.ServeHTTP(w, req)
			return
		}

		path := requestPathAfterStudioID(req)
		if isExemptPath(path) {
			next.ServeHTTP(w, req)
			return
		}

		keys, found := permissionKeyForPath(path)
		if !found {
			httpx.WriteError(w, http.StatusForbidden, "permission_denied", "you don't have access to this section")
			return
		}
		for _, need := range keys {
			if c.HasPermission(need) {
				next.ServeHTTP(w, req)
				return
			}
		}
		httpx.WriteError(w, http.StatusForbidden, "permission_denied", "you don't have access to this section")
	})
}

// RequirePasswordSet blocks every request from a caller whose Claims still
// have MustResetPassword set. It's only ever applied to route groups other
// than the /auth/* routes (login/logout/me/password all live outside the
// group this wraps — see cmd/server/main.go), so no path-based carve-out is
// needed inside the middleware itself: those routes are structurally
// unreachable through this chain.
func RequirePasswordSet(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := ClaimsFrom(r.Context())
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		if c.MustResetPassword {
			httpx.WriteError(w, http.StatusForbidden, "password_reset_required", "you must set a new password before continuing")
			return
		}
		next.ServeHTTP(w, r)
	})
}
