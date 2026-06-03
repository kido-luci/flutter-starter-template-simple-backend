package main

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type NotificationDto struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Type      string    `json:"type"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

type UserActivityDto struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
}

func getNotificationsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := r.Context().Value(userCtxKey).(user)
		rows, err := db.Query("SELECT id, title, body, type, is_read, created_at FROM notifications WHERE owner_id = ? ORDER BY created_at DESC", u.ID)
		out := make([]NotificationDto, 0)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch notifications.")
			return
		}
		defer rows.Close()

		for rows.Next() {
			var n NotificationDto
			if err := rows.Scan(&n.ID, &n.Title, &n.Body, &n.Type, &n.IsRead, &n.CreatedAt); err == nil {
				out = append(out, n)
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func markNotificationReadHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := r.Context().Value(userCtxKey).(user)
		id := chi.URLParam(r, "id")

		_, err := db.Exec("UPDATE notifications SET is_read = 1 WHERE id = ? AND owner_id = ?", id, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to update notification.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func getActivityHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := r.Context().Value(userCtxKey).(user)
		rows, err := db.Query("SELECT id, description, type, created_at FROM activities WHERE owner_id = ? ORDER BY created_at DESC", u.ID)
		out := make([]UserActivityDto, 0)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch activities.")
			return
		}
		defer rows.Close()

		for rows.Next() {
			var a UserActivityDto
			if err := rows.Scan(&a.ID, &a.Description, &a.Type, &a.CreatedAt); err == nil {
				out = append(out, a)
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}
