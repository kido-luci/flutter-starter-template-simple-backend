package domain

import "time"

// Collection is a named folder of bookmarks owned by a single user. Membership
// is stored as a list of bookmark ids so the collection stays self-contained.
type Collection struct {
	ID          string
	OwnerID     string
	Name        string
	Icon        string
	Color       int
	BookmarkIDs []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CollectionRepository persists collections, scoped by owner.
type CollectionRepository interface {
	ListByOwner(ownerID string) ([]Collection, error)
	// GetOwned returns the owner's collection, or ErrNotFound.
	GetOwned(id, ownerID string) (Collection, error)
	// Create stores a new collection. It returns ErrConflict if the id is taken.
	Create(c Collection) error
	Update(c Collection) error
	// Delete removes the owner's collection, or returns ErrNotFound.
	Delete(id, ownerID string) error
}
