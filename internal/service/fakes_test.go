package service_test

import (
	"errors"
	"fmt"
	"time"

	"simple_backend_server/internal/domain"
)

// This file holds in-memory fakes for the domain ports, shared by the service
// tests. They favor simple, observable behavior over mocking frameworks.

// --- IDGenerator ---

type fakeIDGen struct {
	n   int
	err error
}

func (g *fakeIDGen) NewID() (string, error) {
	if g.err != nil {
		return "", g.err
	}
	g.n++
	return fmt.Sprintf("id-%d", g.n), nil
}

// --- PasswordHasher ---

// fakeHasher hashes by prefixing, so a hash and password "match" when the hash
// is exactly "hashed:" + password.
type fakeHasher struct{ hashErr error }

func (h fakeHasher) Hash(password string) (string, error) {
	if h.hashErr != nil {
		return "", h.hashErr
	}
	return "hashed:" + password, nil
}

func (h fakeHasher) Compare(hash, password string) error {
	if hash == "hashed:"+password {
		return nil
	}
	return errors.New("password mismatch")
}

// --- TokenIssuer ---

type fakeTokenIssuer struct{ signErr error }

func (t fakeTokenIssuer) Sign(u domain.User) (string, error) {
	if t.signErr != nil {
		return "", t.signErr
	}
	return "token:" + u.ID, nil
}

func (t fakeTokenIssuer) Parse(raw string) (domain.User, error) {
	return domain.User{}, errors.New("not implemented")
}

// --- UserRepository ---

type storedUser struct {
	user domain.User
	hash string
}

type fakeUserRepo struct {
	byName    map[string]storedUser
	byID      map[string]storedUser
	createErr error
}

var _ domain.UserRepository = (*fakeUserRepo)(nil)

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byName: map[string]storedUser{}, byID: map[string]storedUser{}}
}

func (r *fakeUserRepo) Create(u domain.User, hash string) error {
	if r.createErr != nil {
		return r.createErr
	}
	if _, ok := r.byName[u.Username]; ok {
		return domain.ErrConflict
	}
	su := storedUser{user: u, hash: hash}
	r.byName[u.Username] = su
	r.byID[u.ID] = su
	return nil
}

func (r *fakeUserRepo) FindByUsername(username string) (domain.User, string, error) {
	su, ok := r.byName[username]
	if !ok {
		return domain.User{}, "", domain.ErrNotFound
	}
	return su.user, su.hash, nil
}

func (r *fakeUserRepo) PasswordHash(userID string) (string, error) {
	su, ok := r.byID[userID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return su.hash, nil
}

func (r *fakeUserRepo) UpdatePassword(userID, hash string) error {
	su, ok := r.byID[userID]
	if !ok {
		return domain.ErrNotFound
	}
	su.hash = hash
	r.byID[userID] = su
	r.byName[su.user.Username] = su
	return nil
}

// --- RefreshTokenRepository ---

type refreshRecord struct {
	userID    string
	expiresAt time.Time
}

type fakeRefreshRepo struct {
	tokens    map[string]refreshRecord
	usernames map[string]string // userID -> username, for Rotate's result
	issued    []string
	revoked   []string
}

var _ domain.RefreshTokenRepository = (*fakeRefreshRepo)(nil)

func newFakeRefreshRepo() *fakeRefreshRepo {
	return &fakeRefreshRepo{tokens: map[string]refreshRecord{}, usernames: map[string]string{}}
}

func (r *fakeRefreshRepo) Issue(token, userID string, expiresAt time.Time) error {
	r.tokens[token] = refreshRecord{userID: userID, expiresAt: expiresAt}
	r.issued = append(r.issued, token)
	return nil
}

func (r *fakeRefreshRepo) Rotate(oldToken, newToken string, expiresAt time.Time) (domain.User, error) {
	rec, ok := r.tokens[oldToken]
	if !ok {
		return domain.User{}, domain.RefreshError{Message: "refresh token is not recognized"}
	}
	delete(r.tokens, oldToken)
	if time.Now().After(rec.expiresAt) {
		return domain.User{}, domain.RefreshError{Message: "refresh token is expired"}
	}
	r.tokens[newToken] = refreshRecord{userID: rec.userID, expiresAt: expiresAt}
	return domain.User{ID: rec.userID, Username: r.usernames[rec.userID]}, nil
}

func (r *fakeRefreshRepo) Revoke(token string) {
	delete(r.tokens, token)
	r.revoked = append(r.revoked, token)
}

// --- BookmarkRepository ---

type fakeBookmarkRepo struct {
	items     map[string]domain.Bookmark // keyed by id
	createErr error
	updateErr error
}

var _ domain.BookmarkRepository = (*fakeBookmarkRepo)(nil)

func newFakeBookmarkRepo() *fakeBookmarkRepo {
	return &fakeBookmarkRepo{items: map[string]domain.Bookmark{}}
}

func (r *fakeBookmarkRepo) ListByOwner(ownerID string) ([]domain.Bookmark, error) {
	out := make([]domain.Bookmark, 0)
	for _, b := range r.items {
		if b.OwnerID == ownerID {
			out = append(out, b)
		}
	}
	return out, nil
}

func (r *fakeBookmarkRepo) GetOwned(id, ownerID string) (domain.Bookmark, error) {
	b, ok := r.items[id]
	if !ok || b.OwnerID != ownerID {
		return domain.Bookmark{}, domain.ErrNotFound
	}
	return b, nil
}

func (r *fakeBookmarkRepo) Create(b domain.Bookmark) error {
	if r.createErr != nil {
		return r.createErr
	}
	if _, ok := r.items[b.ID]; ok {
		return domain.ErrConflict
	}
	r.items[b.ID] = b
	return nil
}

func (r *fakeBookmarkRepo) Update(b domain.Bookmark) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.items[b.ID] = b
	return nil
}

