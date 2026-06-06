package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"simple_backend_server/internal/domain"
)

func (rt *Router) handleListCollections(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	list, err := rt.listCollections(r, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch collections.")
		return
	}
	writeJSON(w, http.StatusOK, toCollectionDTOs(list))
}

// listCollections serves either a full live list or, when a ?since cursor is
// present, the delta of rows (including tombstones) changed after that revision.
func (rt *Router) listCollections(r *http.Request, ownerID string) ([]domain.Collection, error) {
	if since, ok := sinceCursor(r); ok {
		return rt.collections.ListSince(ownerID, since)
	}
	return rt.collections.List(ownerID)
}

func (rt *Router) handleGetCollection(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	c, err := rt.collections.Get(chi.URLParam(r, "id"), u.ID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Collection not found.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch collection.")
	default:
		writeJSON(w, http.StatusOK, toCollectionDTO(c))
	}
}

func (rt *Router) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	var req collectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Request body is not valid JSON.")
		return
	}
	c, err := rt.collections.Create(u.ID, req.toInput())
	var ve domain.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, "invalid_input", ve.Error())
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "Collection with this id already exists.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "create_failed", "Failed to create collection.")
	default:
		writeJSON(w, http.StatusCreated, toCollectionDTO(c))
	}
}

func (rt *Router) handleUpdateCollection(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	var req collectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Request body is not valid JSON.")
		return
	}
	c, err := rt.collections.Update(chi.URLParam(r, "id"), u.ID, req.toInput(), expectedRev(r))
	var ve domain.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, "invalid_input", ve.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Collection not found.")
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "Collection was modified on the server.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "update_failed", "Failed to update collection.")
	default:
		writeJSON(w, http.StatusOK, toCollectionDTO(c))
	}
}

func (rt *Router) handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	err := rt.collections.Delete(chi.URLParam(r, "id"), u.ID, expectedRev(r))
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Collection not found.")
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "Collection was modified on the server.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "delete_failed", "Failed to delete collection.")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
