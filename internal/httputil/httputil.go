// Package httputil provides shared HTTP response utilities for the API handlers.
package httputil

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"files-browser-backend/internal/pathutil"
)

// writeJSON encodes data as JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("WARN: encode json response: %v", err)
	}
}

// ErrorResponse sends a JSON error response with the given status code and message.
func ErrorResponse(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// JSONResponse sends a JSON response with the given status code and data.
func JSONResponse(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, data)
}

// HandlePathError writes an appropriate HTTP error response for path-related errors.
// For PathError types, it uses the error's status code and message.
// For other errors, it logs the error with operation context and returns a 500.
func HandlePathError(w http.ResponseWriter, err error, operation string) {
	var pathErr *pathutil.PathError
	if errors.As(err, &pathErr) {
		ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
		return
	}
	log.Printf("ERROR: %s: %v", operation, err)
	ErrorResponse(w, http.StatusInternalServerError, "internal server error")
}

// HandleRenameError writes an appropriate HTTP error response for os.Rename errors.
// It maps common filesystem errors to appropriate HTTP status codes.
func HandleRenameError(w http.ResponseWriter, err error, operation string) {
	switch {
	case os.IsNotExist(err):
		ErrorResponse(w, http.StatusNotFound, "source path does not exist")
	case os.IsPermission(err):
		ErrorResponse(w, http.StatusForbidden, "permission denied")
	default:
		ErrorResponse(w, http.StatusInternalServerError, operation+" failed")
	}
}

// DecodeJSON decodes a JSON request body into the provided type.
// Returns the decoded value and any error encountered.
func DecodeJSON[T any](r *http.Request) (T, error) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, err
	}
	return v, nil
}
