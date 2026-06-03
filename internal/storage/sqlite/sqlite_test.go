package sqlite_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"simple_backend_server/internal/domain"
	"simple_backend_server/internal/storage/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestUserRepository_CreateAndLookup(t *testing.T) {
	repo := sqlite.NewUserRepository(newTestDB(t))
	u := domain.User{ID: "u1", Username: "alice"}

	if err := repo.Create(u, "hash1"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, hash, err := repo.FindByUsername("alice")
	if err != nil {
		t.Fatalf("FindByUsername failed: %v", err)
	}
	if got != u || hash != "hash1" {
		t.Errorf("got %+v / %q, want %+v / hash1", got, hash, u)
	}

	if _, _, err := repo.FindByUsername("ghost"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("missing user err = %v, want domain.ErrNotFound", err)
	}
}

func TestUserRepository_DuplicateUsernameConflict(t *testing.T) {
	repo := sqlite.NewUserRepository(newTestDB(t))
	if err := repo.Create(domain.User{ID: "u1", Username: "alice"}, "h"); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	err := repo.Create(domain.User{ID: "u2", Username: "alice"}, "h")
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("err = %v, want domain.ErrConflict", err)
	}
}

func TestUserRepository_UpdatePassword(t *testing.T) {
	repo := sqlite.NewUserRepository(newTestDB(t))
	if err := repo.Create(domain.User{ID: "u1", Username: "alice"}, "old"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.UpdatePassword("u1", "new"); err != nil {
		t.Fatalf("UpdatePassword failed: %v", err)
	}
	hash, err := repo.PasswordHash("u1")
	if err != nil {
		t.Fatalf("PasswordHash failed: %v", err)
	}
	if hash != "new" {
		t.Errorf("hash = %q, want new", hash)
	}
}

func TestBookmarkRepository_RoundTripWithJSONFields(t *testing.T) {
	repo := sqlite.NewBookmarkRepository(newTestDB(t))
	now := time.Now().UTC().Truncate(time.Second)
	b := domain.Bookmark{
		ID:        "bm1",
		OwnerID:   "owner-1",
		Title:     "Go",
		URL:       "https://go.dev",
		Tags:      []string{"lang", "tools"},
		ImageURLs: []string{"https://x/i.png"},
		VideoURL:  "https://x/v.mp4",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.Create(b); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.GetOwned("bm1", "owner-1")
	if err != nil {
		t.Fatalf("GetOwned failed: %v", err)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "lang" || got.Tags[1] != "tools" {
		t.Errorf("tags = %v, want [lang tools]", got.Tags)
	}
	if len(got.ImageURLs) != 1 || got.ImageURLs[0] != "https://x/i.png" {
		t.Errorf("image_urls = %v", got.ImageURLs)
	}
	if got.VideoURL != "https://x/v.mp4" {
		t.Errorf("video_url = %q", got.VideoURL)
	}
}

func TestBookmarkRepository_EmptyJSONFieldsDecodeToNonNil(t *testing.T) {
	repo := sqlite.NewBookmarkRepository(newTestDB(t))
	now := time.Now().UTC()
	b := domain.Bookmark{
		ID: "bm1", OwnerID: "owner-1", Title: "T", URL: "https://x",
		Tags: []string{}, ImageURLs: []string{}, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(b); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.GetOwned("bm1", "owner-1")
	if err != nil {
		t.Fatalf("GetOwned failed: %v", err)
	}
	if got.Tags == nil || got.ImageURLs == nil {
		t.Errorf("slices should decode to non-nil: tags=%v images=%v", got.Tags, got.ImageURLs)
	}
}

func TestBookmarkRepository_NotFoundAndConflictAndOwnerScoping(t *testing.T) {
	repo := sqlite.NewBookmarkRepository(newTestDB(t))
	now := time.Now().UTC()
	b := domain.Bookmark{ID: "bm1", OwnerID: "owner-1", Title: "T", URL: "https://x", Tags: []string{}, ImageURLs: []string{}, CreatedAt: now, UpdatedAt: now}
	if err := repo.Create(b); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Create(b); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("duplicate id err = %v, want domain.ErrConflict", err)
	}
	if _, err := repo.GetOwned("bm1", "intruder"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("cross-owner GetOwned err = %v, want domain.ErrNotFound", err)
	}
	if err := repo.Delete("bm1", "intruder"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("cross-owner Delete err = %v, want domain.ErrNotFound", err)
	}
	if err := repo.Delete("bm1", "owner-1"); err != nil {
		t.Errorf("owner Delete failed: %v", err)
	}
}

func TestBookmarkRepository_UpdateMissingRowIsNotFound(t *testing.T) {
	repo := sqlite.NewBookmarkRepository(newTestDB(t))
	now := time.Now().UTC()
	b := domain.Bookmark{ID: "bm1", OwnerID: "owner-1", Title: "T", URL: "https://x", Tags: []string{}, ImageURLs: []string{}, CreatedAt: now, UpdatedAt: now}

	// Updating a row that was never created must report not-found rather than
	// silently succeeding (guards the Get-then-Update race).
	if err := repo.Update(b); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Update of missing row err = %v, want domain.ErrNotFound", err)
	}
}

func TestRefreshTokenRepository_RotateConsumesOldToken(t *testing.T) {
	db := newTestDB(t)
	users := sqlite.NewUserRepository(db)
	tokens := sqlite.NewRefreshTokenRepository(db)
	if err := users.Create(domain.User{ID: "u1", Username: "alice"}, "h"); err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
	if err := tokens.Issue("old", "u1", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	u, err := tokens.Rotate("old", "new", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("rotated user = %+v, want alice", u)
	}

	// Old token is single-use: a second rotation must fail.
	_, err = tokens.Rotate("old", "newer", time.Now().Add(time.Hour))
	var re domain.RefreshError
	if !errors.As(err, &re) {
		t.Errorf("reuse err = %v, want domain.RefreshError", err)
	}
}

func TestRefreshTokenRepository_RotateExpired(t *testing.T) {
	db := newTestDB(t)
	tokens := sqlite.NewRefreshTokenRepository(db)
	if err := tokens.Issue("old", "u1", time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	_, err := tokens.Rotate("old", "new", time.Now().Add(time.Hour))
	var re domain.RefreshError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want domain.RefreshError", err)
	}
	if re.Error() != "refresh token is expired" {
		t.Errorf("message = %q, want 'refresh token is expired'", re.Error())
	}

	// A failed rotation rolls back, so the token remains and a retry still
	// reports it as expired rather than disappearing.
	_, err = tokens.Rotate("old", "new", time.Now().Add(time.Hour))
	var re2 domain.RefreshError
	if !errors.As(err, &re2) || re2.Error() != "refresh token is expired" {
		t.Errorf("second attempt err = %v, want 'refresh token is expired'", err)
	}
}

func TestNotificationRepository_CreateListMarkRead(t *testing.T) {
	repo := sqlite.NewNotificationRepository(newTestDB(t))
	now := time.Now().UTC()
	if err := repo.Create("owner-1", domain.Notification{ID: "n1", Title: "Hi", Body: "b", Type: "system", CreatedAt: now}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	list, err := repo.ListByOwner("owner-1")
	if err != nil {
		t.Fatalf("ListByOwner failed: %v", err)
	}
	if len(list) != 1 || list[0].IsRead {
		t.Fatalf("list = %v, want one unread notification", list)
	}

	if err := repo.MarkRead("n1", "owner-1"); err != nil {
		t.Fatalf("MarkRead failed: %v", err)
	}
	list, _ = repo.ListByOwner("owner-1")
	if !list[0].IsRead {
		t.Error("notification should be marked read")
	}
}
