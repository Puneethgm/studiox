package identity

import (
	"context"
	"errors"
	"testing"
)

type fakeBreachChecker struct {
	breached map[string]bool
	err      error
}

func (f *fakeBreachChecker) IsBreached(ctx context.Context, password string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.breached[password], nil
}

func TestCheckPasswordPolicy_LengthBoundary(t *testing.T) {
	h := &Handler{}
	if errs := h.checkPasswordPolicy(context.Background(), "elevenchars"); errs["newPassword"] == "" {
		t.Error("11 chars should fail the 12-char minimum")
	}
	if errs := h.checkPasswordPolicy(context.Background(), "twelvechars!"); errs["newPassword"] != "" {
		t.Errorf("exactly 12 chars should pass, got error: %v", errs)
	}
}

func TestCheckPasswordPolicy_BreachedPasswordRejected(t *testing.T) {
	h := &Handler{breachChecker: &fakeBreachChecker{breached: map[string]bool{"password12345": true}}}
	errs := h.checkPasswordPolicy(context.Background(), "password12345")
	if errs["newPassword"] == "" {
		t.Error("expected a known-breached password to be rejected")
	}
}

func TestCheckPasswordPolicy_NonBreachedPasswordAccepted(t *testing.T) {
	h := &Handler{breachChecker: &fakeBreachChecker{breached: map[string]bool{"password12345": true}}}
	errs := h.checkPasswordPolicy(context.Background(), "xk7q9mz4p2wrt8")
	if errs["newPassword"] != "" {
		t.Errorf("expected a random password to be accepted, got: %v", errs)
	}
}

func TestCheckPasswordPolicy_BreachCheckFailsOpen(t *testing.T) {
	h := &Handler{breachChecker: &fakeBreachChecker{err: errors.New("HIBP unreachable")}}
	errs := h.checkPasswordPolicy(context.Background(), "someLongEnoughPassword")
	if errs["newPassword"] != "" {
		t.Errorf("a breach-check error must fail open (not reject), got: %v", errs)
	}
}

func TestCheckPasswordPolicy_NilCheckerSkipsBreachCheck(t *testing.T) {
	h := &Handler{breachChecker: nil}
	errs := h.checkPasswordPolicy(context.Background(), "someLongEnoughPassword")
	if errs["newPassword"] != "" {
		t.Errorf("nil breachChecker should skip the check entirely, got: %v", errs)
	}
}
