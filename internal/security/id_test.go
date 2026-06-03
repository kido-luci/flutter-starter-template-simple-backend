package security_test

import (
	"encoding/hex"
	"testing"

	"simple_backend_server/internal/security"
)

func TestRandomID_FormatAndUniqueness(t *testing.T) {
	var gen security.RandomID

	const samples = 100
	seen := make(map[string]struct{}, samples)
	for i := 0; i < samples; i++ {
		id, err := gen.NewID()
		if err != nil {
			t.Fatalf("NewID returned error: %v", err)
		}
		if len(id) != 48 {
			t.Fatalf("id length = %d, want 48 hex chars", len(id))
		}
		if _, err := hex.DecodeString(id); err != nil {
			t.Fatalf("id %q is not valid hex: %v", id, err)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id generated: %q", id)
		}
		seen[id] = struct{}{}
	}
}
