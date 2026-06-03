package security

import (
	"crypto/rand"
	"encoding/hex"
)

// RandomID mints cryptographically random 48-character hex identifiers. It
// satisfies service.IDGenerator.
type RandomID struct{}

func (RandomID) NewID() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
