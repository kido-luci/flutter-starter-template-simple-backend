package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"simple_backend_server/internal/domain"
)

func (rt *Router) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req signInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Request body is not valid JSON.")
		return
	}
	if strings.TrimSpace(req.Username) == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_credentials", "Username and password are required.")
		return
	}
	res, err := rt.auth.Register(req.Username, req.Password)
	switch {
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "Username already exists.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "server_error", "Failed to register user.")
	default:
		writeJSON(w, http.StatusOK, toTokenPairDTO(res))
	}
}

func (rt *Router) handleSignIn(w http.ResponseWriter, r *http.Request) {
	var req signInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Request body is not valid JSON.")
		return
	}
	if strings.TrimSpace(req.Username) == "" || req.Password == "" {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Username and password are required.")
		return
	}
	res, err := rt.auth.SignIn(req.Username, req.Password)
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "server_error", "Database error.")
	default:
		writeJSON(w, http.StatusOK, toTokenPairDTO(res))
	}
}

func (rt *Router) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	u, ok := userFrom(r)
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
	err := rt.auth.ChangePassword(u.ID, req.CurrentPassword, req.NewPassword)
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Incorrect current password.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "server_error", "Failed to update password.")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (rt *Router) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
		writeError(w, http.StatusBadRequest, "invalid_body", "refresh_token is required.")
		return
	}
	res, err := rt.auth.Refresh(req.RefreshToken)
	var re domain.RefreshError
	switch {
	case errors.As(err, &re):
		writeError(w, http.StatusUnauthorized, "invalid_refresh", re.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, "token_error", "Failed to issue access token.")
	default:
		writeJSON(w, http.StatusOK, toTokenPairDTO(res))
	}
}

func (rt *Router) handleSignOut(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	rt.auth.SignOut(strings.TrimSpace(req.RefreshToken))
	w.WriteHeader(http.StatusNoContent)
}

func (rt *Router) handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := userFrom(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Missing or invalid token.")
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(u))
}
