package main

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func initDB(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := createSchema(db); err != nil {
		return nil, err
	}

	return db, nil
}

func createSchema(db *sql.DB) error {
	schema := `
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
	`
	_, err := db.Exec(schema)
	return err
}
