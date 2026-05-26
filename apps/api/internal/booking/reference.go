package booking

import (
	"crypto/rand"
	"strings"
)

// Reference codes are 9 chars: "HB-XXXXXX" using Crockford base32 minus
// vowels (so no accidental words) and minus visually ambiguous chars
// (0/O, 1/I/L). 28^6 ≈ 480M values — plenty for our scale and short
// enough to read aloud over the phone.
const refAlphabet = "23456789BCDFGHJKMNPQRSTVWXYZ" // 28 chars

func generateReference() (string, error) {
	const length = 6
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var b strings.Builder
	b.Grow(3 + length)
	b.WriteString("HB-")
	for _, x := range buf {
		b.WriteByte(refAlphabet[int(x)%len(refAlphabet)])
	}
	return b.String(), nil
}
