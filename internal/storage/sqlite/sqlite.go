// Package sqlite provides SQLite-backed implementations of the domain
// repository interfaces.
package sqlite

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

// Open connects to the SQLite database at dsn, verifies the connection, and
// applies the schema migrations.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	const schema = `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS refresh_tokens (
		token TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		expires_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS bookmarks (
		id TEXT PRIMARY KEY,
		owner_id TEXT NOT NULL,
		title TEXT NOT NULL,
		url TEXT NOT NULL,
		description TEXT,
		tags TEXT,
		image_urls TEXT,
		video_url TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS activities (
		id TEXT PRIMARY KEY,
		owner_id TEXT NOT NULL,
		description TEXT NOT NULL,
		type TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS notifications (
		id TEXT PRIMARY KEY,
		owner_id TEXT NOT NULL,
		title TEXT NOT NULL,
		body TEXT NOT NULL,
		type TEXT NOT NULL,
		is_read BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL
	);
	`
	_, err := db.Exec(schema)
	return err
}
