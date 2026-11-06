package identity

import "testing"

func TestParseUserAgent(t *testing.T) {
	cases := []struct {
		ua             string
		wantDevice     string
		wantBrowserHas string
	}{
		{
			ua:             "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36",
			wantDevice:     "Linux",
			wantBrowserHas: "Chrome 145",
		},
		{
			ua:             "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			wantDevice:     "iPhone",
			wantBrowserHas: "Safari 604",
		},
		{
			ua:             "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			wantDevice:     "Windows",
			wantBrowserHas: "Edge 120",
		},
		{
			ua:             "",
			wantDevice:     "",
			wantBrowserHas: "",
		},
	}

	for _, c := range cases {
		device, browser := parseUserAgent(c.ua)
		if device != c.wantDevice {
			t.Errorf("parseUserAgent(%q) device = %q, want %q", c.ua, device, c.wantDevice)
		}
		if browser != c.wantBrowserHas {
			t.Errorf("parseUserAgent(%q) browser = %q, want %q", c.ua, browser, c.wantBrowserHas)
		}
	}
}
