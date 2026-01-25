// Package httputil provides shared HTTP response utilities for the API handlers.
package httputil

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorResponse sends a JSON error response with the given status code and message.
func ErrorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		log.Printf("WARN: failed to encode error response: %v", err)
	}
}

// JSONResponse sends a JSON response with the given status code and data.
func JSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("WARN: failed to encode JSON response: %v", err)
	}
}
