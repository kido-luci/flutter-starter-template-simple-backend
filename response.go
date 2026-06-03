package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// errorResponse is the JSON envelope returned for any non-2xx API response.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON encodes body as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// writeError writes a structured errorResponse with the given status code.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}
