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
	list, err := rt.listBookmarks(r, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch bookmarks.")
		return
	}
	writeJSON(w, http.StatusOK, toBookmarkDTOs(list))
}

// listBookmarks serves either a full live list or, when a ?since cursor is
// present, the delta of rows (including tombstones) changed after that revision.
func (rt *Router) listBookmarks(r *http.Request, ownerID string) ([]domain.Bookmark, error) {
	if since, ok := sinceCursor(r); ok {
		return rt.bookmarks.ListSince(ownerID, since)
	}
	return rt.bookmarks.List(ownerID)
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
	b, err := rt.bookmarks.Update(chi.URLParam(r, "id"), u.ID, req.toInput(), expectedRev(r))
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
	err := rt.bookmarks.Delete(chi.URLParam(r, "id"), u.ID, expectedRev(r))
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
