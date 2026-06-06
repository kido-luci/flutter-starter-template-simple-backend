package httpapi

import (
	"time"

	"simple_backend_server/internal/domain"
	"simple_backend_server/internal/service"
)

// This file owns the JSON wire format. Domain entities stay free of transport
// concerns; these DTOs and their mappers preserve the exact field names and
// shapes the API exposes.

type userDTO struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func toUserDTO(u domain.User) userDTO {
	return userDTO{ID: u.ID, Username: u.Username}
}

type tokenPairDTO struct {
	User         userDTO `json:"user"`
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	ExpiresIn    int64   `json:"expires_in"`
}

func toTokenPairDTO(a domain.AuthResult) tokenPairDTO {
	return tokenPairDTO{
		User:         toUserDTO(a.User),
		AccessToken:  a.AccessToken,
		RefreshToken: a.RefreshToken,
		ExpiresIn:    a.ExpiresIn,
	}
}

type signInRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type bookmarkDTO struct {
	ID          string     `json:"id"`
	OwnerID     string     `json:"owner_id"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	Description string     `json:"description"`
	Tags        []string   `json:"tags"`
	ImageURLs   []string   `json:"image_urls"`
	VideoURL    string     `json:"video_url"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Rev         int        `json:"rev"`
	DeletedAt   *time.Time `json:"deleted_at"`
}

func toBookmarkDTO(b domain.Bookmark) bookmarkDTO {
	return bookmarkDTO{
		ID:          b.ID,
		OwnerID:     b.OwnerID,
		Title:       b.Title,
		URL:         b.URL,
		Description: b.Description,
		Tags:        b.Tags,
		ImageURLs:   b.ImageURLs,
		VideoURL:    b.VideoURL,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
		Rev:         b.Rev,
		DeletedAt:   b.DeletedAt,
	}
}

func toBookmarkDTOs(bs []domain.Bookmark) []bookmarkDTO {
	out := make([]bookmarkDTO, 0, len(bs))
	for _, b := range bs {
		out = append(out, toBookmarkDTO(b))
	}
	return out
}

// bookmarkRequest is the create/update payload. ID is honored only on create.
type bookmarkRequest struct {
	ID          string   `json:"id,omitempty"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	ImageURLs   []string `json:"image_urls"`
	VideoURL    string   `json:"video_url"`
}

func (req bookmarkRequest) toInput() service.BookmarkInput {
	return service.BookmarkInput{
		ID:          req.ID,
		Title:       req.Title,
		URL:         req.URL,
		Description: req.Description,
		Tags:        req.Tags,
		ImageURLs:   req.ImageURLs,
		VideoURL:    req.VideoURL,
	}
}

type collectionDTO struct {
	ID          string     `json:"id"`
	OwnerID     string     `json:"owner_id"`
	Name        string     `json:"name"`
	Icon        string     `json:"icon"`
	Color       int        `json:"color"`
	BookmarkIDs []string   `json:"bookmark_ids"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Rev         int        `json:"rev"`
	DeletedAt   *time.Time `json:"deleted_at"`
}

func toCollectionDTO(c domain.Collection) collectionDTO {
	return collectionDTO{
		ID:          c.ID,
		OwnerID:     c.OwnerID,
		Name:        c.Name,
		Icon:        c.Icon,
		Color:       c.Color,
		BookmarkIDs: c.BookmarkIDs,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
		Rev:         c.Rev,
		DeletedAt:   c.DeletedAt,
	}
}

func toCollectionDTOs(cs []domain.Collection) []collectionDTO {
	out := make([]collectionDTO, 0, len(cs))
	for _, c := range cs {
		out = append(out, toCollectionDTO(c))
	}
	return out
}

// collectionRequest is the create/update payload. ID is honored only on create.
type collectionRequest struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Icon        string   `json:"icon"`
	Color       int      `json:"color"`
	BookmarkIDs []string `json:"bookmark_ids"`
}

func (req collectionRequest) toInput() service.CollectionInput {
	return service.CollectionInput{
		ID:          req.ID,
		Name:        req.Name,
		Icon:        req.Icon,
		Color:       req.Color,
		BookmarkIDs: req.BookmarkIDs,
	}
}

type notificationDTO struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Type      string    `json:"type"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

func toNotificationDTOs(ns []domain.Notification) []notificationDTO {
	out := make([]notificationDTO, 0, len(ns))
	for _, n := range ns {
		out = append(out, notificationDTO{
			ID:        n.ID,
			Title:     n.Title,
			Body:      n.Body,
			Type:      n.Type,
			IsRead:    n.IsRead,
			CreatedAt: n.CreatedAt,
		})
	}
	return out
}

type activityDTO struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
}

func toActivityDTOs(as []domain.Activity) []activityDTO {
	out := make([]activityDTO, 0, len(as))
	for _, a := range as {
		out = append(out, activityDTO{
			ID:          a.ID,
			Description: a.Description,
			Type:        a.Type,
			CreatedAt:   a.CreatedAt,
		})
	}
	return out
}
