package sqlite_test

import (
	"errors"
	"testing"
	"time"

	"simple_backend_server/internal/domain"
	"simple_backend_server/internal/storage/sqlite"
)

func TestCollectionRepository_RoundTripWithJSONMembership(t *testing.T) {
	repo := sqlite.NewCollectionRepository(newTestDB(t))
	now := time.Now().UTC().Truncate(time.Second)

	c := domain.Collection{
		ID:          "c1",
		OwnerID:     "owner-1",
		Name:        "Design",
		Icon:        "f5fd",
		Color:       0xFF6366F1,
		BookmarkIDs: []string{"b1", "b2"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repo.Create(c); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	got, err := repo.GetOwned("c1", "owner-1")
	if err != nil {
		t.Fatalf("GetOwned error: %v", err)
	}
	if got.Name != "Design" || got.Color != 0xFF6366F1 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if len(got.BookmarkIDs) != 2 || got.BookmarkIDs[0] != "b1" || got.BookmarkIDs[1] != "b2" {
		t.Errorf("bookmarkIDs = %v, want [b1 b2]", got.BookmarkIDs)
	}
}

func TestCollectionRepository_EmptyMembershipDecodesToNonNil(t *testing.T) {
	repo := sqlite.NewCollectionRepository(newTestDB(t))
	now := time.Now().UTC()

	c := domain.Collection{ID: "c1", OwnerID: "o", Name: "N", BookmarkIDs: []string{}, CreatedAt: now, UpdatedAt: now}
	if err := repo.Create(c); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	got, err := repo.GetOwned("c1", "o")
	if err != nil {
		t.Fatalf("GetOwned error: %v", err)
	}
	if got.BookmarkIDs == nil {
		t.Error("BookmarkIDs should decode to a non-nil empty slice")
	}
}

func TestCollectionRepository_NotFoundConflictAndOwnerScoping(t *testing.T) {
	repo := sqlite.NewCollectionRepository(newTestDB(t))
	now := time.Now().UTC()
	c := domain.Collection{ID: "c1", OwnerID: "owner-1", Name: "N", BookmarkIDs: []string{}, CreatedAt: now, UpdatedAt: now}
	if err := repo.Create(c); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if err := repo.Create(c); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("duplicate Create = %v, want ErrConflict", err)
	}
	if _, err := repo.GetOwned("c1", "other"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("cross-owner Get = %v, want ErrNotFound", err)
	}
	if err := repo.Delete("c1", "other"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("cross-owner Delete = %v, want ErrNotFound", err)
	}
}

func TestCollectionRepository_UpdateMissingRowIsNotFound(t *testing.T) {
	repo := sqlite.NewCollectionRepository(newTestDB(t))
	now := time.Now().UTC()
	c := domain.Collection{ID: "missing", OwnerID: "owner-1", Name: "N", BookmarkIDs: []string{}, CreatedAt: now, UpdatedAt: now}
	if err := repo.Update(c); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Update missing = %v, want ErrNotFound", err)
	}
}
