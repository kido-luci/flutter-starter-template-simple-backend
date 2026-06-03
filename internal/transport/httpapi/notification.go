package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (rt *Router) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	list, err := rt.notifications.List(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch notifications.")
		return
	}
	writeJSON(w, http.StatusOK, toNotificationDTOs(list))
}

func (rt *Router) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	if err := rt.notifications.MarkRead(chi.URLParam(r, "id"), u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to update notification.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (rt *Router) handleListActivity(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r)
	list, err := rt.notifications.ListActivity(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch activities.")
		return
	}
	writeJSON(w, http.StatusOK, toActivityDTOs(list))
}
