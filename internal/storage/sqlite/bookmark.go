package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"simple_backend_server/internal/domain"
)

const bookmarkColumns = "id, owner_id, title, url, description, tags, image_urls, video_url, created_at, updated_at, rev, deleted_at"

// nextRevExpr computes the next per-owner revision inline so the assignment is
// atomic with the write (SQLite's single writer serializes it). The bound
// parameter is the owner id.
const nextBookmarkRev = "(SELECT COALESCE(MAX(rev), 0) + 1 FROM bookmarks WHERE owner_id = ?)"

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
	return r.queryBookmarks(
		"SELECT "+bookmarkColumns+" FROM bookmarks WHERE owner_id = ? AND deleted_at IS NULL ORDER BY created_at DESC",
		ownerID,
	)
}

func (r *BookmarkRepository) ListByOwnerSince(ownerID string, since int) ([]domain.Bookmark, error) {
	return r.queryBookmarks(
		"SELECT "+bookmarkColumns+" FROM bookmarks WHERE owner_id = ? AND rev > ? ORDER BY rev ASC",
		ownerID, since,
	)
}

func (r *BookmarkRepository) queryBookmarks(query string, args ...any) ([]domain.Bookmark, error) {
	rows, err := r.db.Query(query, args...)
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
		"SELECT "+bookmarkColumns+" FROM bookmarks WHERE id = ? AND owner_id = ? AND deleted_at IS NULL",
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

func (r *BookmarkRepository) Create(b domain.Bookmark) (domain.Bookmark, error) {
	tags, _ := json.Marshal(b.Tags)
	images, _ := json.Marshal(b.ImageURLs)
	err := r.db.QueryRow(
		"INSERT INTO bookmarks ("+bookmarkColumns+") "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, "+nextBookmarkRev+", NULL) RETURNING rev",
		b.ID, b.OwnerID, b.Title, b.URL, b.Description, string(tags), string(images), b.VideoURL, b.CreatedAt, b.UpdatedAt, b.OwnerID,
	).Scan(&b.Rev)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.Bookmark{}, domain.ErrConflict
		}
		return domain.Bookmark{}, err
	}
	return b, nil
}

func (r *BookmarkRepository) Update(b domain.Bookmark, expectedRev int) (domain.Bookmark, error) {
	tags, _ := json.Marshal(b.Tags)
	images, _ := json.Marshal(b.ImageURLs)
	query := "UPDATE bookmarks SET title = ?, url = ?, description = ?, tags = ?, image_urls = ?, video_url = ?, updated_at = ?, rev = " + nextBookmarkRev +
		" WHERE id = ? AND owner_id = ? AND deleted_at IS NULL"
	args := []any{b.Title, b.URL, b.Description, string(tags), string(images), b.VideoURL, b.UpdatedAt, b.OwnerID, b.ID, b.OwnerID}
	if expectedRev != 0 {
		query += " AND rev = ?"
		args = append(args, expectedRev)
	}
	query += " RETURNING rev"

	err := r.db.QueryRow(query, args...).Scan(&b.Rev)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Bookmark{}, r.writeMiss(b.ID, b.OwnerID)
	}
	if err != nil {
		return domain.Bookmark{}, err
	}
	return b, nil
}

func (r *BookmarkRepository) Delete(id, ownerID string, expectedRev int) error {
	now := time.Now().UTC()
	query := "UPDATE bookmarks SET deleted_at = ?, rev = " + nextBookmarkRev +
		" WHERE id = ? AND owner_id = ? AND deleted_at IS NULL"
	args := []any{now, ownerID, id, ownerID}
	if expectedRev != 0 {
		query += " AND rev = ?"
		args = append(args, expectedRev)
	}

	res, err := r.db.Exec(query, args...)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return r.writeMiss(id, ownerID)
	}
	return nil
}

// writeMiss classifies why a rev-guarded write affected no rows: a live row
// that still exists means the expectedRev didn't match (conflict); otherwise the
// row is missing or already tombstoned (not found).
func (r *BookmarkRepository) writeMiss(id, ownerID string) error {
	var rev int
	err := r.db.QueryRow(
		"SELECT rev FROM bookmarks WHERE id = ? AND owner_id = ? AND deleted_at IS NULL",
		id, ownerID,
	).Scan(&rev)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	return domain.ErrConflict
}

// scanner abstracts over *sql.Row and *sql.Rows so a single scan routine serves
// both single-row and list reads.
type scanner interface {
	Scan(dest ...any) error
}

func scanBookmark(s scanner) (domain.Bookmark, error) {
	var b domain.Bookmark
	var tagsJSON, imagesJSON string
	var deletedAt sql.NullTime
	if err := s.Scan(
		&b.ID, &b.OwnerID, &b.Title, &b.URL, &b.Description,
		&tagsJSON, &imagesJSON, &b.VideoURL, &b.CreatedAt, &b.UpdatedAt, &b.Rev, &deletedAt,
	); err != nil {
		return domain.Bookmark{}, err
	}
	if err := json.Unmarshal([]byte(tagsJSON), &b.Tags); err != nil {
		return domain.Bookmark{}, err
	}
	if err := json.Unmarshal([]byte(imagesJSON), &b.ImageURLs); err != nil {
		return domain.Bookmark{}, err
	}
	if b.Tags == nil {
		b.Tags = []string{}
	}
	if b.ImageURLs == nil {
		b.ImageURLs = []string{}
	}
	if deletedAt.Valid {
		t := deletedAt.Time.UTC()
		b.DeletedAt = &t
	}
	return b, nil
}
