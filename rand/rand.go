package rand

import "crypto/rand"

func GenerateRandomData(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func GenerateRandomDataFromBuffer(b []byte) error {
	_, err := rand.Read(b)
	if err != nil {
		return err
	}
	return nil
}