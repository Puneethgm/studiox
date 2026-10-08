package identity

import (
	"strings"
	"testing"
)

func TestGenerateTempPassword(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		pw, err := GenerateTempPassword()
		if err != nil {
			t.Fatal(err)
		}
		if len(pw) != tempPasswordLength {
			t.Fatalf("length = %d, want %d", len(pw), tempPasswordLength)
		}
		for _, c := range pw {
			if !strings.ContainsRune(tempPasswordAlphabet, c) {
				t.Fatalf("password %q contains %q outside the alphabet", pw, c)
			}
		}
		if seen[pw] {
			t.Fatalf("duplicate temp password %q in 200 draws", pw)
		}
		seen[pw] = true
	}
	// The retired shared default must never be what we hand out.
	if pw, _ := GenerateTempPassword(); pw == "password123" {
		t.Fatal("generated the old fixed default")
	}
}
