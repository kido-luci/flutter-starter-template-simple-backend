package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxUploadBytes = 10 << 20 // 10 MiB

// handleUpload stores a single multipart "file" field under the uploads
// directory and returns its public URL.
func (rt *Router) handleUpload(w http.ResponseWriter, r *http.Request) {
	// Enforce a hard cap on the upload. ParseMultipartForm's argument is only
	// the in-memory spill threshold, not a size limit, so wrap the body in a
	// MaxBytesReader to actually reject oversized requests.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "File exceeds the 10MB upload limit.")
		return
	}
	file, handler, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_file", "Failed to get file from form: "+err.Error())
		return
	}
	defer file.Close()

	if err := os.MkdirAll(rt.uploadsDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "Failed to create uploads directory.")
		return
	}

	ext := ""
	if parts := strings.Split(handler.Filename, "."); len(parts) > 1 {
		ext = "." + parts[len(parts)-1]
	}
	token, err := rt.ids.NewID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "Failed to generate file name.")
		return
	}
	filename := token + ext
	filePath := filepath.Join(rt.uploadsDir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "Failed to save file on disk.")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "Failed to copy file contents.")
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	fileURL := fmt.Sprintf("%s://%s/uploads/%s", scheme, r.Host, filename)

	writeJSON(w, http.StatusOK, map[string]string{"url": fileURL})
}
