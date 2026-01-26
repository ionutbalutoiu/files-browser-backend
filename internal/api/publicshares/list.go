package publicshares

import (
	"log"
	"net/http"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/httputil"
	"files-browser-backend/internal/service"
)

// ListHandler handles GET /api/public-shares requests.
type ListHandler struct {
	Config config.Config
}

// NewListHandler creates a new public shares list handler.
func NewListHandler(cfg config.Config) *ListHandler {
	return &ListHandler{Config: cfg}
}

// ServeHTTP handles GET /api/public-shares requests.
// Returns a JSON array of relative paths to all publicly shared files.
func (h *ListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if public sharing is enabled
	if h.Config.PublicBaseDir == "" {
		httputil.ErrorResponse(w, http.StatusNotImplemented, "public sharing is not enabled (public-base-dir not configured)")
		return
	}

	// List all publicly shared files
	files, err := service.ListSharePublicFiles(r.Context(), h.Config.PublicBaseDir)
	if err != nil {
		log.Printf("ERROR: failed to list public shared files: %v", err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "failed to list public shared files")
		return
	}

	// Return empty array instead of null if no files
	if files == nil {
		files = []string{}
	}

	httputil.JSONResponse(w, http.StatusOK, files)
}
