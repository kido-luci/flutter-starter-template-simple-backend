package service_test

import (
	"testing"

	"simple_backend_server/internal/domain"
	"simple_backend_server/internal/service"
)

func TestNotificationService_List(t *testing.T) {
	notifs := &fakeNotificationRepo{}
	_ = notifs.Create("owner-1", domain.Notification{ID: "n1", Title: "Hi"})
	_ = notifs.Create("owner-2", domain.Notification{ID: "n2", Title: "Other"})
	svc := service.NewNotificationService(notifs, &fakeActivityRepo{})

	got, err := svc.List("owner-1")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "n1" {
		t.Errorf("List(owner-1) = %v, want only n1", got)
	}
}

func TestNotificationService_MarkRead(t *testing.T) {
	notifs := &fakeNotificationRepo{}
	svc := service.NewNotificationService(notifs, &fakeActivityRepo{})

	if err := svc.MarkRead("n1", "owner-1"); err != nil {
		t.Fatalf("MarkRead returned error: %v", err)
	}
	if len(notifs.reads) != 1 || notifs.reads[0] != (readArgs{id: "n1", ownerID: "owner-1"}) {
		t.Errorf("reads = %v, want one {n1, owner-1}", notifs.reads)
	}
}

func TestNotificationService_ListActivity(t *testing.T) {
	activities := &fakeActivityRepo{}
	_ = activities.Create("owner-1", domain.Activity{ID: "a1", Type: "bookmark_created"})
	_ = activities.Create("owner-2", domain.Activity{ID: "a2"})
	svc := service.NewNotificationService(&fakeNotificationRepo{}, activities)

	got, err := svc.ListActivity("owner-1")
	if err != nil {
		t.Fatalf("ListActivity returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "a1" {
		t.Errorf("ListActivity(owner-1) = %v, want only a1", got)
	}
}
