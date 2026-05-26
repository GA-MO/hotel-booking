package roomtype

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{"single char", "A", nil},
		{"normal", "Deluxe Twin", nil},
		{"thai", "ห้องสวีท", nil},
		{"max length", strings.Repeat("a", 120), nil},

		{"empty", "", ErrInvalidName},
		{"too long", strings.Repeat("a", 121), ErrInvalidName},
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

func TestValidateCapacity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      int
		wantErr error
	}{
		{"min", 1, nil},
		{"normal", 4, nil},
		{"large", 999, nil},

		{"zero", 0, ErrInvalidCapacity},
		{"negative", -1, ErrInvalidCapacity},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := validateCapacity(c.in)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestValidateInventory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      int
		wantErr error
	}{
		{"zero allowed", 0, nil},
		{"positive", 10, nil},

		{"negative", -1, ErrInvalidInventory},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := validateInventory(c.in)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestValidateBaseRate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      float64
		wantErr error
	}{
		{"zero allowed", 0, nil},
		{"positive", 1200.50, nil},

		{"negative", -0.01, ErrInvalidBaseRate},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := validateBaseRate(c.in)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestValidateCurrency(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{"upper THB", "THB", nil},
		{"upper USD", "USD", nil},
		{"lower allowed", "thb", nil},
		{"mixed", "Thb", nil},

		{"empty", "", ErrInvalidCurrency},
		{"too short", "TH", ErrInvalidCurrency},
		{"too long", "THBX", ErrInvalidCurrency},
		{"digits", "TH1", ErrInvalidCurrency},
		{"symbol", "TH$", ErrInvalidCurrency},
		{"unicode", "ทบ฿", ErrInvalidCurrency},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := validateCurrency(c.in)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestValidateDisplayOrder(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      int
		wantErr error
	}{
		{"zero", 0, nil},
		{"positive", 5, nil},

		{"negative", -1, ErrInvalidDisplayOrder},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := validateDisplayOrder(c.in)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestValidateStorageKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{"normal", "hotels/abc/cover.jpg", nil},
		{"single char", "x", nil},
		{"max", strings.Repeat("a", storageKeyMax), nil},

		{"empty", "", ErrInvalidStorageKey},
		{"whitespace", "   ", ErrInvalidStorageKey},
		{"too long", strings.Repeat("a", storageKeyMax+1), ErrInvalidStorageKey},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := validateStorageKey(c.in)
			if c.wantErr == nil && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
		})
	}
}

func TestValidatePhotoCreate(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		req := CreatePhotoRequest{StorageKey: "hotels/abc.jpg", DisplayOrder: 0}
		if err := validatePhotoCreate(&req); err != nil {
			t.Fatalf("want nil, got %v", err)
		}
	})

	t.Run("trims storage key", func(t *testing.T) {
		t.Parallel()
		req := CreatePhotoRequest{StorageKey: "  hotels/abc.jpg  "}
		if err := validatePhotoCreate(&req); err != nil {
			t.Fatalf("want nil, got %v", err)
		}
		if req.StorageKey != "hotels/abc.jpg" {
			t.Fatalf("want trimmed, got %q", req.StorageKey)
		}
	})

	t.Run("missing storage key", func(t *testing.T) {
		t.Parallel()
		req := CreatePhotoRequest{StorageKey: ""}
		if err := validatePhotoCreate(&req); !errors.Is(err, ErrInvalidStorageKey) {
			t.Fatalf("want ErrInvalidStorageKey, got %v", err)
		}
	})

	t.Run("negative display order", func(t *testing.T) {
		t.Parallel()
		req := CreatePhotoRequest{StorageKey: "ok", DisplayOrder: -1}
		if err := validatePhotoCreate(&req); !errors.Is(err, ErrInvalidDisplayOrder) {
			t.Fatalf("want ErrInvalidDisplayOrder, got %v", err)
		}
	})
}

func TestValidatePhotoUpdate(t *testing.T) {
	t.Parallel()

	t.Run("nil display order skips check", func(t *testing.T) {
		t.Parallel()
		req := UpdatePhotoRequest{}
		if err := validatePhotoUpdate(&req); err != nil {
			t.Fatalf("want nil, got %v", err)
		}
	})

	t.Run("valid display order", func(t *testing.T) {
		t.Parallel()
		n := 3
		req := UpdatePhotoRequest{DisplayOrder: &n}
		if err := validatePhotoUpdate(&req); err != nil {
			t.Fatalf("want nil, got %v", err)
		}
	})

	t.Run("negative display order", func(t *testing.T) {
		t.Parallel()
		n := -1
		req := UpdatePhotoRequest{DisplayOrder: &n}
		if err := validatePhotoUpdate(&req); !errors.Is(err, ErrInvalidDisplayOrder) {
			t.Fatalf("want ErrInvalidDisplayOrder, got %v", err)
		}
	})
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
