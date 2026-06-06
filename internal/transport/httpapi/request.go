package httpapi

import (
	"net/http"
	"strconv"
)

// expectedRevHeader is the optimistic-concurrency token a client echoes on an
// update/delete: the revision its edit was based on. Absent means "no
// expectation" and the write proceeds unconditionally; present-but-malformed is
// a client error (a malformed token must not silently bypass the check).
const expectedRevHeader = "X-Expected-Rev"

// sinceQuery is the delta-sync cursor: clients pass the highest revision they
// have already seen to fetch only newer rows (including tombstones).
const sinceQuery = "since"

// expectedRev parses the X-Expected-Rev header. ok is false only when the header
// is present but not a non-negative integer (a malformed token), which callers
// should reject rather than treat as "no expectation".
func expectedRev(r *http.Request) (rev int, ok bool) {
	raw := r.Header.Get(expectedRevHeader)
	if raw == "" {
		return 0, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

// sinceCursor parses the ?since delta cursor. present is true when one was
// supplied; ok is false only when it was supplied but is not a non-negative
// integer. A malformed cursor must be rejected, not silently downgraded to a
// full list (which would mask a client bug and resurrect deleted rows).
func sinceCursor(r *http.Request) (since int, present, ok bool) {
	raw := r.URL.Query().Get(sinceQuery)
	if raw == "" {
		return 0, false, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0, false, false
	}
	return v, true, true
}
