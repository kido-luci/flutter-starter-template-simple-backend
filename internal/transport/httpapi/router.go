// Package httpapi is the HTTP interface-adapter layer: it wires routes,
// decodes/encodes JSON, and delegates to the service use cases.
package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"simple_backend_server/internal/service"
)

// Router holds the dependencies the HTTP handlers delegate to.
type Router struct {
	auth          *service.AuthService
	bookmarks     *service.BookmarkService
	notifications *service.NotificationService
	tokens        service.TokenIssuer
	ids           service.IDGenerator
	uploadsDir    string
}

// New builds the application HTTP handler with all routes and middleware.
func New(
	auth *service.AuthService,
	bookmarks *service.BookmarkService,
	notifications *service.NotificationService,
	tokens service.TokenIssuer,
	ids service.IDGenerator,
	uploadsDir string,
) http.Handler {
	rt := &Router{
		auth:          auth,
		bookmarks:     bookmarks,
		notifications: notifications,
		tokens:        tokens,
		ids:           ids,
		uploadsDir:    uploadsDir,
	}

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

	// Serve static files from the uploads directory.
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(rt.uploadsDir))))

	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/register", rt.handleRegister)
		r.Post("/sign-in", rt.handleSignIn)
		r.Post("/refresh", rt.handleRefresh)
		r.Post("/sign-out", rt.handleSignOut)
		r.Get("/me", rt.authenticated(rt.handleMe))
		r.Post("/change-password", rt.authenticated(rt.handleChangePassword))
	})

	r.Post("/api/upload", rt.authenticated(rt.handleUpload))

	r.Route("/api/bookmarks", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		r.Get("/", rt.handleListBookmarks)
		r.Post("/", rt.handleCreateBookmark)
		r.Get("/{id}", rt.handleGetBookmark)
		r.Put("/{id}", rt.handleUpdateBookmark)
		r.Delete("/{id}", rt.handleDeleteBookmark)
	})

	r.Get("/api/notifications", rt.authenticated(rt.handleListNotifications))
	r.Patch("/api/notifications/{id}/read", rt.authenticated(rt.handleMarkNotificationRead))
	r.Get("/api/activity", rt.authenticated(rt.handleListActivity))

	return r
}
