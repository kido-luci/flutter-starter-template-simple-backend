package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"simple_backend_server/internal/domain"
)

func (rt *Router) handleListBookmarks(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	since, present, ok := sinceCursor(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", "since must be a non-negative integer.")
		return
	}

	var (
		list []domain.Bookmark
		err  error
	)
	// A ?since cursor requests the delta (rows changed after that revision,
	// including tombstones); its absence requests the full live list.
	if present {
		list, err = rt.bookmarks.ListSince(u.ID, since)
	} else {
		list, err = rt.bookmarks.List(u.ID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch bookmarks.")
		return
	}
	writeJSON(w, http.StatusOK, toBookmarkDTOs(list))
}

func (rt *Router) handleGetBookmark(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	b, err := rt.bookmarks.Get(chi.URLParam(r, "id"), u.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Bookmark not found.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch bookmark.")
	default:
		writeJSON(w, http.StatusOK, toBookmarkDTO(b))
	}
}

func (rt *Router) handleCreateBookmark(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	var req bookmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Request body is not valid JSON.")
		return
	}
	b, err := rt.bookmarks.Create(u.ID, req.toInput())
	var ve domain.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, "invalid_input", ve.Error())
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "Bookmark with this id already exists.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "create_failed", "Failed to create bookmark.")
	default:
		writeJSON(w, http.StatusCreated, toBookmarkDTO(b))
	}
}

func (rt *Router) handleUpdateBookmark(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	var req bookmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Request body is not valid JSON.")
		return
	}
	rev, ok := expectedRev(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", "X-Expected-Rev must be a non-negative integer.")
		return
	}
	b, err := rt.bookmarks.Update(chi.URLParam(r, "id"), u.ID, req.toInput(), rev)
	var ve domain.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, "invalid_input", ve.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Bookmark not found.")
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "Bookmark was modified on the server.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "update_failed", "Failed to update bookmark.")
	default:
		writeJSON(w, http.StatusOK, toBookmarkDTO(b))
	}
}

func (rt *Router) handleDeleteBookmark(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	rev, ok := expectedRev(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", "X-Expected-Rev must be a non-negative integer.")
		return
	}
	err := rt.bookmarks.Delete(chi.URLParam(r, "id"), u.ID, rev)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Bookmark not found.")
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "Bookmark was modified on the server.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "delete_failed", "Failed to delete bookmark.")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
