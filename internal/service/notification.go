package service

import "simple_backend_server/internal/domain"

// NotificationService implements the notification and activity-feed read use
// cases.
type NotificationService struct {
	notifications domain.NotificationRepository
	activities    domain.ActivityRepository
}

func NewNotificationService(
	notifications domain.NotificationRepository,
	activities domain.ActivityRepository,
) *NotificationService {
	return &NotificationService{notifications: notifications, activities: activities}
}

func (s *NotificationService) List(ownerID string) ([]domain.Notification, error) {
	return s.notifications.ListByOwner(ownerID)
}

func (s *NotificationService) MarkRead(id, ownerID string) error {
	return s.notifications.MarkRead(id, ownerID)
}

func (s *NotificationService) ListActivity(ownerID string) ([]domain.Activity, error) {
	return s.activities.ListByOwner(ownerID)
}
