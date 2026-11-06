package identity

import (
	"strings"
)

// parseUserAgent turns a raw User-Agent header into a human-readable
// (device, browser) pair for the sessions list (sessions_http.go's
// toSessionRes) — e.g. "Linux" / "Chrome 145" instead of the full raw
// string. HasPriorSessionFromDevice still compares the raw stored value
// directly, unaffected by this — this is presentation-only.
func parseUserAgent(ua string) (device, browser string) {
	if ua == "" {
		return "", ""
	}
	return parseDevice(ua), parseBrowser(ua)
}

func parseDevice(ua string) string {
	switch {
	case strings.Contains(ua, "iPad"):
		return "iPad"
	case strings.Contains(ua, "iPhone"):
		return "iPhone"
	case strings.Contains(ua, "Android"):
		if strings.Contains(ua, "Mobile") {
			return "Android"
		}
		return "Android tablet"
	case strings.Contains(ua, "Windows"):
		return "Windows"
	case strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Mac OS X"):
		return "Mac"
	case strings.Contains(ua, "Linux"):
		return "Linux"
	default:
		return "Unknown device"
	}
}

// Checked in order — Edge and Opera's UAs also contain "Chrome" and
// "Safari" tokens, and Chrome's UA also contains "Safari", so the more
// specific tokens must win first.
var browserPatterns = []struct {
	token string
	name  string
}{
	{"Edg/", "Edge"},
	{"OPR/", "Opera"},
	{"Chrome/", "Chrome"},
	{"Firefox/", "Firefox"},
	{"Safari/", "Safari"},
}

func parseBrowser(ua string) string {
	for _, p := range browserPatterns {
		idx := strings.Index(ua, p.token)
		if idx == -1 {
			continue
		}
		rest := ua[idx+len(p.token):]
		version := ""
		for _, c := range rest {
			if c < '0' || c > '9' {
				break
			}
			version += string(c)
		}
		if version != "" {
			// Only the major version — "145.0.0.0" -> "145".
			return p.name + " " + version
		}
		return p.name
	}
	return "Unknown browser"
}
