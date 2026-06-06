package domain

import "time"

// Bookmark is a saved link owned by a single user.
//
// Rev is a per-owner, strictly increasing revision reassigned on every write.
// It drives offline-first sync: clients use it as a delta cursor (pull rows
// with rev greater than the last seen) and as an optimistic-concurrency token
// (an update echoes the rev it was based on; a mismatch is a conflict).
// DeletedAt marks a tombstone: a soft-deleted row that still surfaces in delta
// pulls so clients can remove their local copy.
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
	Rev         int
	DeletedAt   *time.Time
}

// BookmarkRepository persists bookmarks, scoped by owner.
type BookmarkRepository interface {
	// ListByOwner returns the owner's live (non-tombstoned) bookmarks.
	ListByOwner(ownerID string) ([]Bookmark, error)
	// ListByOwnerSince returns the owner's bookmarks (including tombstones)
	// whose Rev is greater than since, ordered by Rev ascending.
	ListByOwnerSince(ownerID string, since int) ([]Bookmark, error)
	// GetOwned returns the owner's live bookmark, or ErrNotFound.
	GetOwned(id, ownerID string) (Bookmark, error)
	// Create stores a new bookmark, assigning its Rev, and returns the stored
	// row. It returns ErrConflict if the id is taken.
	Create(b Bookmark) (Bookmark, error)
	// Update overwrites a live bookmark, assigning a fresh Rev, and returns the
	// stored row. When expectedRev is non-zero it must match the current rev,
	// otherwise ErrConflict. ErrNotFound if the row is missing or tombstoned.
	Update(b Bookmark, expectedRev int) (Bookmark, error)
	// Delete soft-deletes (tombstones) the owner's bookmark, assigning a fresh
	// Rev. expectedRev semantics match Update. ErrNotFound if missing.
	Delete(id, ownerID string, expectedRev int) error
}
