package secrets

import (
	"encoding/base64"
	"strings"
	"testing"
)

func testMasterKey(t *testing.T) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(make([]byte, RequiredKeyBytes))
}

func TestEnvelopeEncryption_RoundTrip(t *testing.T) {
	dataKey, err := GenerateDataKey()
	if err != nil {
		t.Fatalf("GenerateDataKey: %v", err)
	}
	if dataKey == "" {
		t.Fatal("GenerateDataKey returned empty key")
	}

	plaintext := "sk-live-super-secret-provider-key-12345"
	ciphertext, err := EncryptWithDataKey(dataKey, plaintext)
	if err != nil {
		t.Fatalf("EncryptWithDataKey: %v", err)
	}
	if ciphertext == plaintext || strings.Contains(ciphertext, plaintext) {
		t.Fatal("ciphertext must not contain the plaintext")
	}

	got, err := DecryptWithDataKey(dataKey, ciphertext)
	if err != nil {
		t.Fatalf("DecryptWithDataKey: %v", err)
	}
	if got != plaintext {
		t.Errorf("round trip = %q, want %q", got, plaintext)
	}
}

// TestEnvelopeEncryption_TwoDataKeysAreIndependent proves the core envelope
// property: compromising one secret's data key doesn't expose any other
// secret's data key or content.
func TestEnvelopeEncryption_TwoDataKeysAreIndependent(t *testing.T) {
	dataKeyA, err := GenerateDataKey()
	if err != nil {
		t.Fatalf("GenerateDataKey A: %v", err)
	}
	dataKeyB, err := GenerateDataKey()
	if err != nil {
		t.Fatalf("GenerateDataKey B: %v", err)
	}
	if dataKeyA == dataKeyB {
		t.Fatal("two calls to GenerateDataKey produced the same key")
	}

	ciphertextA, err := EncryptWithDataKey(dataKeyA, "secret-a")
	if err != nil {
		t.Fatalf("encrypt a: %v", err)
	}

	if _, err := DecryptWithDataKey(dataKeyB, ciphertextA); err == nil {
		t.Error("decrypting secret A's ciphertext with data key B should fail, but it succeeded")
	}
}

// TestEnvelopeEncryption_MasterKeyWrapsDataKey exercises the full two-layer
// scheme as it's actually used: a per-secret data key encrypts the secret,
// and the master Cipher (from TOKEN_ENCRYPTION_KEY) encrypts the data key
// itself — the master key is the only thing that must stay outside the
// database, per design.md Decision 4.
func TestEnvelopeEncryption_MasterKeyWrapsDataKey(t *testing.T) {
	master, err := New(testMasterKey(t))
	if err != nil {
		t.Fatalf("New(master): %v", err)
	}

	dataKey, err := GenerateDataKey()
	if err != nil {
		t.Fatalf("GenerateDataKey: %v", err)
	}
	providerKey := "sk-live-abcdef0123456789wxyz"

	keyCiphertext, err := EncryptWithDataKey(dataKey, providerKey)
	if err != nil {
		t.Fatalf("EncryptWithDataKey: %v", err)
	}
	dataKeyCiphertext, err := master.Encrypt(dataKey)
	if err != nil {
		t.Fatalf("master.Encrypt(dataKey): %v", err)
	}

	// Simulate a read: only the two ciphertexts are "in the row"; dataKey
	// itself is never persisted anywhere.
	recoveredDataKey, err := master.Decrypt(dataKeyCiphertext)
	if err != nil {
		t.Fatalf("master.Decrypt(dataKeyCiphertext): %v", err)
	}
	if recoveredDataKey != dataKey {
		t.Fatalf("recovered data key = %q, want %q", recoveredDataKey, dataKey)
	}
	recoveredProviderKey, err := DecryptWithDataKey(recoveredDataKey, keyCiphertext)
	if err != nil {
		t.Fatalf("DecryptWithDataKey(recoveredDataKey, ...): %v", err)
	}
	if recoveredProviderKey != providerKey {
		t.Errorf("recovered provider key = %q, want %q", recoveredProviderKey, providerKey)
	}
}

func TestEnvelopeEncryption_TamperedCiphertextFailsToDecrypt(t *testing.T) {
	dataKey, err := GenerateDataKey()
	if err != nil {
		t.Fatalf("GenerateDataKey: %v", err)
	}
	ciphertext, err := EncryptWithDataKey(dataKey, "sk-live-value")
	if err != nil {
		t.Fatalf("EncryptWithDataKey: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatalf("decode ciphertext: %v", err)
	}
	raw[len(raw)-1] ^= 0xFF // flip the last byte (inside the GCM tag)
	tampered := base64.StdEncoding.EncodeToString(raw)

	if _, err := DecryptWithDataKey(dataKey, tampered); err == nil {
		t.Error("decrypting a tampered ciphertext should fail, but it succeeded")
	}
}

func TestEnvelopeEncryption_GenerateDataKeyIsUniquePerCall(t *testing.T) {
	seen := make(map[string]bool, 20)
	for i := 0; i < 20; i++ {
		k, err := GenerateDataKey()
		if err != nil {
			t.Fatalf("GenerateDataKey: %v", err)
		}
		if seen[k] {
			t.Fatalf("GenerateDataKey produced a duplicate key on call %d", i)
		}
		seen[k] = true
	}
}
