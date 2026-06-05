package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"simple_backend_server/internal/domain"
)

const collectionColumns = "id, owner_id, name, icon, color, bookmark_ids, created_at, updated_at"

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
	rows, err := r.db.Query(
		"SELECT "+collectionColumns+" FROM collections WHERE owner_id = ? ORDER BY created_at DESC",
		ownerID,
	)
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
		"SELECT "+collectionColumns+" FROM collections WHERE id = ? AND owner_id = ?",
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

func (r *CollectionRepository) Create(c domain.Collection) error {
	ids, err := json.Marshal(c.BookmarkIDs)
	if err != nil {
		return err
	}
	if _, err := r.db.Exec(
		"INSERT INTO collections ("+collectionColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		c.ID, c.OwnerID, c.Name, c.Icon, c.Color, string(ids), c.CreatedAt, c.UpdatedAt,
	); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.ErrConflict
		}
		return err
	}
	return nil
}

func (r *CollectionRepository) Update(c domain.Collection) error {
	ids, err := json.Marshal(c.BookmarkIDs)
	if err != nil {
		return err
	}
	res, err := r.db.Exec(
		"UPDATE collections SET name = ?, icon = ?, color = ?, bookmark_ids = ?, updated_at = ? WHERE id = ? AND owner_id = ?",
		c.Name, c.Icon, c.Color, string(ids), c.UpdatedAt, c.ID, c.OwnerID,
	)
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

func (r *CollectionRepository) Delete(id, ownerID string) error {
	res, err := r.db.Exec("DELETE FROM collections WHERE id = ? AND owner_id = ?", id, ownerID)
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

func scanCollection(s scanner) (domain.Collection, error) {
	var c domain.Collection
	var idsJSON sql.NullString
	if err := s.Scan(
		&c.ID, &c.OwnerID, &c.Name, &c.Icon, &c.Color, &idsJSON, &c.CreatedAt, &c.UpdatedAt,
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
	return c, nil
}
