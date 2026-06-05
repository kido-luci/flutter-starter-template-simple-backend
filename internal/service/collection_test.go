package service_test

import (
	"errors"
	"testing"

	"simple_backend_server/internal/domain"
	"simple_backend_server/internal/service"
)

type collectionFixture struct {
	svc           *service.CollectionService
	collections   *fakeCollectionRepo
	activities    *fakeActivityRepo
	notifications *fakeNotificationRepo
	ids           *fakeIDGen
}

func newCollectionFixture() *collectionFixture {
	f := &collectionFixture{
		collections:   newFakeCollectionRepo(),
		activities:    &fakeActivityRepo{},
		notifications: &fakeNotificationRepo{},
		ids:           &fakeIDGen{},
	}
	f.svc = service.NewCollectionService(f.collections, f.activities, f.notifications, f.ids)
	return f
}

func (f *collectionFixture) mustCreate(t *testing.T, in service.CollectionInput) domain.Collection {
	t.Helper()
	c, err := f.svc.Create("owner-1", in)
	if err != nil {
		t.Fatalf("setup Create failed: %v", err)
	}
	return c
}

func TestCollectionService_Create_GeneratesID_NormalizesAndRecordsSideEffects(t *testing.T) {
	f := newCollectionFixture()

	in := service.CollectionInput{
		Name:        "  Design  ",
		Icon:        "f5fd",
		Color:       0xFF6366F1,
		BookmarkIDs: []string{"b1", "b1", " b2 ", ""},
	}
	c, err := f.svc.Create("owner-1", in)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if c.ID != "id-1" {
		t.Errorf("generated id = %q, want id-1", c.ID)
	}
	if c.OwnerID != "owner-1" {
		t.Errorf("owner = %q, want owner-1", c.OwnerID)
	}
	if c.Name != "Design" {
		t.Errorf("name = %q, want trimmed 'Design'", c.Name)
	}
	if c.Color != 0xFF6366F1 {
		t.Errorf("color = %d, want %d", c.Color, 0xFF6366F1)
	}
	if got := c.BookmarkIDs; len(got) != 2 || got[0] != "b1" || got[1] != "b2" {
		t.Errorf("bookmarkIDs = %v, want [b1 b2] (trimmed + deduped)", got)
	}
	if c.CreatedAt.IsZero() || !c.CreatedAt.Equal(c.UpdatedAt) {
		t.Errorf("timestamps not set consistently: created=%v updated=%v", c.CreatedAt, c.UpdatedAt)
	}

	if len(f.activities.created) != 1 {
		t.Fatalf("activity count = %d, want 1", len(f.activities.created))
	}
	act := f.activities.created[0]
	if act.ownerID != "owner-1" || act.activity.Type != "collection_created" {
		t.Errorf("activity = %+v, want owner-1 / collection_created", act)
	}
	if len(f.notifications.created) != 1 {
		t.Fatalf("notification count = %d, want 1", len(f.notifications.created))
	}
}

func TestCollectionService_Create_HonorsClientID_AndConflict(t *testing.T) {
	f := newCollectionFixture()

	first, err := f.svc.Create("owner-1", service.CollectionInput{ID: "c-1", Name: "A"})
	if err != nil {
		t.Fatalf("first Create error: %v", err)
	}
	if first.ID != "c-1" {
		t.Errorf("id = %q, want client-provided c-1", first.ID)
	}

	_, err = f.svc.Create("owner-1", service.CollectionInput{ID: "c-1", Name: "B"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("duplicate id error = %v, want ErrConflict", err)
	}
}

func TestCollectionService_Create_RequiresName(t *testing.T) {
	f := newCollectionFixture()

	_, err := f.svc.Create("owner-1", service.CollectionInput{Name: "   "})
	var ve domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error = %v, want ValidationError", err)
	}
}

func TestCollectionService_Update_OverwritesOwnedCollection(t *testing.T) {
	f := newCollectionFixture()
	created := f.mustCreate(t, service.CollectionInput{Name: "Old", BookmarkIDs: []string{"b1"}})

	updated, err := f.svc.Update(created.ID, "owner-1", service.CollectionInput{
		Name:        "New",
		Icon:        "f02d",
		Color:       0xFF10B981,
		BookmarkIDs: []string{"b2", "b3"},
	})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if updated.Name != "New" || updated.Color != 0xFF10B981 {
		t.Errorf("update did not apply fields: %+v", updated)
	}
	if len(updated.BookmarkIDs) != 2 || updated.BookmarkIDs[0] != "b2" {
		t.Errorf("bookmarkIDs = %v, want [b2 b3]", updated.BookmarkIDs)
	}
}

func TestCollectionService_Update_MissingIsNotFound(t *testing.T) {
	f := newCollectionFixture()

	_, err := f.svc.Update("missing", "owner-1", service.CollectionInput{Name: "X"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestCollectionService_Delete_ScopesToOwner(t *testing.T) {
	f := newCollectionFixture()
	created := f.mustCreate(t, service.CollectionInput{Name: "A"})

	if err := f.svc.Delete(created.ID, "other"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("delete by non-owner = %v, want ErrNotFound", err)
	}
	if err := f.svc.Delete(created.ID, "owner-1"); err != nil {
		t.Errorf("delete by owner = %v, want nil", err)
	}
}
