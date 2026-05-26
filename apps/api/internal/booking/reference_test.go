package booking

import (
	"strings"
	"testing"
)

func TestGenerateReference_Format(t *testing.T) {
	t.Parallel()
	ref, err := generateReference()
	if err != nil {
		t.Fatalf("generateReference: %v", err)
	}
	if !strings.HasPrefix(ref, "HB-") {
		t.Fatalf("expected HB- prefix, got %q", ref)
	}
	if len(ref) != 9 {
		t.Fatalf("expected length 9, got %d (%q)", len(ref), ref)
	}
	body := ref[3:]
	for i, r := range body {
		if !strings.ContainsRune(refAlphabet, r) {
			t.Fatalf("char %d (%q) not in alphabet", i, r)
		}
	}
}

func TestGenerateReference_Unique(t *testing.T) {
	t.Parallel()
	const n = 1000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		ref, err := generateReference()
		if err != nil {
			t.Fatalf("generateReference: %v", err)
		}
		if _, dup := seen[ref]; dup {
			t.Fatalf("collision after %d gens: %s", i, ref)
		}
		seen[ref] = struct{}{}
	}
}

func TestRefAlphabet_NoAmbiguousChars(t *testing.T) {
	t.Parallel()
	forbidden := "01OILUAEIOiouae"
	for _, r := range refAlphabet {
		if strings.ContainsRune(forbidden, r) {
			t.Fatalf("alphabet contains visually-ambiguous or vowel char: %q", r)
		}
	}
}
