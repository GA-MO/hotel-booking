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
