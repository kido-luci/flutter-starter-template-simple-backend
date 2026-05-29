package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type bookmark struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	ImageUrls   []string  `json:"image_urls"`
	VideoUrl    string    `json:"video_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type bookmarkRequest struct {
	// Optional client-provided ID. When non-empty on POST, the server uses
	// it instead of generating one, so offline-first clients can mint stable
	// IDs locally. Ignored on PUT (path id wins).
	ID          string   `json:"id,omitempty"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	ImageUrls   []string `json:"image_urls"`
	VideoUrl    string   `json:"video_url"`
}

type bookmarkStore struct {
	db *sql.DB
}

func newBookmarkStore(db *sql.DB) *bookmarkStore {
	return &bookmarkStore{db: db}
}

func (s *bookmarkStore) listByOwner(ownerID string) []bookmark {
	rows, err := s.db.Query("SELECT id, owner_id, title, url, description, tags, image_urls, video_url, created_at, updated_at FROM bookmarks WHERE owner_id = ? ORDER BY created_at DESC", ownerID)
	out := make([]bookmark, 0)
	if err != nil {
		return out
	}
	defer rows.Close()

	for rows.Next() {
		var b bookmark
		var tagsJSON, imagesJSON string
		if err := rows.Scan(&b.ID, &b.OwnerID, &b.Title, &b.URL, &b.Description, &tagsJSON, &imagesJSON, &b.VideoUrl, &b.CreatedAt, &b.UpdatedAt); err == nil {
			_ = json.Unmarshal([]byte(tagsJSON), &b.Tags)
			_ = json.Unmarshal([]byte(imagesJSON), &b.ImageUrls)
			if b.Tags == nil {
				b.Tags = []string{}
			}
			if b.ImageUrls == nil {
				b.ImageUrls = []string{}
			}
			out = append(out, b)
		}
	}
	return out
}

func (s *bookmarkStore) getOwned(id, ownerID string) (bookmark, bool) {
	row := s.db.QueryRow("SELECT id, owner_id, title, url, description, tags, image_urls, video_url, created_at, updated_at FROM bookmarks WHERE id = ? AND owner_id = ?", id, ownerID)
	var b bookmark
	var tagsJSON, imagesJSON string
	err := row.Scan(&b.ID, &b.OwnerID, &b.Title, &b.URL, &b.Description, &tagsJSON, &imagesJSON, &b.VideoUrl, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return bookmark{}, false
	}
	_ = json.Unmarshal([]byte(tagsJSON), &b.Tags)
	_ = json.Unmarshal([]byte(imagesJSON), &b.ImageUrls)
	if b.Tags == nil {
		b.Tags = []string{}
	}
	if b.ImageUrls == nil {
		b.ImageUrls = []string{}
	}
	return b, true
}

// errBookmarkConflict is returned when a client-provided ID collides with an
// existing bookmark, so the handler can map it to HTTP 409.
var errBookmarkConflict = errors.New("bookmark with this id already exists")

func (s *bookmarkStore) create(ownerID string, req bookmarkRequest) (bookmark, error) {
	id := strings.TrimSpace(req.ID)
	if id == "" {
		generated, err := randomToken()
		if err != nil {
			return bookmark{}, err
		}
		id = generated
	}
	now := time.Now().UTC()
	b := bookmark{
		ID:          id,
		OwnerID:     ownerID,
		Title:       req.Title,
		URL:         req.URL,
		Description: req.Description,
		Tags:        normalizeTags(req.Tags),
		ImageUrls:   req.ImageUrls,
		VideoUrl:    req.VideoUrl,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	tagsJSON, _ := json.Marshal(b.Tags)
	imagesJSON, _ := json.Marshal(b.ImageUrls)

	_, err := s.db.Exec("INSERT INTO bookmarks (id, owner_id, title, url, description, tags, image_urls, video_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		b.ID, b.OwnerID, b.Title, b.URL, b.Description, string(tagsJSON), string(imagesJSON), b.VideoUrl, b.CreatedAt, b.UpdatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return bookmark{}, errBookmarkConflict
		}
		return bookmark{}, err
	}
	return b, nil
}

