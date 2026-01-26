package publicshares

import (
	"errors"
	"log"
	"net/http"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/httputil"
	"files-browser-backend/internal/pathutil"
	"files-browser-backend/internal/service"
)

// DeleteHandler handles DELETE /api/public-shares?path=... requests.
type DeleteHandler struct {
	Config config.Config
}

// NewDeleteHandler creates a new public shares DELETE handler.
func NewDeleteHandler(cfg config.Config) *DeleteHandler {
	return &DeleteHandler{Config: cfg}
}

// ServeHTTP handles DELETE /api/public-shares?path=... requests.
// Deletes a public share symlink identified by the path query parameter.
//
// SECURITY:
// - Validates path is safe (no path traversal, no absolute paths)
// - Only deletes symlinks (not regular files or directories)
// - Never removes public-base-dir itself during cleanup
func (h *DeleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if public sharing is enabled
	if h.Config.PublicBaseDir == "" {
		httputil.ErrorResponse(w, http.StatusNotImplemented, "public sharing is not enabled (public-base-dir not configured)")
		return
	}

	// Extract path from query parameter
	path := r.URL.Query().Get("path")
	if path == "" {
		httputil.ErrorResponse(w, http.StatusBadRequest, "path query parameter is required")
		return
	}

	// Validate path doesn't contain traversal attempts
	if err := pathutil.ValidateRelativePath(path); err != nil {
		httputil.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Delete the public share
	if err := service.DeletePublicShare(r.Context(), h.Config.PublicBaseDir, path); err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			httputil.ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: public-share delete failed for %s: %v", path, err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	log.Printf("OK: deleted public share for %s", path)
	w.WriteHeader(http.StatusNoContent)
}
