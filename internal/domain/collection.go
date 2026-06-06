package domain

import "time"

// Collection is a named folder of bookmarks owned by a single user. Membership
// is stored as a list of bookmark ids so the collection stays self-contained.
//
// Rev and DeletedAt carry the same offline-first sync semantics as on Bookmark:
// a per-owner revision used as a delta cursor + concurrency token, and a
// tombstone marker surfaced in delta pulls. See [Bookmark].
type Collection struct {
	ID          string
	OwnerID     string
	Name        string
	Icon        string
	Color       int
	BookmarkIDs []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Rev         int
	DeletedAt   *time.Time
}

// CollectionRepository persists collections, scoped by owner.
type CollectionRepository interface {
	// ListByOwner returns the owner's live (non-tombstoned) collections.
	ListByOwner(ownerID string) ([]Collection, error)
	// ListByOwnerSince returns the owner's collections (including tombstones)
	// whose Rev is greater than since, ordered by Rev ascending.
	ListByOwnerSince(ownerID string, since int) ([]Collection, error)
	// GetOwned returns the owner's live collection, or ErrNotFound.
	GetOwned(id, ownerID string) (Collection, error)
	// Create stores a new collection, assigning its Rev, and returns the stored
	// row. It returns ErrConflict if the id is taken.
	Create(c Collection) (Collection, error)
	// Update overwrites a live collection, assigning a fresh Rev, and returns
	// the stored row. When expectedRev is non-zero it must match the current
	// rev, otherwise ErrConflict. ErrNotFound if missing or tombstoned.
	Update(c Collection, expectedRev int) (Collection, error)
	// Delete soft-deletes (tombstones) the owner's collection, assigning a
	// fresh Rev. expectedRev semantics match Update. ErrNotFound if missing.
	Delete(id, ownerID string, expectedRev int) error
}
