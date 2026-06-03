package sqlite

import (
	"database/sql"
	"errors"
	"time"

	"simple_backend_server/internal/domain"
)

// RefreshTokenRepository is the SQLite-backed domain.RefreshTokenRepository.
type RefreshTokenRepository struct {
	db *sql.DB
}

var _ domain.RefreshTokenRepository = (*RefreshTokenRepository)(nil)

func NewRefreshTokenRepository(db *sql.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Issue(token, userID string, expiresAt time.Time) error {
	_, err := r.db.Exec(
		"INSERT INTO refresh_tokens (token, user_id, expires_at) VALUES (?, ?, ?)",
		token, userID, expiresAt,
	)
	return err
}

// Rotate consumes oldToken and stores newToken atomically. The old token is
// always deleted (single-use), even when expired.
func (r *RefreshTokenRepository) Rotate(oldToken, newToken string, expiresAt time.Time) (domain.User, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback()

	var userID string
	var oldExpiresAt time.Time
	err = tx.QueryRow(
		"SELECT user_id, expires_at FROM refresh_tokens WHERE token = ?", oldToken,
	).Scan(&userID, &oldExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, domain.RefreshError{Message: "refresh token is not recognized"}
		}
		return domain.User{}, err
	}

	if _, err := tx.Exec("DELETE FROM refresh_tokens WHERE token = ?", oldToken); err != nil {
		return domain.User{}, err
	}

	if time.Now().After(oldExpiresAt) {
		return domain.User{}, domain.RefreshError{Message: "refresh token is expired"}
	}

	var username string
	if err := tx.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username); err != nil {
		return domain.User{}, err
	}

	if _, err := tx.Exec(
		"INSERT INTO refresh_tokens (token, user_id, expires_at) VALUES (?, ?, ?)",
		newToken, userID, expiresAt,
	); err != nil {
		return domain.User{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return domain.User{ID: userID, Username: username}, nil
}

func (r *RefreshTokenRepository) Revoke(token string) {
	r.db.Exec("DELETE FROM refresh_tokens WHERE token = ?", token)
}
