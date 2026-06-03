package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"simple_backend_server/internal/domain"
)

type ctxKey int

const userKey ctxKey = iota

// authenticated wraps a handler so it runs only for requests carrying a valid
// Bearer access token, injecting the resolved user into the request context.
func (rt *Router) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rt.authenticate(w, r, next)
	}
}

// authMiddleware is the chi-style equivalent of authenticated, for route groups.
func (rt *Router) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rt.authenticate(w, r, next.ServeHTTP)
	})
}

func (rt *Router) authenticate(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	token, err := bearerToken(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", err.Error())
		return
	}
	u, err := rt.tokens.Parse(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Access token is invalid or expired.")
		return
	}
	ctx := context.WithValue(r.Context(), userKey, u)
	next.ServeHTTP(w, r.WithContext(ctx))
}

// userFrom returns the authenticated user attached by authenticate.
func userFrom(r *http.Request) (domain.User, bool) {
	u, ok := r.Context().Value(userKey).(domain.User)
	return u, ok
}

func bearerToken(r *http.Request) (string, error) {
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
