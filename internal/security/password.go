package security

import "golang.org/x/crypto/bcrypt"

// BcryptHasher hashes and verifies passwords with bcrypt. It satisfies
// service.PasswordHasher.
type BcryptHasher struct{}

func (BcryptHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
