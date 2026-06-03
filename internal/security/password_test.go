package security_test

import (
	"testing"

	"simple_backend_server/internal/security"
)

func TestBcryptHasher_HashAndCompare(t *testing.T) {
	var h security.BcryptHasher

	hash, err := h.Hash("correct horse")
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}
	if hash == "correct horse" {
		t.Error("hash must not equal the plaintext password")
	}

	if err := h.Compare(hash, "correct horse"); err != nil {
		t.Errorf("Compare with the right password failed: %v", err)
	}
	if err := h.Compare(hash, "battery staple"); err == nil {
		t.Error("Compare with the wrong password should fail")
	}
}

func TestBcryptHasher_HashIsSalted(t *testing.T) {
	var h security.BcryptHasher

	first, err := h.Hash("same")
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}
	second, err := h.Hash("same")
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}
	if first == second {
		t.Error("bcrypt should produce distinct hashes for the same password (salting)")
	}
}
