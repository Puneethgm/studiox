package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		remote  string
		want    string
	}{
		{
			name:    "X-Forwarded-For IPv4-mapped IPv6 collapses to IPv4",
			headers: map[string]string{"X-Forwarded-For": "::ffff:100.122.229.3"},
			want:    "100.122.229.3",
		},
		{
			name:    "X-Forwarded-For plain IPv4",
			headers: map[string]string{"X-Forwarded-For": "203.0.113.5"},
			want:    "203.0.113.5",
		},
		{
			name:    "X-Forwarded-For takes the first of a list",
			headers: map[string]string{"X-Forwarded-For": "203.0.113.5, 10.0.0.1"},
			want:    "203.0.113.5",
		},
		{
			name:    "X-Real-IP used when X-Forwarded-For absent",
			headers: map[string]string{"X-Real-IP": "198.51.100.7"},
			want:    "198.51.100.7",
		},
		{
			name:   "RemoteAddr port is stripped",
			remote: "172.20.0.5:54321",
			want:   "172.20.0.5",
		},
		{
			name:   "RemoteAddr bare IPv6 without port is left intact",
			remote: "::1",
			want:   "::1",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, v := range c.headers {
				r.Header.Set(k, v)
			}
			if c.remote != "" {
				r.RemoteAddr = c.remote
			}
			got := ClientIP(r)
			if got != c.want {
				t.Errorf("ClientIP() = %q, want %q", got, c.want)
			}
		})
	}
}
