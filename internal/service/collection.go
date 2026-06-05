package service

import (
	"strings"
	"time"

	"simple_backend_server/internal/domain"
)

// CollectionInput carries the mutable fields of a collection from the transport
// layer. ID is honored only on Create (offline-first clients may mint ids).
type CollectionInput struct {
	ID          string
	Name        string
	Icon        string
	Color       int
	BookmarkIDs []string
}

// CollectionService implements the collection use cases. Creating a collection
// also records an activity-feed entry and a notification for the owner.
type CollectionService struct {
	collections   domain.CollectionRepository
	activities    domain.ActivityRepository
	notifications domain.NotificationRepository
	ids           IDGenerator
	now           func() time.Time
}

func NewCollectionService(
	collections domain.CollectionRepository,
	activities domain.ActivityRepository,
	notifications domain.NotificationRepository,
	ids IDGenerator,
) *CollectionService {
	return &CollectionService{
		collections:   collections,
		activities:    activities,
		notifications: notifications,
		ids:           ids,
		now:           time.Now,
	}
}

func (s *CollectionService) List(ownerID string) ([]domain.Collection, error) {
	return s.collections.ListByOwner(ownerID)
}

func (s *CollectionService) Get(id, ownerID string) (domain.Collection, error) {
	return s.collections.GetOwned(id, ownerID)
}

// Create stores a new collection for the owner and records the side-effect
// activity and notification (best-effort). It returns a domain.ValidationError
// for missing fields and domain.ErrConflict if a client-provided id is taken.
func (s *CollectionService) Create(ownerID string, in CollectionInput) (domain.Collection, error) {
	if err := validateCollection(in); err != nil {
		return domain.Collection{}, err
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		generated, err := s.ids.NewID()
		if err != nil {
			return domain.Collection{}, err
		}
		id = generated
	}
	now := s.now().UTC()
	c := domain.Collection{
		ID:      id,
		OwnerID: ownerID,
		Name:    strings.TrimSpace(in.Name),
		Icon:    in.Icon,
		Color:   in.Color,
		// normalizeTags trims, de-duplicates, and drops empties, which is
		// exactly the normalization we want for bookmark ids.
		BookmarkIDs: normalizeTags(in.BookmarkIDs),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.collections.Create(c); err != nil {
		return domain.Collection{}, err
	}
	s.recordCreation(ownerID, c)
	return c, nil
}

// Update overwrites an existing collection's fields. It returns a
// domain.ValidationError for missing fields and domain.ErrNotFound when the
// collection does not exist for the owner.
func (s *CollectionService) Update(id, ownerID string, in CollectionInput) (domain.Collection, error) {
	if err := validateCollection(in); err != nil {
		return domain.Collection{}, err
	}
	existing, err := s.collections.GetOwned(id, ownerID)
	if err != nil {
		return domain.Collection{}, err
	}
	existing.Name = strings.TrimSpace(in.Name)
	existing.Icon = in.Icon
	existing.Color = in.Color
	existing.BookmarkIDs = normalizeTags(in.BookmarkIDs)
	existing.UpdatedAt = s.now().UTC()
	if err := s.collections.Update(existing); err != nil {
		return domain.Collection{}, err
	}
	return existing, nil
}

func (s *CollectionService) Delete(id, ownerID string) error {
	return s.collections.Delete(id, ownerID)
}

// recordCreation writes the activity-feed entry and notification that
// accompany a new collection. Failures are non-fatal to the create operation.
func (s *CollectionService) recordCreation(ownerID string, c domain.Collection) {
	now := c.CreatedAt
	if actID, err := s.ids.NewID(); err == nil {
		_ = s.activities.Create(ownerID, domain.Activity{
			ID:          actID,
			Description: "Created a new collection '" + c.Name + "'",
			Type:        "collection_created",
			CreatedAt:   now,
		})
	}
	if notifID, err := s.ids.NewID(); err == nil {
		_ = s.notifications.Create(ownerID, domain.Notification{
			ID:        notifID,
			Title:     "New Collection",
			Body:      "You successfully created the collection '" + c.Name + "'.",
			Type:      "system",
			IsRead:    false,
			CreatedAt: now,
		})
	}
}

func validateCollection(in CollectionInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return domain.ValidationError{Message: "name is required"}
	}
	return nil
}
