package auth

import (
	"strings"
	"testing"
)

// Argon2id hashing is intentionally slow (~100ms+). We minimize the number of
// HashPassword calls by sharing one hash across multiple sub-tests via t.Run.

func TestPassword(t *testing.T) {
	const pw = "correct horse battery staple 9"

	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: unexpected error: %v", err)
	}

	t.Run("PHC prefix", func(t *testing.T) {
		if !strings.HasPrefix(hash, "$argon2id$v=19$") {
			t.Errorf("hash missing PHC prefix: %q", hash)
		}
	})

	t.Run("six PHC segments", func(t *testing.T) {
		// PHC string: $argon2id$v=19$m=...,t=...,p=...$<salt>$<hash>
		// strings.Split on "$" yields 6 parts (leading empty + 5 segments).
		parts := strings.Split(hash, "$")
		if len(parts) != 6 {
			t.Errorf("expected 6 split segments, got %d: %v", len(parts), parts)
		}
	})

	t.Run("salt is random across calls", func(t *testing.T) {
		other, err := HashPassword(pw)
		if err != nil {
			t.Fatalf("HashPassword second call: %v", err)
		}
		if other == hash {
			t.Errorf("expected different hashes for repeated calls (random salt), got identical")
		}
	})

	t.Run("verify correct password", func(t *testing.T) {
		ok, err := VerifyPassword(hash, pw)
		if err != nil {
			t.Fatalf("VerifyPassword: unexpected error: %v", err)
		}
		if !ok {
			t.Errorf("expected verify=true for correct password")
		}
	})

	t.Run("verify wrong password", func(t *testing.T) {
		ok, err := VerifyPassword(hash, pw+"!")
		if err != nil {
			t.Fatalf("VerifyPassword: unexpected error: %v", err)
		}
		if ok {
			t.Errorf("expected verify=false for wrong password")
		}
	})
}

func TestVerifyPasswordMalformed(t *testing.T) {
	cases := []struct {
		name    string
		encoded string
	}{
		{"empty", ""},
		{"not enough segments", "$argon2id$v=19$m=65536,t=3,p=4$salt"},
		{"wrong algo", "$argon2i$v=19$m=65536,t=3,p=4$c2FsdHNhbHQ$aGFzaA"},
		{"junk version", "$argon2id$vXX$m=65536,t=3,p=4$c2FsdHNhbHQ$aGFzaA"},
		{"junk params", "$argon2id$v=19$mmm$c2FsdHNhbHQ$aGFzaA"},
		{"bad salt b64", "$argon2id$v=19$m=65536,t=3,p=4$!!!!$aGFzaA"},
		{"bad hash b64", "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHQ$!!!!"},
		{"incompatible version", "$argon2id$v=99$m=65536,t=3,p=4$c2FsdHNhbHQ$aGFzaA"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ok, err := VerifyPassword(tc.encoded, "whatever")
			if err == nil {
				t.Errorf("expected error for malformed hash %q", tc.encoded)
			}
			if ok {
				t.Errorf("expected ok=false for malformed hash %q", tc.encoded)
			}
		})
	}
}
