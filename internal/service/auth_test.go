package service_test

import (
	"errors"
	"testing"
	"time"

	"simple_backend_server/internal/domain"
	"simple_backend_server/internal/service"
)

const (
	testAccessTTL  = 15 * time.Minute
	testRefreshTTL = time.Hour
)

type authFixture struct {
	svc     *service.AuthService
	users   *fakeUserRepo
	refresh *fakeRefreshRepo
	ids     *fakeIDGen
	tokens  fakeTokenIssuer
	hasher  fakeHasher
}

func newAuthFixture() *authFixture {
	f := &authFixture{
		users:   newFakeUserRepo(),
		refresh: newFakeRefreshRepo(),
		ids:     &fakeIDGen{},
	}
	f.svc = service.NewAuthService(
		f.users, f.refresh, f.hasher, f.tokens, f.ids, testAccessTTL, testRefreshTTL,
	)
	return f
}

func TestAuthService_Register_Success(t *testing.T) {
	f := newAuthFixture()

	res, err := f.svc.Register("alice", "pw")
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if res.User.Username != "alice" {
		t.Errorf("username = %q, want alice", res.User.Username)
	}
	if res.User.ID != "id-1" {
		t.Errorf("user id = %q, want id-1", res.User.ID)
	}
	if res.AccessToken != "token:id-1" {
		t.Errorf("access token = %q, want token:id-1", res.AccessToken)
	}
	if res.RefreshToken != "id-2" {
		t.Errorf("refresh token = %q, want id-2", res.RefreshToken)
	}
	if want := int64(testAccessTTL.Seconds()); res.ExpiresIn != want {
		t.Errorf("expires_in = %d, want %d", res.ExpiresIn, want)
	}

	if got := f.users.byID["id-1"].hash; got != "hashed:pw" {
		t.Errorf("stored hash = %q, want hashed:pw", got)
	}
	if len(f.refresh.issued) != 1 || f.refresh.issued[0] != "id-2" {
		t.Errorf("issued refresh tokens = %v, want [id-2]", f.refresh.issued)
	}
}

func TestAuthService_Register_DuplicateUsername(t *testing.T) {
	f := newAuthFixture()
	if _, err := f.svc.Register("alice", "pw"); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	_, err := f.svc.Register("alice", "other")
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("err = %v, want domain.ErrConflict", err)
	}
}

func TestAuthService_SignIn_Success(t *testing.T) {
	f := newAuthFixture()
	if _, err := f.svc.Register("alice", "pw"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	res, err := f.svc.SignIn("alice", "pw")
	if err != nil {
		t.Fatalf("SignIn returned error: %v", err)
	}
	if res.User.Username != "alice" {
		t.Errorf("username = %q, want alice", res.User.Username)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Errorf("expected non-empty tokens, got access=%q refresh=%q", res.AccessToken, res.RefreshToken)
	}
}

func TestAuthService_SignIn_UnknownUser(t *testing.T) {
	f := newAuthFixture()

	_, err := f.svc.SignIn("ghost", "pw")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestAuthService_SignIn_WrongPassword(t *testing.T) {
	f := newAuthFixture()
	if _, err := f.svc.Register("alice", "pw"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	_, err := f.svc.SignIn("alice", "nope")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want domain.ErrInvalidCredentials", err)
	}
}

func TestAuthService_ChangePassword_Success(t *testing.T) {
	f := newAuthFixture()
	if _, err := f.svc.Register("alice", "pw"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := f.svc.ChangePassword("id-1", "pw", "newpw"); err != nil {
		t.Fatalf("ChangePassword returned error: %v", err)
	}
	if got := f.users.byID["id-1"].hash; got != "hashed:newpw" {
		t.Errorf("stored hash = %q, want hashed:newpw", got)
	}
}

func TestAuthService_ChangePassword_WrongCurrent(t *testing.T) {
	f := newAuthFixture()
	if _, err := f.svc.Register("alice", "pw"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := f.svc.ChangePassword("id-1", "wrong", "newpw")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want domain.ErrInvalidCredentials", err)
	}
	if got := f.users.byID["id-1"].hash; got != "hashed:pw" {
		t.Errorf("hash changed to %q despite failed verification", got)
	}
}

func TestAuthService_Refresh_RotatesToken(t *testing.T) {
	f := newAuthFixture()
	f.refresh.usernames["u1"] = "alice"
	if err := f.refresh.Issue("old-token", "u1", time.Now().Add(testRefreshTTL)); err != nil {
		t.Fatalf("seed Issue failed: %v", err)
	}

	res, err := f.svc.Refresh("old-token")
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if res.User.Username != "alice" {
		t.Errorf("username = %q, want alice", res.User.Username)
	}
	if res.RefreshToken == "old-token" || res.RefreshToken == "" {
		t.Errorf("expected a rotated refresh token, got %q", res.RefreshToken)
	}
	if _, stillPresent := f.refresh.tokens["old-token"]; stillPresent {
		t.Error("old refresh token should be consumed after rotation")
	}
	if _, ok := f.refresh.tokens[res.RefreshToken]; !ok {
		t.Error("new refresh token should be persisted")
	}
}

func TestAuthService_Refresh_UnknownToken(t *testing.T) {
	f := newAuthFixture()

	_, err := f.svc.Refresh("does-not-exist")
	var re domain.RefreshError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want domain.RefreshError", err)
	}
	if re.Error() != "refresh token is not recognized" {
		t.Errorf("message = %q", re.Error())
	}
}

func TestAuthService_Refresh_ExpiredToken(t *testing.T) {
	f := newAuthFixture()
	f.refresh.usernames["u1"] = "alice"
	if err := f.refresh.Issue("old-token", "u1", time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("seed Issue failed: %v", err)
	}

	_, err := f.svc.Refresh("old-token")
	var re domain.RefreshError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want domain.RefreshError", err)
	}
	if re.Error() != "refresh token is expired" {
		t.Errorf("message = %q, want 'refresh token is expired'", re.Error())
	}
}

func TestAuthService_SignOut(t *testing.T) {
	f := newAuthFixture()
	if err := f.refresh.Issue("tok", "u1", time.Now().Add(testRefreshTTL)); err != nil {
		t.Fatalf("seed Issue failed: %v", err)
	}

	f.svc.SignOut("tok")
	if len(f.refresh.revoked) != 1 || f.refresh.revoked[0] != "tok" {
		t.Errorf("revoked = %v, want [tok]", f.refresh.revoked)
	}

	f.svc.SignOut("")
	if len(f.refresh.revoked) != 1 {
		t.Errorf("empty token should not trigger a revoke, revoked = %v", f.refresh.revoked)
	}
}
