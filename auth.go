package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type user struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type signInRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type tokenPair struct {
	User         user   `json:"user"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// ctxKey is a private type for request-context keys to avoid collisions.
type ctxKey string

// userCtxKey holds the authenticated user injected by authMiddleware.
const userCtxKey ctxKey = "user"

// issueTokens signs a fresh access token and persists a refresh token for u.
func issueTokens(issuer *jwtIssuer, db *sql.DB, u user) (tokenPair, error) {
	access, err := issuer.sign(u)
	if err != nil {
		return tokenPair{}, err
	}
	refresh, err := issueRefreshToken(db, u)
	if err != nil {
		return tokenPair{}, err
	}
	return tokenPair{
		User:         u,
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
	}, nil
}

// authMiddleware rejects requests without a valid Bearer access token and, on
// success, stores the resolved user under userCtxKey for downstream handlers.
func authMiddleware(issuer *jwtIssuer, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := tokenFromHeader(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", err.Error())
			return
		}
		u, err := issuer.parse(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Access token is invalid or expired.")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func tokenFromHeader(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", errors.New("Authorization header is required.")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("Authorization header must be a Bearer token.")
	}
	return parts[1], nil
}

func registerHandler(issuer *jwtIssuer, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req signInRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "Request body is not valid JSON.")
			return
		}
		if strings.TrimSpace(req.Username) == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "invalid_credentials", "Username and password are required.")
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", "Failed to hash password.")
			return
		}

		id, err := randomToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", "Failed to generate ID.")
			return
		}

		u := user{ID: id, Username: req.Username}
		_, err = db.Exec("INSERT INTO users (id, username, password_hash) VALUES (?, ?, ?)", id, req.Username, string(hashedPassword))
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				writeError(w, http.StatusConflict, "conflict", "Username already exists.")
				return
			}
			writeError(w, http.StatusInternalServerError, "server_error", "Failed to save user.")
			return
		}

		pair, err := issueTokens(issuer, db, u)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "token_error", "Failed to issue tokens.")
			return
		}
		writeJSON(w, http.StatusOK, pair)
	}
}

func signInHandler(issuer *jwtIssuer, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req signInRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "Request body is not valid JSON.")
			return
		}
		if strings.TrimSpace(req.Username) == "" || req.Password == "" {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "Username and password are required.")
			return
		}

		var id, passwordHash string
		err := db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", req.Username).Scan(&id, &passwordHash)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password.")
				return
			}
			writeError(w, http.StatusInternalServerError, "server_error", "Database error.")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password.")
			return
		}

		u := user{ID: id, Username: req.Username}
		pair, err := issueTokens(issuer, db, u)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "token_error", "Failed to issue tokens.")
			return
		}
		writeJSON(w, http.StatusOK, pair)
	}
}

func changePasswordHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := r.Context().Value(userCtxKey).(user)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Missing or invalid token.")
			return
		}

		var req changePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "Request body is not valid JSON.")
			return
		}
		if strings.TrimSpace(req.CurrentPassword) == "" || strings.TrimSpace(req.NewPassword) == "" {
			writeError(w, http.StatusBadRequest, "invalid_input", "Current and new password are required.")
			return
		}

		var passwordHash string
		err := db.QueryRow("SELECT password_hash FROM users WHERE id = ?", u.ID).Scan(&passwordHash)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", "Database error.")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.CurrentPassword)); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "Incorrect current password.")
			return
		}

		newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", "Failed to hash new password.")
			return
		}

		_, err = db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(newHash), u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", "Failed to update password.")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func refreshHandler(issuer *jwtIssuer, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req refreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
			writeError(w, http.StatusBadRequest, "invalid_body", "refresh_token is required.")
			return
		}
		u, newRefresh, err := rotateRefreshToken(db, req.RefreshToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_refresh", err.Error())
			return
		}
		access, err := issuer.sign(u)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "token_error", "Failed to issue access token.")
			return
		}
		writeJSON(w, http.StatusOK, tokenPair{
			User:         u,
			AccessToken:  access,
			RefreshToken: newRefresh,
			ExpiresIn:    int64(accessTokenTTL.Seconds()),
		})
	}
}

func signOutHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req refreshRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if strings.TrimSpace(req.RefreshToken) != "" {
			revokeRefreshToken(db, req.RefreshToken)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func meHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := r.Context().Value(userCtxKey).(user)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Missing or invalid token.")
			return
		}
		writeJSON(w, http.StatusOK, u)
	}
}
