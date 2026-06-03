package service

import "simple_backend_server/internal/domain"

// PasswordHasher hashes and verifies passwords. Implemented by the security
// layer (bcrypt).
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

// TokenIssuer signs and parses short-lived access tokens. Implemented by the
// security layer (JWT).
type TokenIssuer interface {
	Sign(u domain.User) (string, error)
	Parse(raw string) (domain.User, error)
}

// IDGenerator mints opaque random identifiers (user ids, bookmark ids, refresh
// tokens). Implemented by the security layer.
type IDGenerator interface {
	NewID() (string, error)
}
