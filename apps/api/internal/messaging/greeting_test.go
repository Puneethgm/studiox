package messaging

import (
	"testing"
	"time"
)

func TestGreetingWord(t *testing.T) {
	cases := []struct {
		hour int
		want string
	}{
		{0, "Good night"},
		{4, "Good night"},
		{5, "Good morning"},
		{9, "Good morning"},
		{11, "Good morning"},
		{12, "Good afternoon"},
		{16, "Good afternoon"},
		{17, "Good evening"},
		{20, "Good evening"},
		{21, "Good night"},
		{23, "Good night"},
	}
	for _, c := range cases {
		if got := greetingWord(c.hour); got != c.want {
			t.Errorf("greetingWord(%d) = %q, want %q", c.hour, got, c.want)
		}
	}
}

func TestTimezoneForPhone(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string // "" means expect nil
	}{
		{"india wa_id, no plus (WhatsApp Cloud API format)", "919876543210", "Asia/Calcutta"},
		{"singapore, with plus", "+6591234567", "Asia/Singapore"},
		{"us number", "+14155552671", "America/Los_Angeles"},
		{"empty", "", ""},
		{"not a number", "notanumber", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loc := timezoneForPhone(c.raw)
			if c.want == "" {
				if loc != nil {
					t.Errorf("timezoneForPhone(%q) = %v, want nil", c.raw, loc)
				}
				return
			}
			if loc == nil || loc.String() != c.want {
				t.Errorf("timezoneForPhone(%q) = %v, want %q", c.raw, loc, c.want)
			}
		})
	}
}

func TestGreetingLocation(t *testing.T) {
	studioLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load studio location: %v", err)
	}

	t.Run("whatsapp conversation uses the recipient's own country", func(t *testing.T) {
		conv := &Conversation{ChannelKind: KindWhatsAppMeta, ContactValue: "919876543210"}
		loc := greetingLocation(conv, studioLoc)
		if loc.String() != "Asia/Calcutta" {
			t.Errorf("got %v, want Asia/Calcutta", loc)
		}
	})

	t.Run("nil conversation falls back to the studio's timezone (test-chat with no phone)", func(t *testing.T) {
		loc := greetingLocation(nil, studioLoc)
		if loc != studioLoc {
			t.Errorf("got %v, want studioLoc %v", loc, studioLoc)
		}
	})

	t.Run("non-phone channel (Telegram chat ID) falls back to the studio's timezone", func(t *testing.T) {
		conv := &Conversation{ChannelKind: KindTelegram, ContactValue: "123456789"}
		loc := greetingLocation(conv, studioLoc)
		if loc != studioLoc {
			t.Errorf("got %v, want studioLoc %v", loc, studioLoc)
		}
	})

	t.Run("whatsapp conversation with an unparseable number falls back to the studio's timezone", func(t *testing.T) {
		conv := &Conversation{ChannelKind: KindWhatsAppMeta, ContactValue: "not-a-phone"}
		loc := greetingLocation(conv, studioLoc)
		if loc != studioLoc {
			t.Errorf("got %v, want studioLoc %v", loc, studioLoc)
		}
	})
}

func TestResolveGreetingLocation(t *testing.T) {
	studioLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load studio location: %v", err)
	}

	t.Run("test-chat override wins even when a phone-based conversation is also present", func(t *testing.T) {
		conv := &Conversation{ChannelKind: KindWhatsAppMeta, ContactValue: "919876543210"} // would resolve to Asia/Calcutta
		loc := resolveGreetingLocation("Asia/Tokyo", conv, studioLoc)
		if loc.String() != "Asia/Tokyo" {
			t.Errorf("got %v, want Asia/Tokyo (override should win)", loc)
		}
	})

	t.Run("empty override falls through to phone-based resolution", func(t *testing.T) {
		conv := &Conversation{ChannelKind: KindWhatsAppMeta, ContactValue: "919876543210"}
		loc := resolveGreetingLocation("", conv, studioLoc)
		if loc.String() != "Asia/Calcutta" {
			t.Errorf("got %v, want Asia/Calcutta", loc)
		}
	})

	t.Run("unrecognized override falls back to studio timezone (no conversation, test-chat)", func(t *testing.T) {
		loc := resolveGreetingLocation("Not/AZone", nil, studioLoc)
		if loc != studioLoc {
			t.Errorf("got %v, want studioLoc %v", loc, studioLoc)
		}
	})

	t.Run("valid override with no conversation (typical test-chat call)", func(t *testing.T) {
		loc := resolveGreetingLocation("Asia/Kolkata", nil, studioLoc)
		if loc.String() != "Asia/Kolkata" {
			t.Errorf("got %v, want Asia/Kolkata", loc)
		}
	})
}
