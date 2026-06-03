package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"simple_backend_server/internal/domain"
)

// JWTIssuer signs and parses short-lived HS256 access tokens. It satisfies
// service.TokenIssuer.
type JWTIssuer struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTIssuer(secret []byte, ttl time.Duration) *JWTIssuer {
	return &JWTIssuer{secret: secret, ttl: ttl}
}

type accessClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func (j *JWTIssuer) Sign(u domain.User) (string, error) {
	now := time.Now()
	claims := accessClaims{
		Username: u.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

func (j *JWTIssuer) Parse(raw string) (domain.User, error) {
	claims := &accessClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		// Pin the exact algorithm we sign with, rather than accepting any HMAC
		// variant (HS384/HS512), to avoid algorithm-confusion attacks.
		if t.Method == nil || t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return j.secret, nil
	})
	if err != nil {
		return domain.User{}, err
	}
	return domain.User{ID: claims.Subject, Username: claims.Username}, nil
}