func (r *fakeBookmarkRepo) Delete(id, ownerID string) error {
	b, ok := r.items[id]
	if !ok || b.OwnerID != ownerID {
		return domain.ErrNotFound
	}
	delete(r.items, id)
	return nil
}

// --- CollectionRepository ---

type fakeCollectionRepo struct {
	items     map[string]domain.Collection // keyed by id
	createErr error
	updateErr error
}

var _ domain.CollectionRepository = (*fakeCollectionRepo)(nil)

func newFakeCollectionRepo() *fakeCollectionRepo {
	return &fakeCollectionRepo{items: map[string]domain.Collection{}}
}

func (r *fakeCollectionRepo) ListByOwner(ownerID string) ([]domain.Collection, error) {
	out := make([]domain.Collection, 0)
	for _, c := range r.items {
		if c.OwnerID == ownerID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *fakeCollectionRepo) GetOwned(id, ownerID string) (domain.Collection, error) {
	c, ok := r.items[id]
	if !ok || c.OwnerID != ownerID {
		return domain.Collection{}, domain.ErrNotFound
	}
	return c, nil
}

func (r *fakeCollectionRepo) Create(c domain.Collection) error {
	if r.createErr != nil {
		return r.createErr
	}
	if _, ok := r.items[c.ID]; ok {
		return domain.ErrConflict
	}
	r.items[c.ID] = c
	return nil
}

func (r *fakeCollectionRepo) Update(c domain.Collection) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.items[c.ID] = c
	return nil
}

func (r *fakeCollectionRepo) Delete(id, ownerID string) error {
	c, ok := r.items[id]
	if !ok || c.OwnerID != ownerID {
		return domain.ErrNotFound
	}
	delete(r.items, id)
	return nil
}

// --- ActivityRepository ---

type ownedActivity struct {
	ownerID  string
	activity domain.Activity
}

type fakeActivityRepo struct {
	created   []ownedActivity
	createErr error
}

var _ domain.ActivityRepository = (*fakeActivityRepo)(nil)

func (r *fakeActivityRepo) ListByOwner(ownerID string) ([]domain.Activity, error) {
	out := make([]domain.Activity, 0)
	for _, a := range r.created {
		if a.ownerID == ownerID {
			out = append(out, a.activity)
		}
	}
	return out, nil
}

func (r *fakeActivityRepo) Create(ownerID string, a domain.Activity) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, ownedActivity{ownerID: ownerID, activity: a})
	return nil
}

// --- NotificationRepository ---

type ownedNotification struct {
	ownerID      string
	notification domain.Notification
}

type readArgs struct {
	id, ownerID string
}

type fakeNotificationRepo struct {
	created   []ownedNotification
	reads     []readArgs
	createErr error
	readErr   error
}

var _ domain.NotificationRepository = (*fakeNotificationRepo)(nil)

func (r *fakeNotificationRepo) ListByOwner(ownerID string) ([]domain.Notification, error) {
	out := make([]domain.Notification, 0)
	for _, n := range r.created {
		if n.ownerID == ownerID {
			out = append(out, n.notification)
		}
	}
	return out, nil
}

func (r *fakeNotificationRepo) MarkRead(id, ownerID string) error {
	r.reads = append(r.reads, readArgs{id: id, ownerID: ownerID})
	return r.readErr
}

func (r *fakeNotificationRepo) Create(ownerID string, n domain.Notification) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, ownedNotification{ownerID: ownerID, notification: n})
	return nil
}
