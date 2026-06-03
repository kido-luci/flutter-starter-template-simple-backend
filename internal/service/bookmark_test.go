package service_test

import (
	"errors"
	"testing"

	"simple_backend_server/internal/domain"
	"simple_backend_server/internal/service"
)

type bookmarkFixture struct {
	svc           *service.BookmarkService
	bookmarks     *fakeBookmarkRepo
	activities    *fakeActivityRepo
	notifications *fakeNotificationRepo
	ids           *fakeIDGen
}

func newBookmarkFixture() *bookmarkFixture {
	f := &bookmarkFixture{
		bookmarks:     newFakeBookmarkRepo(),
		activities:    &fakeActivityRepo{},
		notifications: &fakeNotificationRepo{},
		ids:           &fakeIDGen{},
	}
	f.svc = service.NewBookmarkService(f.bookmarks, f.activities, f.notifications, f.ids)
	return f
}

func TestBookmarkService_Create_GeneratesID_NormalizesAndRecordsSideEffects(t *testing.T) {
	f := newBookmarkFixture()

	in := service.BookmarkInput{
		Title:     "Go",
		URL:       "https://go.dev",
		Tags:      []string{"a", "a", " b ", ""},
		ImageURLs: nil,
	}
	b, err := f.svc.Create("owner-1", in)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if b.ID != "id-1" {
		t.Errorf("generated id = %q, want id-1", b.ID)
	}
	if b.OwnerID != "owner-1" {
		t.Errorf("owner = %q, want owner-1", b.OwnerID)
	}
	if got := b.Tags; len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("tags = %v, want [a b] (trimmed + deduped)", got)
	}
	if b.ImageURLs == nil {
		t.Error("ImageURLs should be a non-nil empty slice, not nil")
	}
	if len(b.ImageURLs) != 0 {
		t.Errorf("ImageURLs = %v, want []", b.ImageURLs)
	}
	if b.CreatedAt.IsZero() || !b.CreatedAt.Equal(b.UpdatedAt) {
		t.Errorf("timestamps not set consistently: created=%v updated=%v", b.CreatedAt, b.UpdatedAt)
	}

	if len(f.activities.created) != 1 {
		t.Fatalf("activity count = %d, want 1", len(f.activities.created))
	}
	act := f.activities.created[0]
	if act.ownerID != "owner-1" || act.activity.Type != "bookmark_created" {
		t.Errorf("activity = %+v, want owner-1 / bookmark_created", act)
	}
	if act.activity.Description != "Created a new bookmark 'Go'" {
		t.Errorf("activity description = %q", act.activity.Description)
	}

	if len(f.notifications.created) != 1 {
		t.Fatalf("notification count = %d, want 1", len(f.notifications.created))
	}
	notif := f.notifications.created[0]
	if notif.ownerID != "owner-1" || notif.notification.Title != "New Bookmark" {
		t.Errorf("notification = %+v, want owner-1 / 'New Bookmark'", notif)
	}
}

func TestBookmarkService_Create_HonorsClientID(t *testing.T) {
	f := newBookmarkFixture()

	b, err := f.svc.Create("owner-1", service.BookmarkInput{ID: "bm1", Title: "T", URL: "https://x"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if b.ID != "bm1" {
		t.Errorf("id = %q, want bm1 (client-provided)", b.ID)
	}
	if _, ok := f.bookmarks.items["bm1"]; !ok {
		t.Error("bookmark not persisted under client id")
	}
}

func TestBookmarkService_Create_Validation(t *testing.T) {
	tests := []struct {
		name    string
		in      service.BookmarkInput
		wantMsg string
	}{
		{"missing title", service.BookmarkInput{URL: "https://x"}, "title is required"},
		{"blank title", service.BookmarkInput{Title: "   ", URL: "https://x"}, "title is required"},
		{"missing url", service.BookmarkInput{Title: "T"}, "url is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newBookmarkFixture()
			_, err := f.svc.Create("owner-1", tc.in)
			var ve domain.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want domain.ValidationError", err)
			}
			if ve.Error() != tc.wantMsg {
				t.Errorf("message = %q, want %q", ve.Error(), tc.wantMsg)
			}
			if len(f.bookmarks.items) != 0 {
				t.Error("no bookmark should be stored on validation failure")
			}
			if len(f.activities.created) != 0 || len(f.notifications.created) != 0 {
				t.Error("no side effects should be recorded on validation failure")
			}
		})
	}
}

