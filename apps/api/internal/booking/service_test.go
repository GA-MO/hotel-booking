package booking

import (
	"errors"
	"testing"
	"time"
)

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"plain", "guest@example.com", "guest@example.com", false},
		{"upper trimmed", "  GUEST@Example.COM ", "guest@example.com", false},
		{"plus tag", "g+sale@example.com", "g+sale@example.com", false},
		{"empty", "", "", true},
		{"no at", "guest.example.com", "", true},
		{"malformed", "guest@@example.com", "", true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeEmail(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil")
				}
				if !errors.Is(err, ErrInvalidGuest) {
					t.Fatalf("want ErrInvalidGuest, got %v", err)
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

func TestParseDateRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in, out string
		wantErr bool
		wantNights int
	}{
		{"one night", "2026-06-01", "2026-06-02", false, 1},
		{"seven nights", "2026-06-01", "2026-06-08", false, 7},
		{"cross month", "2026-06-30", "2026-07-02", false, 2},
		{"cross year", "2026-12-30", "2027-01-02", false, 3},
		{"trimmed", "  2026-06-01 ", " 2026-06-02 ", false, 1},

		{"same day", "2026-06-01", "2026-06-01", true, 0},
		{"reversed", "2026-06-02", "2026-06-01", true, 0},
		{"bad format", "2026/06/01", "2026-06-02", true, 0},
		{"empty in", "", "2026-06-02", true, 0},
		{"empty out", "2026-06-01", "", true, 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ci, co, err := parseDateRange(c.in, c.out)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil")
				}
				if !errors.Is(err, ErrInvalidDates) {
					t.Fatalf("want ErrInvalidDates, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			nights := int(co.Sub(ci).Hours() / 24)
			if nights != c.wantNights {
				t.Fatalf("nights: want %d, got %d", c.wantNights, nights)
			}
		})
	}
}

func TestParseDateRange_AlwaysUTCDate(t *testing.T) {
	t.Parallel()
	ci, _, err := parseDateRange("2026-06-01", "2026-06-02")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// time.Parse with the "2006-01-02" layout returns a time with location=UTC
	// and a zero clock — this is the contract callers rely on for storage.
	if ci.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %v", ci.Location())
	}
	h, m, s := ci.Clock()
	if h != 0 || m != 0 || s != 0 {
		t.Fatalf("expected midnight, got %02d:%02d:%02d", h, m, s)
	}
}