func (s *bookmarkStore) update(id, ownerID string, req bookmarkRequest) (bookmark, bool) {
	existing, ok := s.getOwned(id, ownerID)
	if !ok {
		return bookmark{}, false
	}
	
	existing.Title = req.Title
	existing.URL = req.URL
	existing.Description = req.Description
	existing.Tags = normalizeTags(req.Tags)
	existing.ImageUrls = req.ImageUrls
	existing.VideoUrl = req.VideoUrl
	existing.UpdatedAt = time.Now().UTC()

	tagsJSON, _ := json.Marshal(existing.Tags)
	imagesJSON, _ := json.Marshal(existing.ImageUrls)

	_, err := s.db.Exec("UPDATE bookmarks SET title = ?, url = ?, description = ?, tags = ?, image_urls = ?, video_url = ?, updated_at = ? WHERE id = ? AND owner_id = ?",
		existing.Title, existing.URL, existing.Description, string(tagsJSON), string(imagesJSON), existing.VideoUrl, existing.UpdatedAt, id, ownerID)
	
	if err != nil {
		return bookmark{}, false
	}
	return existing, true
}

func (s *bookmarkStore) delete(id, ownerID string) bool {
	res, err := s.db.Exec("DELETE FROM bookmarks WHERE id = ? AND owner_id = ?", id, ownerID)
	if err != nil {
		return false
	}
	affected, _ := res.RowsAffected()
	return affected > 0
}

func normalizeTags(in []string) []string {
	if in == nil {
		return []string{}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func validateBookmarkRequest(req bookmarkRequest) error {
	if strings.TrimSpace(req.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(req.URL) == "" {
		return errors.New("url is required")
	}
	return nil
}

func registerBookmarkRoutes(r chi.Router, issuer *jwtIssuer, store *bookmarkStore) {
	r.Route("/api/bookmarks", func(r chi.Router) {
		r.Use(authMiddlewareChi(issuer))
		r.Get("/", listBookmarksHandler(store))
		r.Post("/", createBookmarkHandler(store))
		r.Get("/{id}", getBookmarkHandler(store))
		r.Put("/{id}", updateBookmarkHandler(store))
		r.Delete("/{id}", deleteBookmarkHandler(store))
	})
}

// authMiddlewareChi adapts the existing authMiddleware to chi's middleware
// signature so it can be applied to a route group.
func authMiddlewareChi(issuer *jwtIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authMiddleware(issuer, func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})(w, r)
		})
	}
}

func listBookmarksHandler(store *bookmarkStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := r.Context().Value(userCtxKey).(user)
		writeJSON(w, http.StatusOK, store.listByOwner(u.ID))
	}
}

func getBookmarkHandler(store *bookmarkStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := r.Context().Value(userCtxKey).(user)
		id := chi.URLParam(r, "id")
		b, ok := store.getOwned(id, u.ID)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "Bookmark not found.")
			return
		}
		writeJSON(w, http.StatusOK, b)
	}
}

func createBookmarkHandler(store *bookmarkStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := r.Context().Value(userCtxKey).(user)
		var req bookmarkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "Request body is not valid JSON.")
			return
		}
		if err := validateBookmarkRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
			return
		}
		b, err := store.create(u.ID, req)
		if errors.Is(err, errBookmarkConflict) {
			writeError(w, http.StatusConflict, "conflict", "Bookmark with this id already exists.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create_failed", "Failed to create bookmark.")
			return
		}
		writeJSON(w, http.StatusCreated, b)
	}
}

func updateBookmarkHandler(store *bookmarkStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := r.Context().Value(userCtxKey).(user)
		id := chi.URLParam(r, "id")
		var req bookmarkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "Request body is not valid JSON.")
			return
		}
		if err := validateBookmarkRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
			return
		}
		b, ok := store.update(id, u.ID, req)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "Bookmark not found.")
			return
		}
		writeJSON(w, http.StatusOK, b)
	}
}

func deleteBookmarkHandler(store *bookmarkStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := r.Context().Value(userCtxKey).(user)
		id := chi.URLParam(r, "id")
		if !store.delete(id, u.ID) {
			writeError(w, http.StatusNotFound, "not_found", "Bookmark not found.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
