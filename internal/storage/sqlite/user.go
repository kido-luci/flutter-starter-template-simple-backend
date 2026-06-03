package sqlite

import (
	"database/sql"
	"errors"
	"strings"

	"simple_backend_server/internal/domain"
)

// UserRepository is the SQLite-backed domain.UserRepository.
type UserRepository struct {
	db *sql.DB
}

var _ domain.UserRepository = (*UserRepository)(nil)

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(u domain.User, passwordHash string) error {
	_, err := r.db.Exec(
		"INSERT INTO users (id, username, password_hash) VALUES (?, ?, ?)",
		u.ID, u.Username, passwordHash,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return domain.ErrConflict
		}
		return err
	}
	return nil
}

func (r *UserRepository) FindByUsername(username string) (domain.User, string, error) {
	var id, hash string
	err := r.db.QueryRow(
		"SELECT id, password_hash FROM users WHERE username = ?", username,
	).Scan(&id, &hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, "", domain.ErrNotFound
		}
		return domain.User{}, "", err
	}
	return domain.User{ID: id, Username: username}, hash, nil
}

func (r *UserRepository) PasswordHash(userID string) (string, error) {
	var hash string
	err := r.db.QueryRow(
		"SELECT password_hash FROM users WHERE id = ?", userID,
	).Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return hash, nil
}

func (r *UserRepository) UpdatePassword(userID, passwordHash string) error {
	_, err := r.db.Exec(
		"UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID,
	)
	return err
}
