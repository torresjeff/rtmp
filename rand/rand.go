package rand

import (
	cryptoRand "crypto/rand"
	"github.com/google/uuid"
)

// GenerateCryptoSafeRandomDataN returns a slice of bytes of length n, filled with cryptographically-safe random data.
func GenerateCryptoSafeRandomDataN(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := cryptoRand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GenerateCryptoSafeRandomData fills b with cryptographically-safe random data.
func GenerateCryptoSafeRandomData(b []byte) error {
	_, err := cryptoRand.Read(b)
	if err != nil {
		return err
	}
	return nil
}


// GenerateUuid returns a UUID in string format (including hyphens).
func GenerateUuid() string {
	return uuid.NewString()
}