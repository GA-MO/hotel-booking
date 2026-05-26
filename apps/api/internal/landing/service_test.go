package landing

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateLocale(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		// valid
		{"two lower", "th", nil},
		{"two lower en", "en", nil},
		{"with region", "en-US", nil},
		{"with region th-TH", "th-TH", nil},

		// invalid
		{"empty", "", ErrInvalidLocale},
		{"single char", "e", ErrInvalidLocale},
		{"three chars", "eng", ErrInvalidLocale},
		{"upper lang", "EN", ErrInvalidLocale},
		{"lower region", "en-us", ErrInvalidLocale},
		{"underscore", "en_US", ErrInvalidLocale},
		{"locale-region too long", "en-USA", ErrInvalidLocale},
		{"surrounding space", " en", ErrInvalidLocale},
		{"unicode", "ภาษา", ErrInvalidLocale},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := validateLocale(c.in)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestValidateBranding(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      Branding
		wantErr error
	}{
		{"empty", Branding{}, nil},
		{"valid uppercase", Branding{PrimaryColor: "#FF00AA", AccentColor: "#001122"}, nil},
		{"valid lowercase", Branding{PrimaryColor: "#ff00aa"}, nil},
		{"only logo + font", Branding{LogoURL: "https://x/y.png", FontFamily: "Inter"}, nil},

		{"missing hash", Branding{PrimaryColor: "FF00AA"}, ErrInvalidColor},
		{"short hex", Branding{PrimaryColor: "#FFF"}, ErrInvalidColor},
		{"long hex", Branding{PrimaryColor: "#FF00AABB"}, ErrInvalidColor},
		{"non hex char", Branding{PrimaryColor: "#FF00ZZ"}, ErrInvalidColor},
		{"accent invalid", Branding{AccentColor: "red"}, ErrInvalidColor},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := validateBranding(c.in)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestIsAllowedSectionType(t *testing.T) {
	t.Parallel()
	known := []string{
		"hero", "gallery", "about", "rooms", "amenities",
		"location", "reviews", "faq", "policies", "contact",
	}
	for _, k := range known {
		k := k
		t.Run("allowed "+k, func(t *testing.T) {
			t.Parallel()
			if !isAllowedSectionType(k) {
				t.Fatalf("section %q should be allowed", k)
			}
		})
	}

	disallowed := []string{
		"", "HERO", "Hero", "html", "script", "custom", "raw",
		strings.Repeat("a", 100),
	}
	for _, d := range disallowed {
		d := d
		t.Run("rejected "+d, func(t *testing.T) {
			t.Parallel()
			if isAllowedSectionType(d) {
				t.Fatalf("section %q should be rejected", d)
			}
		})
	}
}

func TestValidateSections(t *testing.T) {
	t.Parallel()
	mkSec := func(typ string, order int) Section {
		return Section{Type: typ, Enabled: true, Order: order, Content: json.RawMessage(`{}`)}
	}

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		if err := validateSections(nil); err != nil {
			t.Fatalf("nil should pass, got %v", err)
		}
		if err := validateSections([]Section{}); err != nil {
			t.Fatalf("empty slice should pass, got %v", err)
		}
	})

	t.Run("all known", func(t *testing.T) {
		t.Parallel()
		secs := []Section{
			mkSec("hero", 0),
			mkSec("gallery", 1),
			mkSec("about", 2),
		}
		if err := validateSections(secs); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("unknown type rejected", func(t *testing.T) {
		t.Parallel()
		secs := []Section{
			mkSec("hero", 0),
			mkSec("custom-html", 1),
		}
		if err := validateSections(secs); !errors.Is(err, ErrInvalidSectionType) {
			t.Fatalf("want ErrInvalidSectionType, got %v", err)
		}
	})

	t.Run("duplicate order rejected", func(t *testing.T) {
		t.Parallel()
		secs := []Section{
			mkSec("hero", 0),
			mkSec("gallery", 0),
		}
		if err := validateSections(secs); !errors.Is(err, ErrInvalidSection) {
			t.Fatalf("want ErrInvalidSection, got %v", err)
		}
	})
}
