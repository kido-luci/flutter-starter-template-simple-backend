package sqlite

import (
	"database/sql"

	"simple_backend_server/internal/domain"
)

// ActivityRepository is the SQLite-backed domain.ActivityRepository.
type ActivityRepository struct {
	db *sql.DB
}

var _ domain.ActivityRepository = (*ActivityRepository)(nil)

func NewActivityRepository(db *sql.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

func (r *ActivityRepository) ListByOwner(ownerID string) ([]domain.Activity, error) {
	rows, err := r.db.Query(
		"SELECT id, description, type, created_at FROM activities WHERE owner_id = ? ORDER BY created_at DESC",
		ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Activity, 0)
	for rows.Next() {
		var a domain.Activity
		if err := rows.Scan(&a.ID, &a.Description, &a.Type, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *ActivityRepository) Create(ownerID string, a domain.Activity) error {
	_, err := r.db.Exec(
		"INSERT INTO activities (id, owner_id, description, type, created_at) VALUES (?, ?, ?, ?, ?)",
		a.ID, ownerID, a.Description, a.Type, a.CreatedAt,
	)
	return err
}
