package messaging

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"syscall"
	"testing"
	"time"
)

func TestIsTransientNetworkError(t *testing.T) {
	// The exact failure seen on prod: the API container couldn't resolve "wa-web".
	dns := &url.Error{Op: "Post", URL: "http://wa-web:3100/sessions/x/send", Err: &net.OpError{
		Op: "dial", Net: "tcp", Err: &net.DNSError{Err: "server misbehaving", Name: "wa-web", Server: "127.0.0.11:53"},
	}}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"dns lookup failure", dns, true},
		{"wrapped dns failure", fmt.Errorf("send: %w", dns), true},
		{"connection refused", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}, true},
		{"connection reset", fmt.Errorf("read: %w", syscall.ECONNRESET), true},
		{"timeout", &url.Error{Op: "Post", URL: "u", Err: context.DeadlineExceeded}, true},
		{"app error from wa-web", errors.New("wa-web send failed 500: session not connected"), false},
		{"rejected number", errors.New("number 65123 is not registered on WhatsApp"), false},
		{"shutdown", context.Canceled, false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		if got := isTransientNetworkError(c.err); got != c.want {
			t.Errorf("%s: isTransientNetworkError = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestTransientBackoff(t *testing.T) {
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 10 * time.Minute, 10 * time.Minute}
	for i, w := range want {
		if got := transientBackoffFor(i + 1); got != w {
			t.Errorf("transientBackoffFor(%d) = %v, want %v", i+1, got, w)
		}
	}
	if got := transientBackoffFor(60); got != maxTransientBackoff {
		t.Errorf("huge attempt count = %v, want the cap %v", got, maxTransientBackoff)
	}
	// The whole transient retry window must be far longer than the ~1 minute that
	// killed the broadcast: at least an hour.
	var total time.Duration
	for a := 1; a < maxTransientAttempts; a++ {
		total += transientBackoffFor(a)
	}
	if total < time.Hour {
		t.Errorf("transient retry window = %v, want at least 1h", total)
	}
}

// Regression for the prod flood: limit 48 but 124 recipients were queued, because only
// *sent* messages were subtracted and sends are spaced 60s apart. Queued-but-unsent
// messages must count against the day's allowance.
func TestBroadcastAllowance(t *testing.T) {
	cases := []struct {
		name                        string
		limit, sent, pending, batch int
		want                        int
	}{
		{"fresh day takes the whole limit", 48, 0, 0, 200, 48},
		{"capped by the per-tick batch", 48, 0, 0, 10, 10},
		{"partly used by sent messages", 48, 27, 0, 200, 21},
		{"queued messages count as used", 48, 27, 21, 200, 0},
		{"queued alone can exhaust it", 48, 0, 48, 200, 0},
		{"over-subscribed never negative", 48, 40, 30, 200, 0},
		{"unlimited ignores sent and pending", 0, 500, 500, 200, 200},
	}
	for _, c := range cases {
		if got := broadcastAllowance(c.limit, c.sent, c.pending, c.batch); got != c.want {
			t.Errorf("%s: broadcastAllowance(%d,%d,%d,%d) = %d, want %d", c.name, c.limit, c.sent, c.pending, c.batch, got, c.want)
		}
	}

	// Simulate the prod run: 124 recipients, limit 48, 27 already sent today, one send
	// per tick completes (60s spacing). With the old rule the queue grew by ~20 each tick.
	limit, sent, pending, queuedTotal := 48, 27, 0, 0
	for tick := 0; tick < 10; tick++ {
		n := broadcastAllowance(limit, sent, pending, 200)
		queuedTotal += n
		pending += n
		if pending > 0 { // one queued message gets sent per tick
			pending--
			sent++
		}
	}
	if queuedTotal > limit-27 {
		t.Errorf("queued %d messages over 10 ticks, but only %d fit in today's allowance", queuedTotal, limit-27)
	}
}

func TestIsRetryableBroadcastFailure(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		// temporary: put the recipient back in the queue
		{"daily_limit_exceeded", true},
		{`Post "http://wa-web:3100/sessions/x/send": dial tcp: lookup wa-web on 127.0.0.11:53: server misbehaving`, true},
		{"no active channel: disconnected", true},
		{"wa-web send failed 500: {\"error\":\"session not connected for studio x\"}", true},
		{"read tcp 1.2.3.4:5: connection reset by peer", true},
		// permanent: stays failed
		{`wa-web send failed 500: {"error":"number 6596558135 is not registered on WhatsApp"}`, false},
		{"dnd enabled — automated send blocked", false},
		{"credentials: invalid token", false},
		{"channel lookup: not found", false},
		{"no sender for channel kind: google_ads", false},
		{"something unexpected", false},
	}
	for _, c := range cases {
		if got := isRetryableBroadcastFailure(c.msg); got != c.want {
			t.Errorf("isRetryableBroadcastFailure(%q) = %v, want %v", c.msg, got, c.want)
		}
	}
}
