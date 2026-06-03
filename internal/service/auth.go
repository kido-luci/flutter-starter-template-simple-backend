package service

import (
	"errors"
	"time"

	"simple_backend_server/internal/domain"
)

// AuthService implements the authentication use cases: registration, sign-in,
// password change, and refresh-token rotation.
type AuthService struct {
	users         domain.UserRepository
	refreshTokens domain.RefreshTokenRepository
	hasher        PasswordHasher
	tokens        TokenIssuer
	ids           IDGenerator
	accessTTL     time.Duration
	refreshTTL    time.Duration
	now           func() time.Time
}

func NewAuthService(
	users domain.UserRepository,
	refreshTokens domain.RefreshTokenRepository,
	hasher PasswordHasher,
	tokens TokenIssuer,
	ids IDGenerator,
	accessTTL, refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		users:         users,
		refreshTokens: refreshTokens,
		hasher:        hasher,
		tokens:        tokens,
		ids:           ids,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
		now:           time.Now,
	}
}

// Register creates a new account and issues an initial token pair. It returns
// domain.ErrConflict if the username is already taken.
func (s *AuthService) Register(username, password string) (domain.AuthResult, error) {
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return domain.AuthResult{}, err
	}
	id, err := s.ids.NewID()
	if err != nil {
		return domain.AuthResult{}, err
	}
	u := domain.User{ID: id, Username: username}
	if err := s.users.Create(u, hash); err != nil {
		return domain.AuthResult{}, err
	}
	return s.issueTokens(u)
}

// SignIn verifies credentials and issues a token pair. It returns
// domain.ErrInvalidCredentials when the username or password is wrong.
func (s *AuthService) SignIn(username, password string) (domain.AuthResult, error) {
	u, hash, err := s.users.FindByUsername(username)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.AuthResult{}, domain.ErrInvalidCredentials
		}
		return domain.AuthResult{}, err
	}
	if err := s.hasher.Compare(hash, password); err != nil {
		return domain.AuthResult{}, domain.ErrInvalidCredentials
	}
	return s.issueTokens(u)
}

// ChangePassword replaces a user's password after verifying the current one. It
// returns domain.ErrInvalidCredentials when currentPassword is wrong.
func (s *AuthService) ChangePassword(userID, currentPassword, newPassword string) error {
	hash, err := s.users.PasswordHash(userID)
	if err != nil {
		return err
	}
	if err := s.hasher.Compare(hash, currentPassword); err != nil {
		return domain.ErrInvalidCredentials
	}
	newHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	return s.users.UpdatePassword(userID, newHash)
}

// Refresh rotates a refresh token and issues a new access token. Rotation
// failures (unknown or expired token) are reported as domain.RefreshError.
func (s *AuthService) Refresh(refreshToken string) (domain.AuthResult, error) {
	newRefresh, err := s.ids.NewID()
	if err != nil {
		return domain.AuthResult{}, err
	}
	u, err := s.refreshTokens.Rotate(refreshToken, newRefresh, s.now().Add(s.refreshTTL))
	if err != nil {
		// Invalid/expired tokens are RefreshError (-> 401); any other failure
		// (e.g. a DB outage) propagates unchanged so transport maps it to 500.
		var re domain.RefreshError
		if errors.As(err, &re) {
			return domain.AuthResult{}, re
		}
		return domain.AuthResult{}, err
	}
	access, err := s.tokens.Sign(u)
	if err != nil {
		return domain.AuthResult{}, err
	}
	return domain.AuthResult{
		User:         u,
		AccessToken:  access,
		RefreshToken: newRefresh,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}, nil
}

// SignOut revokes a refresh token. An empty token is ignored.
func (s *AuthService) SignOut(refreshToken string) {
	if refreshToken != "" {
		s.refreshTokens.Revoke(refreshToken)
	}
}

func (s *AuthService) issueTokens(u domain.User) (domain.AuthResult, error) {
	access, err := s.tokens.Sign(u)
	if err != nil {
		return domain.AuthResult{}, err
	}
	refresh, err := s.ids.NewID()
	if err != nil {
		return domain.AuthResult{}, err
	}
	if err := s.refreshTokens.Issue(refresh, u.ID, s.now().Add(s.refreshTTL)); err != nil {
		return domain.AuthResult{}, err
	}
	return domain.AuthResult{
		User:         u,
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}, nil
}
