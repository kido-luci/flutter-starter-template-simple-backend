package domain

import "time"

// Activity is an entry in a user's activity feed.
type Activity struct {
	ID          string
	Description string
	Type        string
	CreatedAt   time.Time
}

// ActivityRepository persists user activity-feed entries.
type ActivityRepository interface {
	ListByOwner(ownerID string) ([]Activity, error)
	Create(ownerID string, a Activity) error
}
