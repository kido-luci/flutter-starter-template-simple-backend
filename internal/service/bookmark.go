package service

import (
	"strings"
	"time"

	"simple_backend_server/internal/domain"
)

// BookmarkInput carries the mutable fields of a bookmark from the transport
// layer. ID is honored only on Create (offline-first clients may mint ids).
type BookmarkInput struct {
	ID          string
	Title       string
	URL         string
	Description string
	Tags        []string
	ImageURLs   []string
	VideoURL    string
}

// BookmarkService implements the bookmark use cases. Creating a bookmark also
// records an activity-feed entry and a notification for the owner.
type BookmarkService struct {
	bookmarks     domain.BookmarkRepository
	activities    domain.ActivityRepository
	notifications domain.NotificationRepository
	ids           IDGenerator
	now           func() time.Time
}

func NewBookmarkService(
	bookmarks domain.BookmarkRepository,
	activities domain.ActivityRepository,
	notifications domain.NotificationRepository,
	ids IDGenerator,
) *BookmarkService {
	return &BookmarkService{
		bookmarks:     bookmarks,
		activities:    activities,
		notifications: notifications,
		ids:           ids,
		now:           time.Now,
	}
}

func (s *BookmarkService) List(ownerID string) ([]domain.Bookmark, error) {
	return s.bookmarks.ListByOwner(ownerID)
}

func (s *BookmarkService) Get(id, ownerID string) (domain.Bookmark, error) {
	return s.bookmarks.GetOwned(id, ownerID)
}

// Create stores a new bookmark for the owner and records the side-effect
// activity and notification (best-effort). It returns a domain.ValidationError
// for missing fields and domain.ErrConflict if a client-provided id is taken.
func (s *BookmarkService) Create(ownerID string, in BookmarkInput) (domain.Bookmark, error) {
	if err := validateBookmark(in); err != nil {
		return domain.Bookmark{}, err
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		generated, err := s.ids.NewID()
		if err != nil {
			return domain.Bookmark{}, err
		}
		id = generated
	}
	now := s.now().UTC()
	b := domain.Bookmark{
		ID:          id,
		OwnerID:     ownerID,
		Title:       in.Title,
		URL:         in.URL,
		Description: in.Description,
		Tags:        normalizeTags(in.Tags),
		ImageURLs:   nonNil(in.ImageURLs),
		VideoURL:    in.VideoURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.bookmarks.Create(b); err != nil {
		return domain.Bookmark{}, err
	}
	s.recordCreation(ownerID, b)
	return b, nil
}

// Update overwrites an existing bookmark's fields. It returns a
// domain.ValidationError for missing fields and domain.ErrNotFound when the
// bookmark does not exist for the owner.
func (s *BookmarkService) Update(id, ownerID string, in BookmarkInput) (domain.Bookmark, error) {
	if err := validateBookmark(in); err != nil {
		return domain.Bookmark{}, err
	}
	existing, err := s.bookmarks.GetOwned(id, ownerID)
	if err != nil {
		return domain.Bookmark{}, err
	}
	existing.Title = in.Title
	existing.URL = in.URL
	existing.Description = in.Description
	existing.Tags = normalizeTags(in.Tags)
	existing.ImageURLs = nonNil(in.ImageURLs)
	existing.VideoURL = in.VideoURL
	existing.UpdatedAt = s.now().UTC()
	if err := s.bookmarks.Update(existing); err != nil {
		return domain.Bookmark{}, err
	}
	return existing, nil
}

func (s *BookmarkService) Delete(id, ownerID string) error {
	return s.bookmarks.Delete(id, ownerID)
}

// recordCreation writes the activity-feed entry and notification that
// accompany a new bookmark. Failures are non-fatal to the create operation.
func (s *BookmarkService) recordCreation(ownerID string, b domain.Bookmark) {
	now := b.CreatedAt
	if actID, err := s.ids.NewID(); err == nil {
		_ = s.activities.Create(ownerID, domain.Activity{
			ID:          actID,
			Description: "Created a new bookmark '" + b.Title + "'",
			Type:        "bookmark_created",
			CreatedAt:   now,
		})
	}
	if notifID, err := s.ids.NewID(); err == nil {
		_ = s.notifications.Create(ownerID, domain.Notification{
			ID:        notifID,
			Title:     "New Bookmark",
			Body:      "You successfully created the bookmark '" + b.Title + "'.",
			Type:      "system",
			IsRead:    false,
			CreatedAt: now,
		})
	}
}

func validateBookmark(in BookmarkInput) error {
	if strings.TrimSpace(in.Title) == "" {
		return domain.ValidationError{Message: "title is required"}
	}
	if strings.TrimSpace(in.URL) == "" {
		return domain.ValidationError{Message: "url is required"}
	}
	return nil
}

// normalizeTags trims, de-duplicates, and drops empty tags, always returning a
// non-nil slice.
func normalizeTags(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// nonNil returns an empty slice instead of nil so JSON encodes [] rather than null.
func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
