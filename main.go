package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
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

type tokenPair struct {
	User         user   `json:"user"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

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

func revokeRefreshToken(db *sql.DB, token string) {
	db.Exec("DELETE FROM refresh_tokens WHERE token = ?", token)
}

func randomToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

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

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	secret := []byte(os.Getenv("JWT_SECRET"))
	if len(secret) == 0 {
		log.Print("warning: JWT_SECRET not set, using insecure dev default")
		secret = []byte("dev-only-secret-do-not-use-in-prod")
	}

	db, err := initDB("data.db")
	if err != nil {
		log.Fatalf("failed to initialize db: %v", err)
	}
	defer db.Close()

	issuer := newJWTIssuer(secret)
	bookmarks := newBookmarkStore(db)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Serve static files from the uploads directory
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/register", registerHandler(issuer, db))
		r.Post("/sign-in", signInHandler(issuer, db))
		r.Post("/refresh", refreshHandler(issuer, db))
		r.Post("/sign-out", signOutHandler(db))
		r.Get("/me", authMiddleware(issuer, meHandler()))
		r.Post("/change-password", authMiddleware(issuer, changePasswordHandler(db)))
	})

	r.Post("/api/upload", uploadHandler())

	registerBookmarkRoutes(r, issuer, bookmarks)

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
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

type ctxKey string

const userCtxKey ctxKey = "user"

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

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}

func uploadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Limit body size to 10MB
		r.ParseMultipartForm(10 << 20)
		file, handler, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_file", "Failed to get file from form: "+err.Error())
			return
		}
		defer file.Close()

		// Ensure uploads directory exists
		if err := os.MkdirAll("./uploads", 0755); err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", "Failed to create uploads directory.")
			return
		}

		// Generate unique name
		ext := ""
		if parts := strings.Split(handler.Filename, "."); len(parts) > 1 {
			ext = "." + parts[len(parts)-1]
		}
		token, err := randomToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", "Failed to generate file name.")
			return
		}
		filename := token + ext
		filePath := filepath.Join("uploads", filename)

		// Create file on disk
		dst, err := os.Create(filePath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", "Failed to save file on disk.")
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", "Failed to copy file contents.")
			return
		}

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		fileURL := fmt.Sprintf("%s://%s/uploads/%s", scheme, r.Host, filename)

		writeJSON(w, http.StatusOK, map[string]string{"url": fileURL})
	}
}
