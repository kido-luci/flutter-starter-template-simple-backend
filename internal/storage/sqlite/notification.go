package sqlite

import (
	"database/sql"

	"simple_backend_server/internal/domain"
)

// NotificationRepository is the SQLite-backed domain.NotificationRepository.
type NotificationRepository struct {
	db *sql.DB
}

var _ domain.NotificationRepository = (*NotificationRepository)(nil)

func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) ListByOwner(ownerID string) ([]domain.Notification, error) {
	rows, err := r.db.Query(
		"SELECT id, title, body, type, is_read, created_at FROM notifications WHERE owner_id = ? ORDER BY created_at DESC",
		ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Notification, 0)
	for rows.Next() {
		var n domain.Notification
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &n.Type, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *NotificationRepository) MarkRead(id, ownerID string) error {
	_, err := r.db.Exec(
		"UPDATE notifications SET is_read = 1 WHERE id = ? AND owner_id = ?", id, ownerID,
	)
	return err
}

func (r *NotificationRepository) Create(ownerID string, n domain.Notification) error {
	_, err := r.db.Exec(
		"INSERT INTO notifications (id, owner_id, title, body, type, is_read, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		n.ID, ownerID, n.Title, n.Body, n.Type, n.IsRead, n.CreatedAt,
	)
	return err
}
