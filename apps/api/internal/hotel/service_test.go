package hotel

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateSlug(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		// valid
		{"min length", "abc", nil},
		{"hyphenated", "zen-hostel-chiangmai", nil},
		{"digits", "hotel123", nil},
		{"digits-only", "12345", nil},
		{"max length boundary", strings.Repeat("a", 80), nil},

		// invalid
		{"too short", "ab", ErrInvalidSlug},
		{"empty", "", ErrInvalidSlug},
		{"too long", strings.Repeat("a", 81), ErrInvalidSlug},
		{"uppercase", "ZenHostel", ErrInvalidSlug},
		{"leading hyphen", "-foo", ErrInvalidSlug},
		{"trailing hyphen", "foo-", ErrInvalidSlug},
		{"consecutive hyphens", "foo--bar", ErrInvalidSlug},
		{"space", "foo bar", ErrInvalidSlug},
		{"underscore", "foo_bar", ErrInvalidSlug},
		{"unicode", "café", ErrInvalidSlug},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := validateSlug(c.in)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{"single char", "A", nil},
		{"normal", "Zen Hostel", nil},
		{"thai", "โรงแรมเซน", nil},
		{"max length", strings.Repeat("a", 255), nil},

		{"empty", "", ErrInvalidName},
		{"too long", strings.Repeat("a", 256), ErrInvalidName},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := validateName(c.in)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestNormalisePromptPayID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		// valid
		{"10-digit phone", "0812345678", "0812345678", nil},
		{"10-digit phone with dashes", "081-234-5678", "0812345678", nil},
		{"10-digit phone with spaces", "081 234 5678", "0812345678", nil},
		{"13-digit national ID", "1234567890123", "1234567890123", nil},
		{"13-digit national ID dashed", "1-2345-67890-12-3", "1234567890123", nil},
		{"15-char tax ID (e-wallet)", "012345678901234", "012345678901234", nil},
		{"15-char tax ID dashed", "0-1234-56789-01-1-23", "012345678901123", nil},
		{"leading/trailing whitespace", "  0812345678  ", "0812345678", nil},

		// invalid
		{"too short", "081234", "", ErrInvalidPromptPayID},
		{"too long (16 digits)", "1234567890123456", "", ErrInvalidPromptPayID},
		{"letters", "081abc5678", "", ErrInvalidPromptPayID},
		{"15-char not starting with 0", "112345678901234", "", ErrInvalidPromptPayID},
		{"empty", "", "", ErrInvalidPromptPayID},
		{"only whitespace", "   ", "", ErrInvalidPromptPayID},
		{"11-digit (not 10/13/15)", "08123456789", "", ErrInvalidPromptPayID},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalisePromptPayID(c.in)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("want err %v, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != c.want {
				t.Fatalf("want %q, got %q", c.want, got)
			}
		})
	}
}

func TestPtrOrNil(t *testing.T) {
	t.Parallel()
	if got := ptrOrNil[string](nil); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
	s := "hello"
	if got := ptrOrNil(&s); got != "hello" {
		t.Fatalf("want %q, got %v", "hello", got)
	}
	n := 42
	if got := ptrOrNil(&n); got != 42 {
		t.Fatalf("want %d, got %v", 42, got)
	}
}
