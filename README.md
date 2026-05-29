# Simple Backend Server

This directory contains a lightweight, in-memory backend server written in Go. It is designed to provide a local, fully functional REST API for developing and testing the Flutter Starter Template without requiring a complex backend setup or database.

> [!WARNING]
> This server stores all data (users, tokens, and bookmarks) **in-memory**. All data is lost when the server restarts. It is strictly for development and testing purposes and should **never** be used in production.

## Prerequisites

- [Go](https://go.dev/doc/install) 1.21 or later.

## Running the Server

To start the server, simply run:

```bash
go run .
```

Alternatively, you can build and run the executable:

```bash
go build -o simple_backend_server .
./simple_backend_server
```

By default, the server listens on port `8080`.

## Configuration

The server can be configured using environment variables:

- `ADDR`: The address and port to listen on. Defaults to `:8080`.
- `JWT_SECRET`: The secret key used to sign JSON Web Tokens. If not provided, it falls back to an insecure default (`dev-only-secret-do-not-use-in-prod`).

## API Endpoints

The server uses `chi` for routing and provides the following endpoints:

### Health

- `GET /health`
  - Returns `{"status": "ok"}` if the server is running.

### Authentication (`/api/auth`)

The server uses an entirely fake/stubbed authentication system. Any username and password combination provided to `/sign-in` will successfully authenticate and generate a unique user ID (`fake-{username}`).

- `POST /api/auth/sign-in`
  - Body: `{"username": "your_username", "password": "any_password"}`
  - Returns: Access token, refresh token, and user info.
- `POST /api/auth/refresh`
  - Body: `{"refresh_token": "..."}`
  - Returns: A new access token and a rotated refresh token.
- `POST /api/auth/sign-out`
  - Body: `{"refresh_token": "..."}`
  - Action: Revokes the provided refresh token.
- `GET /api/auth/me`
  - Headers: `Authorization: Bearer <access_token>`
  - Returns: The currently authenticated user's information.

### Bookmarks (`/api/bookmarks`)

A complete CRUD API for managing bookmarks. All endpoints under this route require a valid JWT access token provided in the `Authorization` header. Data is scoped to the authenticated user.

- `GET /api/bookmarks`
  - Returns: A list of bookmarks owned by the current user.
- `POST /api/bookmarks`
  - Body: `{"id": "optional", "title": "...", "url": "...", "description": "...", "tags": ["..."]}`
  - Returns: The newly created bookmark.
  - *Note: If `id` is omitted, the server will generate a random ID. Providing an `id` is useful for offline-first clients.*
- `GET /api/bookmarks/{id}`
  - Returns: The bookmark with the specified ID.
- `PUT /api/bookmarks/{id}`
  - Body: Same as POST.
  - Returns: The updated bookmark.
- `DELETE /api/bookmarks/{id}`
  - Action: Deletes the specified bookmark.
