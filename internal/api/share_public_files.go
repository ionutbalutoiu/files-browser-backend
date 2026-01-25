package api

import (
	"encoding/json"
	"log"
	"net/http"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/service"
)

// SharePublicFilesHandler handles requests to list publicly shared files.
type SharePublicFilesHandler struct {
	Config config.Config
}

// NewSharePublicFilesHandler creates a new handler for listing public shared files.
func NewSharePublicFilesHandler(cfg config.Config) *SharePublicFilesHandler {
	return &SharePublicFilesHandler{Config: cfg}
}

// ServeHTTP handles GET /api/share-public-files requests.
// Returns a JSON array of relative paths to all publicly shared files.
func (h *SharePublicFilesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET method
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "only GET method is allowed")
		return
	}

	// Check if public sharing is enabled
	if h.Config.PublicBaseDir == "" {
		ErrorResponse(w, http.StatusNotImplemented, "public sharing is not enabled (public-base-dir not configured)")
		return
	}

	// List all publicly shared files
	files, err := service.ListSharePublicFiles(h.Config.PublicBaseDir)
	if err != nil {
		log.Printf("ERROR: failed to list public shared files: %v", err)
		ErrorResponse(w, http.StatusInternalServerError, "failed to list public shared files")
		return
	}

	// Return empty array instead of null if no files
	if files == nil {
		files = []string{}
	}

	// Write JSON array directly (not wrapped in object)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(files)
}
