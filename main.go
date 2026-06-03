package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"simple_backend_server/internal/security"
	"simple_backend_server/internal/service"
	"simple_backend_server/internal/storage/sqlite"
	"simple_backend_server/internal/transport/httpapi"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
)

func main() {
	addr := envOr("ADDR", ":8080")

	secret := []byte(os.Getenv("JWT_SECRET"))
	if len(secret) == 0 {
		log.Print("warning: JWT_SECRET not set, using insecure dev default")
		secret = []byte("dev-only-secret-do-not-use-in-prod")
	}

	db, err := sqlite.Open("data.db")
	if err != nil {
		log.Fatalf("failed to initialize db: %v", err)
	}
	defer db.Close()

	// Persistence adapters (storage layer).
	users := sqlite.NewUserRepository(db)
	refreshTokens := sqlite.NewRefreshTokenRepository(db)
	bookmarksRepo := sqlite.NewBookmarkRepository(db)
	notificationsRepo := sqlite.NewNotificationRepository(db)
	activitiesRepo := sqlite.NewActivityRepository(db)

	// Crypto adapters (security layer).
	issuer := security.NewJWTIssuer(secret, accessTokenTTL)
	hasher := security.BcryptHasher{}
	ids := security.RandomID{}

	// Use cases (service layer).
	auth := service.NewAuthService(users, refreshTokens, hasher, issuer, ids, accessTokenTTL, refreshTokenTTL)
	bookmarks := service.NewBookmarkService(bookmarksRepo, activitiesRepo, notificationsRepo, ids)
	notifications := service.NewNotificationService(notificationsRepo, activitiesRepo)

	// HTTP transport (interface-adapter layer).
	handler := httpapi.New(auth, bookmarks, notifications, issuer, ids, "./uploads")

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
