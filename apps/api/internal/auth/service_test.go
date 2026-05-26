package auth

import (
	"errors"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cases := []struct {
			in, want string
		}{
			{"foo@bar.com", "foo@bar.com"},
			{"  Foo@Bar.com ", "foo@bar.com"},
			{"User.Name+tag@Example.CO", "user.name+tag@example.co"},
			{"\tA@B.io\n", "a@b.io"},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.in, func(t *testing.T) {
				got, err := normalizeEmail(tc.in)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Errorf("normalizeEmail(%q): got %q want %q", tc.in, got, tc.want)
				}
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		cases := []string{
			"",
			"   ",
			"no-at-sign",
			"@no-local.com",
			"trailing@",
			"two@@signs.com",
			"spaces in@local.com",
		}
		for _, in := range cases {
			in := in
			t.Run(in, func(t *testing.T) {
				_, err := normalizeEmail(in)
				if err == nil {
					t.Errorf("expected error for %q, got nil", in)
					return
				}
				if !errors.Is(err, ErrInvalidEmail) {
					t.Errorf("expected ErrInvalidEmail, got %v", err)
				}
			})
		}
	})
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name    string
		pw      string
		wantErr bool
	}{
		{"too short", "a1b2c3", true},
		{"empty", "", true},
		{"letters only", "abcdefghij", true},
		{"digits only", "1234567890", true},
		{"exactly 8 with letter+digit", "abcdefg1", false},
		{"long mixed", "correct horse battery staple 9", false},
		{"unicode letter + digit", "résumé99", false},
		{"symbols only", "!@#$%^&*()", true},
		{"letter+symbol (no digit)", "abcdefg!@", true},
		{"digit+symbol (no letter)", "1234567!@", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validatePassword(tc.pw)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tc.pw)
					return
				}
				if !errors.Is(err, ErrPasswordTooWeak) {
					t.Errorf("expected ErrPasswordTooWeak, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected nil error for %q, got %v", tc.pw, err)
				}
			}
		})
	}
}
