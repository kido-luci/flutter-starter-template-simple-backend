package domain

import "time"

// Bookmark is a saved link owned by a single user.
type Bookmark struct {
	ID          string
	OwnerID     string
	Title       string
	URL         string
	Description string
	Tags        []string
	ImageURLs   []string
	VideoURL    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// BookmarkRepository persists bookmarks, scoped by owner.
type BookmarkRepository interface {
	ListByOwner(ownerID string) ([]Bookmark, error)
	// GetOwned returns the owner's bookmark, or ErrNotFound.
	GetOwned(id, ownerID string) (Bookmark, error)
	// Create stores a new bookmark. It returns ErrConflict if the id is taken.
	Create(b Bookmark) error
	Update(b Bookmark) error
	// Delete removes the owner's bookmark, or returns ErrNotFound.
	Delete(id, ownerID string) error
}
