package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
)

// randomToken returns a cryptographically random 48-character hex string,
// used for opaque refresh tokens, user IDs, and generated resource IDs.
func randomToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// jwtIssuer signs and parses short-lived HS256 access tokens.
type jwtIssuer struct {
	secret []byte
}

func newJWTIssuer(secret []byte) *jwtIssuer {
	return &jwtIssuer{secret: secret}
}

type accessClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func (j *jwtIssuer) sign(u user) (string, error) {
	now := time.Now()
	claims := accessClaims{
		Username: u.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

func (j *jwtIssuer) parse(raw string) (user, error) {
	claims := &accessClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return j.secret, nil
	})
	if err != nil {
		return user{}, err
	}
	return user{ID: claims.Subject, Username: claims.Username}, nil
}

// issueRefreshToken mints and persists a new opaque refresh token for u.
func issueRefreshToken(db *sql.DB, u user) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(refreshTokenTTL)
	_, err = db.Exec("INSERT INTO refresh_tokens (token, user_id, expires_at) VALUES (?, ?, ?)", token, u.ID, expiresAt)
	if err != nil {
		return "", err
	}
	return token, nil
}

// rotateRefreshToken atomically consumes the given refresh token and issues a
// replacement, returning the owning user and the new token. The old token is
// always deleted (single-use), even when it has expired.
func rotateRefreshToken(db *sql.DB, token string) (user, string, error) {
	tx, err := db.Begin()
	if err != nil {
		return user{}, "", err
	}
	defer tx.Rollback()

	var userID string
	var expiresAt time.Time
	err = tx.QueryRow("SELECT user_id, expires_at FROM refresh_tokens WHERE token = ?", token).Scan(&userID, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user{}, "", errors.New("refresh token is not recognized")
		}
		return user{}, "", err
	}

	_, err = tx.Exec("DELETE FROM refresh_tokens WHERE token = ?", token)
	if err != nil {
		return user{}, "", err
	}

	if time.Now().After(expiresAt) {
		return user{}, "", errors.New("refresh token is expired")
	}

	var username string
	err = tx.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
	if err != nil {
		return user{}, "", err
	}
	u := user{ID: userID, Username: username}

	newToken, err := randomToken()
	if err != nil {
		return user{}, "", err
	}
	newExpiresAt := time.Now().Add(refreshTokenTTL)
	_, err = tx.Exec("INSERT INTO refresh_tokens (token, user_id, expires_at) VALUES (?, ?, ?)", newToken, userID, newExpiresAt)
	if err != nil {
		return user{}, "", err
	}

	if err := tx.Commit(); err != nil {
		return user{}, "", err
	}

	return u, newToken, nil
}

// revokeRefreshToken deletes the token if present; a no-op otherwise.
func revokeRefreshToken(db *sql.DB, token string) {
	db.Exec("DELETE FROM refresh_tokens WHERE token = ?", token)
}
