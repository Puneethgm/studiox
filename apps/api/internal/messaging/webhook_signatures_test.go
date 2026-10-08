package messaging

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"testing"
)

// Twilio's validation example parameters; the expected signature was computed independently with
// `openssl dgst -sha1 -hmac 12345 -binary | base64` over url + sorted name+value pairs.
func TestTwilioSignatureValid(t *testing.T) {
	params := url.Values{
		"CallSid": {"CA1234567890ABCDE"},
		"Caller":  {"+12349013030"},
		"Digits":  {"1234"},
		"From":    {"+12349013030"},
		"To":      {"+18005551212"},
	}
	const (
		token = "12345"
		u     = "https://mycompany.com/myapp.php?foo=1&bar=2"
		sig   = "0/KCTR6DLpKmkAf8muzZqo1nDgQ=" // computed independently with openssl
	)

	if !twilioSignatureValid(token, sig, []string{u}, params) {
		t.Fatal("valid signature rejected")
	}
	// matches when any one candidate URL is right (proxy vs. public URL)
	if !twilioSignatureValid(token, sig, []string{"http://internal/x", u}, params) {
		t.Fatal("valid signature rejected when it matches the second candidate URL")
	}
	if twilioSignatureValid("wrong-token", sig, []string{u}, params) {
		t.Fatal("wrong auth token accepted")
	}
	tampered := url.Values{}
	for k, v := range params {
		tampered[k] = v
	}
	tampered.Set("Digits", "9999")
	if twilioSignatureValid(token, sig, []string{u}, tampered) {
		t.Fatal("tampered body accepted")
	}
	if twilioSignatureValid(token, "", []string{u}, params) || twilioSignatureValid("", sig, []string{u}, params) {
		t.Fatal("empty signature or token accepted")
	}
}

func TestXSignatureValid(t *testing.T) {
	body := []byte(`{"for_user_id":"1","direct_message_events":[]}`)
	secret := "consumer-secret"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	header := "sha256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !xSignatureValid(header, body, []string{"other", secret}) {
		t.Fatal("valid signature rejected")
	}
	if xSignatureValid(header, []byte(`{"tampered":true}`), []string{secret}) {
		t.Fatal("tampered body accepted")
	}
	if xSignatureValid(header, body, []string{"other"}) {
		t.Fatal("wrong secret accepted")
	}
	if xSignatureValid("", body, []string{secret}) || xSignatureValid("sha256=", body, []string{secret}) || xSignatureValid(header, body, nil) {
		t.Fatal("missing header or no secrets accepted")
	}
}

func TestVerifyMetaSignedRequest(t *testing.T) {
	secret := "app-secret"
	payload, _ := json.Marshal(map[string]string{"algorithm": "HMAC-SHA256", "user_id": "12345"})
	p := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(p))
	signed := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) + "." + p

	if uid, ok := verifyMetaSignedRequest(signed, secret); !ok || uid != "12345" {
		t.Fatalf("valid signed request rejected: uid=%q ok=%v", uid, ok)
	}
	if _, ok := verifyMetaSignedRequest(signed, "wrong"); ok {
		t.Fatal("wrong secret accepted")
	}
	if _, ok := verifyMetaSignedRequest(signed, ""); ok {
		t.Fatal("empty secret accepted")
	}
	forged := base64.RawURLEncoding.EncodeToString([]byte("not-the-mac")) + "." + p
	if _, ok := verifyMetaSignedRequest(forged, secret); ok {
		t.Fatal("forged signature accepted")
	}
	if _, ok := verifyMetaSignedRequest("garbage", secret); ok {
		t.Fatal("malformed accepted")
	}
	// payload altered after signing
	altered, _ := json.Marshal(map[string]string{"algorithm": "HMAC-SHA256", "user_id": "666"})
	badPayload := signed[:len(signed)-len(p)] + base64.RawURLEncoding.EncodeToString(altered)
	if _, ok := verifyMetaSignedRequest(badPayload, secret); ok {
		t.Fatal("altered payload accepted")
	}
}
