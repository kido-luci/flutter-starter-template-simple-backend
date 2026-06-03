package domain

import "time"

// User identifies an authenticated account.
type User struct {
	ID       string
	Username string
}

// AuthResult is the outcome of a successful authentication: the user plus a
// freshly issued access/refresh token pair and the access-token lifetime in
// seconds.
type AuthResult struct {
	User         User
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

// UserRepository persists user accounts and their password hashes.
type UserRepository interface {
	// Create stores a new user. It returns ErrConflict if the username exists.
	Create(u User, passwordHash string) error
	// FindByUsername returns the user and stored password hash, or ErrNotFound.
	FindByUsername(username string) (User, string, error)
	// PasswordHash returns the stored hash for the user, or ErrNotFound.
	PasswordHash(userID string) (string, error)
	// UpdatePassword replaces the stored password hash.
	UpdatePassword(userID, passwordHash string) error
}

// RefreshTokenRepository persists opaque, single-use refresh tokens.
type RefreshTokenRepository interface {
	// Issue stores a new refresh token for the user.
	Issue(token, userID string, expiresAt time.Time) error
	// Rotate atomically consumes oldToken and stores newToken, returning the
	// owning user. It returns a RefreshError when oldToken is unknown or has
	// expired.
	Rotate(oldToken, newToken string, expiresAt time.Time) (User, error)
	// Revoke deletes the token if present; a no-op otherwise.
	Revoke(token string)
}
