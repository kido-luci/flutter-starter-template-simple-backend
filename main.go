package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

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

	r.Post("/api/upload", authMiddleware(issuer, uploadHandler()))

	registerBookmarkRoutes(r, issuer, bookmarks)

	// Endpoints for notifications and activity
	r.Get("/api/notifications", authMiddleware(issuer, getNotificationsHandler(db)))
	r.Patch("/api/notifications/{id}/read", authMiddleware(issuer, markNotificationReadHandler(db)))
	r.Get("/api/activity", authMiddleware(issuer, getActivityHandler(db)))

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
