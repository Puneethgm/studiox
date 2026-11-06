package identity

import "context"

type fakeCaptchaVerifier struct {
	validTokens map[string]bool
	err         error
}

func (f *fakeCaptchaVerifier) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.validTokens[token], nil
}
