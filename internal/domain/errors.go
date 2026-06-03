package domain

import "errors"

// Sentinel errors shared across the domain. Outer layers (transport) map these
// to protocol-specific responses.
var (
	// ErrNotFound is returned when a requested entity does not exist (or is not
	// owned by the requesting user).
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned when creating an entity whose unique id is taken.
	ErrConflict = errors.New("conflict")
	// ErrInvalidCredentials is returned when a username/password pair does not
	// match a stored account.
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// ValidationError describes invalid user input. Its message is safe to surface
// to the caller.
type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }

// RefreshError describes why a refresh token could not be rotated. Its message
// is safe to surface to the caller.
type RefreshError struct{ Message string }

func (e RefreshError) Error() string { return e.Message }
