package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"simple_backend_server/internal/domain"
)

const bookmarkColumns = "id, owner_id, title, url, description, tags, image_urls, video_url, created_at, updated_at"

// BookmarkRepository is the SQLite-backed domain.BookmarkRepository. Tag and
// image-URL slices are stored as JSON text columns.
type BookmarkRepository struct {
	db *sql.DB
}

var _ domain.BookmarkRepository = (*BookmarkRepository)(nil)

func NewBookmarkRepository(db *sql.DB) *BookmarkRepository {
	return &BookmarkRepository{db: db}
}

func (r *BookmarkRepository) ListByOwner(ownerID string) ([]domain.Bookmark, error) {
	rows, err := r.db.Query(
		"SELECT "+bookmarkColumns+" FROM bookmarks WHERE owner_id = ? ORDER BY created_at DESC",
		ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Bookmark, 0)
	for rows.Next() {
		b, err := scanBookmark(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *BookmarkRepository) GetOwned(id, ownerID string) (domain.Bookmark, error) {
	row := r.db.QueryRow(
		"SELECT "+bookmarkColumns+" FROM bookmarks WHERE id = ? AND owner_id = ?",
		id, ownerID,
	)
	b, err := scanBookmark(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Bookmark{}, domain.ErrNotFound
		}
		return domain.Bookmark{}, err
	}
	return b, nil
}

func (r *BookmarkRepository) Create(b domain.Bookmark) error {
	tags, _ := json.Marshal(b.Tags)
	images, _ := json.Marshal(b.ImageURLs)
	_, err := r.db.Exec(
		"INSERT INTO bookmarks ("+bookmarkColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		b.ID, b.OwnerID, b.Title, b.URL, b.Description, string(tags), string(images), b.VideoURL, b.CreatedAt, b.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.ErrConflict
		}
		return err
	}
	return nil
}

func (r *BookmarkRepository) Update(b domain.Bookmark) error {
	tags, _ := json.Marshal(b.Tags)
	images, _ := json.Marshal(b.ImageURLs)
	_, err := r.db.Exec(
		"UPDATE bookmarks SET title = ?, url = ?, description = ?, tags = ?, image_urls = ?, video_url = ?, updated_at = ? WHERE id = ? AND owner_id = ?",
		b.Title, b.URL, b.Description, string(tags), string(images), b.VideoURL, b.UpdatedAt, b.ID, b.OwnerID,
	)
	return err
}

func (r *BookmarkRepository) Delete(id, ownerID string) error {
	res, err := r.db.Exec("DELETE FROM bookmarks WHERE id = ? AND owner_id = ?", id, ownerID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// scanner abstracts over *sql.Row and *sql.Rows so a single scan routine serves
// both single-row and list reads.
type scanner interface {
	Scan(dest ...any) error
}

func scanBookmark(s scanner) (domain.Bookmark, error) {
	var b domain.Bookmark
	var tagsJSON, imagesJSON string
	if err := s.Scan(
		&b.ID, &b.OwnerID, &b.Title, &b.URL, &b.Description,
		&tagsJSON, &imagesJSON, &b.VideoURL, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return domain.Bookmark{}, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &b.Tags)
	_ = json.Unmarshal([]byte(imagesJSON), &b.ImageURLs)
	if b.Tags == nil {
		b.Tags = []string{}
	}
	if b.ImageURLs == nil {
		b.ImageURLs = []string{}
	}
	return b, nil
}
