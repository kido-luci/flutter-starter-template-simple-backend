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
	since, present, ok := sinceCursor(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", "since must be a non-negative integer.")
		return
	}

	var (
		list []domain.Collection
		err  error
	)
	// A ?since cursor requests the delta (rows changed after that revision,
	// including tombstones); its absence requests the full live list.
	if present {
		list, err = rt.collections.ListSince(u.ID, since)
	} else {
		list, err = rt.collections.List(u.ID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch collections.")
		return
	}
	writeJSON(w, http.StatusOK, toCollectionDTOs(list))
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
	rev, ok := expectedRev(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", "X-Expected-Rev must be a non-negative integer.")
		return
	}
	c, err := rt.collections.Update(chi.URLParam(r, "id"), u.ID, req.toInput(), rev)
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
	rev, ok := expectedRev(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", "X-Expected-Rev must be a non-negative integer.")
		return
	}
	err := rt.collections.Delete(chi.URLParam(r, "id"), u.ID, rev)
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
