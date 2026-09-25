package messaging

import (
	"strings"
	"time"

	"github.com/nyaruka/phonenumbers"
)

// phoneBasedChannels are the channel kinds where Conversation.ContactValue is
// the recipient's actual phone number (not a platform-scoped chat/user ID),
// so a per-recipient timezone can be derived from it via libphonenumber.
var phoneBasedChannels = map[ChannelKind]bool{
	KindWhatsAppMeta: true,
	KindWhatsAppWeb:  true,
	KindSMS:          true,
}

// greetingLocation resolves the timezone used for the "Good
// morning/afternoon/evening" decision in buildPrompt. It prefers the
// recipient's own country (from their phone number's calling code) so a lead
// in a different timezone from the studio gets a greeting that matches their
// local time of day, falling back to studioLoc (the studio's
// AvailabilityTimezone) when the channel isn't phone-based, the number
// doesn't parse, or libphonenumber has no timezone mapping for it — e.g.
// Telegram's numeric chat IDs, malformed numbers, or unmapped ranges.
func greetingLocation(conv *Conversation, studioLoc *time.Location) *time.Location {
	if conv != nil && phoneBasedChannels[conv.ChannelKind] {
		if loc := timezoneForPhone(conv.ContactValue); loc != nil {
			return loc
		}
	}
	return studioLoc
}

// resolveGreetingLocation picks the timezone for the greeting decision.
// tzOverride (an IANA zone name, e.g. "Asia/Kolkata") wins when it's set and
// loads successfully — this is how Test Chat greets using the admin's own
// browser/system clock instead of a phone number, since there's no real
// recipient to derive one from. Real conversations never set an override, so
// they fall through to greetingLocation's phone-based lookup as before.
func resolveGreetingLocation(tzOverride string, conv *Conversation, studioLoc *time.Location) *time.Location {
	if tzOverride != "" {
		if loc, err := time.LoadLocation(tzOverride); err == nil {
			return loc
		}
	}
	return greetingLocation(conv, studioLoc)
}

func timezoneForPhone(raw string) *time.Location {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	// WhatsApp Web's contact_identities.value is the raw JID (e.g.
	// "917483974512@c.us"), not a clean phone number — phonenumbers.Parse
	// rejects the "@c.us" suffix outright (no tolerance for trailing
	// non-digit characters), which silently failed this lookup for every
	// WhatsApp Web contact, always falling through to the studio's
	// timezone/UTC default regardless of the real recipient's location.
	// @lid/@g.us are WhatsApp's own internal identifiers, not phone numbers
	// at all — those must return nil rather than being parsed as one, which
	// would confidently resolve to a bogus country/timezone.
	if strings.HasSuffix(raw, "@lid") || strings.HasSuffix(raw, "@g.us") {
		return nil
	}
	if idx := strings.IndexByte(raw, '@'); idx != -1 {
		raw = raw[:idx]
	}
	// WhatsApp Cloud API's wa_id (and a bare WhatsApp Web number once the
	// JID suffix above is stripped) are digits-only E.164 with no leading
	// "+" — libphonenumber requires it for region-less parsing.
	if !strings.HasPrefix(raw, "+") {
		raw = "+" + raw
	}
	num, err := phonenumbers.Parse(raw, "")
	if err != nil || num == nil {
		return nil
	}
	zones, err := phonenumbers.GetTimezonesForNumber(num)
	if err != nil || len(zones) == 0 || zones[0] == "" || zones[0] == "Etc/Unknown" {
		return nil
	}
	loc, err := time.LoadLocation(zones[0])
	if err != nil {
		return nil
	}
	return loc
}

// greetingWord picks the time-of-day salutation for the given local hour
// (0-23): morning 5am-noon, afternoon noon-5pm, evening 5pm-9pm, night
// otherwise (9pm-5am) — late-night/early-morning hours get "Good night"
// instead of stretching "Good evening"/"Good morning" across them.
func greetingWord(hour int) string {
	switch {
	case hour >= 5 && hour < 12:
		return "Good morning"
	case hour >= 12 && hour < 17:
		return "Good afternoon"
	case hour >= 17 && hour < 21:
		return "Good evening"
	default:
		return "Good night"
	}
}
