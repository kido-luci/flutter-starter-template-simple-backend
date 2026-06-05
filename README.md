# Simple Backend Server

A lightweight REST API written in Go that backs the Flutter Starter Template
during local development. It provides real authentication, bookmark CRUD, file
uploads, and a notification/activity feed without requiring any external
services.

> [!WARNING]
> This server is for **development and testing only**. It ships with an
> insecure default JWT secret, permissive CORS (`*`), and a local SQLite file
> for storage. Do not deploy it as-is to production.

## Storage

Data is persisted to a local **SQLite** database file (`data.db`) via the pure-Go
[`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) driver — no CGO and
no external database required. The schema (users, refresh tokens, bookmarks,
collections, activities, notifications) is created automatically on startup, and data
survives restarts. Delete `data.db` to reset all state. The file and the runtime
`uploads/` directory are git-ignored.

## Prerequisites

- [Go](https://go.dev/doc/install) 1.25 or later (the version is pinned by the
  `go` directive in `go.mod`).

## Running the server

```bash
go run .
```

Or build and run the executable:

```bash
go build -o simple_backend_server .
./simple_backend_server
```

By default the server listens on port `8080`.

## Configuration

Configured via environment variables:

- `ADDR`: address/port to listen on. Defaults to `:8080`.
- `JWT_SECRET`: secret used to sign access tokens. If unset, it falls back to an
  insecure development default and logs a warning. **Always set this outside
  local development.**

## Authentication

Authentication is real, not stubbed:

- Passwords are hashed with **bcrypt** and stored in the database.
- Sign-in returns a short-lived **JWT access token** (HS256, 15-minute TTL) and
  an opaque **refresh token** (30-day TTL).
- Refresh tokens are **single-use**: each refresh rotates the token and
  invalidates the previous one.

Send the access token as `Authorization: Bearer <access_token>` on protected
endpoints. Error responses use the shape `{"code": "...", "message": "..."}`.

## API endpoints

The server uses [`chi`](https://github.com/go-chi/chi) for routing.

### Health

- `GET /health` — returns `{"status": "ok"}`.

### Authentication (`/api/auth`)

- `POST /api/auth/register`
  - Body: `{"username": "...", "password": "..."}`
  - Creates an account and returns a token pair. `409` if the username exists.
- `POST /api/auth/sign-in`
  - Body: `{"username": "...", "password": "..."}`
  - Returns a token pair on valid credentials; `401` otherwise.
- `POST /api/auth/refresh`
  - Body: `{"refresh_token": "..."}`
  - Returns a new access token and a rotated refresh token.
- `POST /api/auth/sign-out`
  - Body: `{"refresh_token": "..."}`
  - Revokes the provided refresh token. Always returns `204`.
- `GET /api/auth/me` _(auth required)_
  - Returns the authenticated user: `{"id": "...", "username": "..."}`.
- `POST /api/auth/change-password` _(auth required)_
  - Body: `{"currentPassword": "...", "newPassword": "..."}`
  - Returns `204` on success; `401` if the current password is wrong.

A successful auth response (register / sign-in / refresh) looks like:

```json
{
  "user": { "id": "...", "username": "alice" },
  "access_token": "<jwt>",
  "refresh_token": "<opaque>",
  "expires_in": 900
}
```

### Bookmarks (`/api/bookmarks`) _(auth required)_

CRUD scoped to the authenticated user.

- `GET /api/bookmarks` — list the current user's bookmarks.
- `POST /api/bookmarks` — create a bookmark; returns `201`.
  - Body: `{"id": "optional", "title": "...", "url": "...", "description": "...", "tags": ["..."], "image_urls": ["..."], "video_url": "..."}`
  - `title` and `url` are required. If `id` is omitted the server generates one;
    supplying a stable `id` lets offline-first clients mint IDs locally (a
    duplicate `id` returns `409`).
- `GET /api/bookmarks/{id}` — fetch one bookmark.
- `PUT /api/bookmarks/{id}` — replace a bookmark (same body as `POST`).
- `DELETE /api/bookmarks/{id}` — delete a bookmark; returns `204`.

Creating a bookmark also records an entry in the activity feed and a
notification for the owner.

### Collections (`/api/collections`) _(auth required)_

CRUD scoped to the authenticated user. A collection is a named folder whose
membership is a list of bookmark ids.

- `GET /api/collections` — list the current user's collections.
- `POST /api/collections` — create a collection; returns `201`.
  - Body: `{"id": "optional", "name": "...", "icon": "...", "color": 4282449393, "bookmark_ids": ["..."]}`
  - `name` is required. `color` is an ARGB int. If `id` is omitted the server
    generates one; supplying a stable `id` lets offline-first clients mint IDs
    locally (a duplicate `id` returns `409`).
- `GET /api/collections/{id}` — fetch one collection.
- `PUT /api/collections/{id}` — replace a collection (same body as `POST`).
- `DELETE /api/collections/{id}` — delete a collection; returns `204`.

Creating a collection also records an entry in the activity feed and a
notification for the owner.

### Uploads

- `POST /api/upload` _(auth required)_
  - `multipart/form-data` with a `file` field (max 10 MB).
  - Returns `{"url": "<public-url>"}`. Uploaded files are served statically from
    `GET /uploads/*`.

### Notifications & activity _(auth required)_

- `GET /api/notifications` — list the user's notifications (newest first).
- `PATCH /api/notifications/{id}/read` — mark a notification read.
- `GET /api/activity` — list the user's activity-feed entries (newest first).

## Architecture

The code follows a clean, layered architecture with dependencies pointing
inward (`transport → service → domain`); infrastructure implements the domain
and service ports and is wired only in `main.go`.

```
main.go                      composition root (wiring); `go run .` entrypoint
internal/
  domain/                    entities, repository ports, sentinel errors (stdlib only)
  service/                   use cases (auth, bookmark, notification) + ports
  storage/sqlite/            SQLite repository implementations + schema migration
  security/                  JWT, bcrypt, and random-id adapters
  transport/httpapi/         chi router, DTOs, HTTP handlers, middleware
```

The HTTP layer owns the JSON wire format (DTOs); domain entities carry no
transport or storage concerns.

## Development

```bash
go test ./...            # run the test suite
go test -race ./...      # with the race detector
go vet ./...             # static checks
gofmt -l .               # list files needing formatting
```

Pushes and pull requests are checked by GitHub Actions (`.github/workflows/ci.yml`):
formatting, `go vet`, build, and the race-enabled test suite.
