package rand

import (
	cryptoRand "crypto/rand"
	"math/rand"

	"time"
)

func GenerateRandomData(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := cryptoRand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func GenerateRandomDataFromBuffer(b []byte) error {
	_, err := cryptoRand.Read(b)
	if err != nil {
		return err
	}
	return nil
}

var source rand.Source = rand.NewSource(time.Now().UnixNano())
var generator *rand.Rand = rand.New(source)


// Generates a unique ID when a session is created to identify it.
func GenerateSessionId() uint32 {
	// guarantee randomness with crypto/rand?
	return generator.Uint32()
}