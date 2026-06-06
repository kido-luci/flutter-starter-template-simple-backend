package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"simple_backend_server/internal/domain"
)

const collectionColumns = "id, owner_id, name, icon, color, bookmark_ids, created_at, updated_at, rev, deleted_at"

const nextCollectionRev = "(SELECT COALESCE(MAX(rev), 0) + 1 FROM collections WHERE owner_id = ?)"

// CollectionRepository is the SQLite-backed domain.CollectionRepository. The
// bookmark-id slice is stored as a JSON text column.
type CollectionRepository struct {
	db *sql.DB
}

var _ domain.CollectionRepository = (*CollectionRepository)(nil)

func NewCollectionRepository(db *sql.DB) *CollectionRepository {
	return &CollectionRepository{db: db}
}

func (r *CollectionRepository) ListByOwner(ownerID string) ([]domain.Collection, error) {
	return r.queryCollections(
		"SELECT "+collectionColumns+" FROM collections WHERE owner_id = ? AND deleted_at IS NULL ORDER BY created_at DESC",
		ownerID,
	)
}

func (r *CollectionRepository) ListByOwnerSince(ownerID string, since int) ([]domain.Collection, error) {
	return r.queryCollections(
		"SELECT "+collectionColumns+" FROM collections WHERE owner_id = ? AND rev > ? ORDER BY rev ASC",
		ownerID, since,
	)
}

func (r *CollectionRepository) queryCollections(query string, args ...any) ([]domain.Collection, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Collection, 0)
	for rows.Next() {
		c, err := scanCollection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CollectionRepository) GetOwned(id, ownerID string) (domain.Collection, error) {
	row := r.db.QueryRow(
		"SELECT "+collectionColumns+" FROM collections WHERE id = ? AND owner_id = ? AND deleted_at IS NULL",
		id, ownerID,
	)
	c, err := scanCollection(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Collection{}, domain.ErrNotFound
		}
		return domain.Collection{}, err
	}
	return c, nil
}

func (r *CollectionRepository) Create(c domain.Collection) (domain.Collection, error) {
	ids, err := json.Marshal(c.BookmarkIDs)
	if err != nil {
		return domain.Collection{}, err
	}
	err = r.db.QueryRow(
		"INSERT INTO collections ("+collectionColumns+") "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, "+nextCollectionRev+", NULL) RETURNING rev",
		c.ID, c.OwnerID, c.Name, c.Icon, c.Color, string(ids), c.CreatedAt, c.UpdatedAt, c.OwnerID,
	).Scan(&c.Rev)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.Collection{}, domain.ErrConflict
		}
		return domain.Collection{}, err
	}
	return c, nil
}

func (r *CollectionRepository) Update(c domain.Collection, expectedRev int) (domain.Collection, error) {
	ids, err := json.Marshal(c.BookmarkIDs)
	if err != nil {
		return domain.Collection{}, err
	}
	query := "UPDATE collections SET name = ?, icon = ?, color = ?, bookmark_ids = ?, updated_at = ?, rev = " + nextCollectionRev +
		" WHERE id = ? AND owner_id = ? AND deleted_at IS NULL"
	args := []any{c.Name, c.Icon, c.Color, string(ids), c.UpdatedAt, c.OwnerID, c.ID, c.OwnerID}
	if expectedRev != 0 {
		query += " AND rev = ?"
		args = append(args, expectedRev)
	}
	query += " RETURNING rev"

	err = r.db.QueryRow(query, args...).Scan(&c.Rev)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Collection{}, r.writeMiss(c.ID, c.OwnerID)
	}
	if err != nil {
		return domain.Collection{}, err
	}
	return c, nil
}

func (r *CollectionRepository) Delete(id, ownerID string, expectedRev int) error {
	now := time.Now().UTC()
	query := "UPDATE collections SET deleted_at = ?, rev = " + nextCollectionRev +
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

func (r *CollectionRepository) writeMiss(id, ownerID string) error {
	var rev int
	err := r.db.QueryRow(
		"SELECT rev FROM collections WHERE id = ? AND owner_id = ? AND deleted_at IS NULL",
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

func scanCollection(s scanner) (domain.Collection, error) {
	var c domain.Collection
	var idsJSON sql.NullString
	var deletedAt sql.NullTime
	if err := s.Scan(
		&c.ID, &c.OwnerID, &c.Name, &c.Icon, &c.Color, &idsJSON, &c.CreatedAt, &c.UpdatedAt, &c.Rev, &deletedAt,
	); err != nil {
		return domain.Collection{}, err
	}
	// Tolerate NULL/empty membership columns (legacy rows) by defaulting to [].
	raw := idsJSON.String
	if !idsJSON.Valid || raw == "" {
		raw = "[]"
	}
	if err := json.Unmarshal([]byte(raw), &c.BookmarkIDs); err != nil {
		return domain.Collection{}, err
	}
	if c.BookmarkIDs == nil {
		c.BookmarkIDs = []string{}
	}
	if deletedAt.Valid {
		t := deletedAt.Time.UTC()
		c.DeletedAt = &t
	}
	return c, nil
}
