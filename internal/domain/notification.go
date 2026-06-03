package domain

import "time"

// Notification is a message shown to a user.
type Notification struct {
	ID        string
	Title     string
	Body      string
	Type      string
	IsRead    bool
	CreatedAt time.Time
}

// NotificationRepository persists user notifications.
type NotificationRepository interface {
	ListByOwner(ownerID string) ([]Notification, error)
	MarkRead(id, ownerID string) error
	Create(ownerID string, n Notification) error
}
