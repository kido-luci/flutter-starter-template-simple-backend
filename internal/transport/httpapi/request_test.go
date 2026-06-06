package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExpectedRev(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		wantRev int
		wantOK  bool
	}{
		{"absent is unconditional", "", 0, true},
		{"valid", "5", 5, true},
		{"zero", "0", 0, true},
		{"malformed is rejected", "abc", 0, false},
		{"negative is rejected", "-1", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/", nil)
			if tc.header != "" {
				r.Header.Set(expectedRevHeader, tc.header)
			}
			rev, ok := expectedRev(r)
			if rev != tc.wantRev || ok != tc.wantOK {
				t.Errorf("expectedRev = (%d, %v), want (%d, %v)", rev, ok, tc.wantRev, tc.wantOK)
			}
		})
	}
}

func TestSinceCursor(t *testing.T) {
	cases := []struct {
		name        string
		query       string
		wantSince   int
		wantPresent bool
		wantOK      bool
	}{
		{"absent requests full list", "", 0, false, true},
		{"valid", "since=7", 7, true, true},
		{"zero", "since=0", 0, true, true},
		{"malformed is rejected", "since=abc", 0, false, false},
		{"negative is rejected", "since=-1", 0, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/?"+tc.query, nil)
			since, present, ok := sinceCursor(r)
			if since != tc.wantSince || present != tc.wantPresent || ok != tc.wantOK {
				t.Errorf(
					"sinceCursor = (%d, %v, %v), want (%d, %v, %v)",
					since, present, ok, tc.wantSince, tc.wantPresent, tc.wantOK,
				)
			}
		})
	}
}
