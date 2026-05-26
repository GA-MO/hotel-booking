package auth

import (
	"encoding/hex"
	"testing"
)

func TestGenerateRefreshToken(t *testing.T) {
	tok, err := generateRefreshToken()
	if err != nil {
		t.Fatalf("generateRefreshToken: %v", err)
	}
	if len(tok) != 64 {
		t.Errorf("expected 64-char hex string (32 bytes), got len=%d (%q)", len(tok), tok)
	}
	if _, err := hex.DecodeString(tok); err != nil {
		t.Errorf("expected valid hex, got %q: %v", tok, err)
	}

	other, err := generateRefreshToken()
	if err != nil {
		t.Fatalf("generateRefreshToken second call: %v", err)
	}
	if other == tok {
		t.Errorf("expected two random tokens to differ; both = %q", tok)
	}
}

func TestHashToken(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		a := hashToken("some-refresh-token")
		b := hashToken("some-refresh-token")
		if a != b {
			t.Errorf("hashToken not deterministic: %q vs %q", a, b)
		}
	})

	t.Run("different input -> different output", func(t *testing.T) {
		a := hashToken("alpha")
		b := hashToken("beta")
		if a == b {
			t.Errorf("hashToken collision on distinct inputs: %q", a)
		}
	})

	t.Run("64-char hex output", func(t *testing.T) {
		// SHA-256 -> 32 bytes -> 64 hex chars.
		cases := []string{"", "x", "longer input string for sha256"}
		for _, in := range cases {
			out := hashToken(in)
			if len(out) != 64 {
				t.Errorf("hashToken(%q): expected len=64, got %d", in, len(out))
			}
			if _, err := hex.DecodeString(out); err != nil {
				t.Errorf("hashToken(%q): not valid hex: %v", in, err)
			}
		}
	})
}