func TestBookmarkService_Create_ConflictSkipsSideEffects(t *testing.T) {
	f := newBookmarkFixture()
	if _, err := f.svc.Create("owner-1", service.BookmarkInput{ID: "bm1", Title: "T", URL: "https://x"}); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}
	activitiesBefore := len(f.activities.created)

	_, err := f.svc.Create("owner-1", service.BookmarkInput{ID: "bm1", Title: "Dup", URL: "https://y"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("err = %v, want domain.ErrConflict", err)
	}
	if len(f.activities.created) != activitiesBefore {
		t.Error("conflict should not record additional activity")
	}
}

func TestBookmarkService_Update_Success(t *testing.T) {
	f := newBookmarkFixture()
	created, err := f.svc.Create("owner-1", service.BookmarkInput{ID: "bm1", Title: "Old", URL: "https://old"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := f.svc.Update("bm1", "owner-1", service.BookmarkInput{
		Title: "New", URL: "https://new", Tags: []string{"x", "x"},
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Title != "New" || updated.URL != "https://new" {
		t.Errorf("fields not updated: %+v", updated)
	}
	if len(updated.Tags) != 1 || updated.Tags[0] != "x" {
		t.Errorf("tags = %v, want [x]", updated.Tags)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Error("CreatedAt should be preserved across update")
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("UpdatedAt should advance: created=%v updated=%v", created.UpdatedAt, updated.UpdatedAt)
	}
}

func TestBookmarkService_Update_NotFound(t *testing.T) {
	f := newBookmarkFixture()

	_, err := f.svc.Update("ghost", "owner-1", service.BookmarkInput{Title: "T", URL: "https://x"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want domain.ErrNotFound", err)
	}
}

func TestBookmarkService_Update_OtherOwnerIsNotFound(t *testing.T) {
	f := newBookmarkFixture()
	if _, err := f.svc.Create("owner-1", service.BookmarkInput{ID: "bm1", Title: "T", URL: "https://x"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err := f.svc.Update("bm1", "intruder", service.BookmarkInput{Title: "T", URL: "https://x"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want domain.ErrNotFound for cross-owner update", err)
	}
}

func TestBookmarkService_Update_Validation(t *testing.T) {
	f := newBookmarkFixture()

	_, err := f.svc.Update("bm1", "owner-1", service.BookmarkInput{Title: "", URL: "https://x"})
	var ve domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want domain.ValidationError", err)
	}
}

func TestBookmarkService_Delete(t *testing.T) {
	f := newBookmarkFixture()
	if _, err := f.svc.Create("owner-1", service.BookmarkInput{ID: "bm1", Title: "T", URL: "https://x"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := f.svc.Delete("bm1", "owner-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, ok := f.bookmarks.items["bm1"]; ok {
		t.Error("bookmark should be removed")
	}

	if err := f.svc.Delete("bm1", "owner-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("second Delete err = %v, want domain.ErrNotFound", err)
	}
}

func TestBookmarkService_ListAndGet(t *testing.T) {
	f := newBookmarkFixture()
	if _, err := f.svc.Create("owner-1", service.BookmarkInput{ID: "bm1", Title: "A", URL: "https://a"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := f.svc.Create("owner-2", service.BookmarkInput{ID: "bm2", Title: "B", URL: "https://b"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	list, err := f.svc.List("owner-1")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 || list[0].ID != "bm1" {
		t.Errorf("List(owner-1) = %v, want only bm1", list)
	}

	got, err := f.svc.Get("bm1", "owner-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Title != "A" {
		t.Errorf("Get title = %q, want A", got.Title)
	}

	if _, err := f.svc.Get("bm2", "owner-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("cross-owner Get err = %v, want domain.ErrNotFound", err)
	}
}
