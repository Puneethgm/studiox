package studios

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/projectx/api/internal/platform/secrets"
)

// TestArchiveEncryptDecryptRoundTrip exercises EncryptArchive/DecryptArchive
// purely in-memory — no DB, no S3 — to prove the envelope-encryption round
// trip actually works: encrypt with a random data key wrapped by a master
// cipher, then decrypt back to the exact original bytes.
func TestArchiveEncryptDecryptRoundTrip(t *testing.T) {
	keyBytes := make([]byte, secrets.RequiredKeyBytes)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatalf("generate master key: %v", err)
	}
	masterCipher, err := secrets.New(base64.StdEncoding.EncodeToString(keyBytes))
	if err != nil {
		t.Fatalf("new master cipher: %v", err)
	}

	// Deliberately includes non-UTF8 bytes to prove the round trip survives
	// arbitrary binary content (a real archive is a zip file, not text).
	original := []byte("fake zip payload \x00\x01\x02 with binary \xff\xfe bytes and unicode café")

	encrypted, wrappedKey, err := EncryptArchive(original, masterCipher)
	if err != nil {
		t.Fatalf("EncryptArchive: %v", err)
	}
	if wrappedKey == "" {
		t.Fatal("expected a non-empty wrapped key")
	}
	if string(encrypted) == string(original) {
		t.Fatal("encrypted output must not equal the plaintext")
	}

	decrypted, err := DecryptArchive(encrypted, wrappedKey, masterCipher)
	if err != nil {
		t.Fatalf("DecryptArchive: %v", err)
	}
	if string(decrypted) != string(original) {
		t.Fatalf("round trip mismatch:\n got:  %q\n want: %q", decrypted, original)
	}

	// A wrong wrapped key must not decrypt (proves the wrap/unwrap step is
	// actually load-bearing, not a no-op).
	if _, err := DecryptArchive(encrypted, "bm90LWEtcmVhbC13cmFwcGVkLWtleQ==", masterCipher); err == nil {
		t.Fatal("expected an error decrypting with an invalid wrapped key")
	}

	// A different master cipher must not be able to unwrap the data key
	// either (proves the master key is actually load-bearing).
	otherKeyBytes := make([]byte, secrets.RequiredKeyBytes)
	if _, err := rand.Read(otherKeyBytes); err != nil {
		t.Fatalf("generate other master key: %v", err)
	}
	otherMasterCipher, err := secrets.New(base64.StdEncoding.EncodeToString(otherKeyBytes))
	if err != nil {
		t.Fatalf("new other master cipher: %v", err)
	}
	if _, err := DecryptArchive(encrypted, wrappedKey, otherMasterCipher); err == nil {
		t.Fatal("expected an error decrypting a wrapped key with the wrong master cipher")
	}
}
