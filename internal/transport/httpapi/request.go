package httpapi

import (
	"net/http"
	"strconv"
)

// expectedRevHeader is the optimistic-concurrency token a client echoes on an
// update/delete: the revision its edit was based on. Absent or unparseable
// means "no expectation" (0), and the write proceeds unconditionally.
const expectedRevHeader = "X-Expected-Rev"

// sinceQuery is the delta-sync cursor: clients pass the highest revision they
// have already seen to fetch only newer rows (including tombstones).
const sinceQuery = "since"

func expectedRev(r *http.Request) int {
	rev, err := strconv.Atoi(r.Header.Get(expectedRevHeader))
	if err != nil {
		return 0
	}
	return rev
}

// sinceCursor reports the delta cursor and whether one was supplied. A missing
// or invalid value means a full (non-delta) list is requested.
func sinceCursor(r *http.Request) (int, bool) {
	raw := r.URL.Query().Get(sinceQuery)
	if raw == "" {
		return 0, false
	}
	since, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return since, true
}
