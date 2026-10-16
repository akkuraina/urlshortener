package shortener

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const (
	// CodeLength gives 62^7 ≈ 3.5 trillion possible codes.
	CodeLength = 7
	alphabet   = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// NewCode returns a random base62 short code of length CodeLength.
// It does not check for uniqueness; the database's unique index does that.
func NewCode() (string, error) {
	max := big.NewInt(int64(len(alphabet)))
	b := make([]byte, CodeLength)
	for i := range b {
		// rand.Int picks uniformly from [0, max), avoiding the bias of byte % 62.
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generating short code: %w", err)
		}
		b[i] = alphabet[n.Int64()]
	}
	return string(b), nil
}
