package security_test

import (
	"testing"
	"time"

	"simple_backend_server/internal/domain"
	"simple_backend_server/internal/security"
)

func TestJWTIssuer_SignParseRoundTrip(t *testing.T) {
	issuer := security.NewJWTIssuer([]byte("secret"), time.Minute)
	want := domain.User{ID: "u1", Username: "alice"}

	token, err := issuer.Sign(want)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	got, err := issuer.Parse(token)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got != want {
		t.Errorf("round-trip user = %+v, want %+v", got, want)
	}
}

func TestJWTIssuer_ParseRejectsWrongSecret(t *testing.T) {
	signer := security.NewJWTIssuer([]byte("secret-a"), time.Minute)
	verifier := security.NewJWTIssuer([]byte("secret-b"), time.Minute)

	token, err := signer.Sign(domain.User{ID: "u1", Username: "alice"})
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	if _, err := verifier.Parse(token); err == nil {
		t.Error("Parse should reject a token signed with a different secret")
	}
}

func TestJWTIssuer_ParseRejectsExpiredToken(t *testing.T) {
	issuer := security.NewJWTIssuer([]byte("secret"), -time.Minute) // already expired

	token, err := issuer.Sign(domain.User{ID: "u1", Username: "alice"})
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	if _, err := issuer.Parse(token); err == nil {
		t.Error("Parse should reject an expired token")
	}
}

func TestJWTIssuer_ParseRejectsGarbage(t *testing.T) {
	issuer := security.NewJWTIssuer([]byte("secret"), time.Minute)

	if _, err := issuer.Parse("not-a-jwt"); err == nil {
		t.Error("Parse should reject a malformed token")
	}
}
