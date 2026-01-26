package publicshares

import (
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
	if !sharingEnabled(h.Config.PublicBaseDir, w) {
		return
	}
	path, ok := h.parsePath(w, r)
	if !ok {
		return
	}
	if !h.deleteShare(w, r, path) {
		return
	}
	log.Printf("OK: deleted public share for %s", path)
	w.WriteHeader(http.StatusNoContent)
}

// parsePath extracts and validates the path query parameter.
func (h *DeleteHandler) parsePath(w http.ResponseWriter, r *http.Request) (string, bool) {
	path := r.URL.Query().Get("path")
	if path == "" {
		httputil.ErrorResponse(w, http.StatusBadRequest, "path query parameter is required")
		return "", false
	}
	if err := pathutil.ValidateRelativePath(path); err != nil {
		httputil.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return "", false
	}
	return path, true
}

// deleteShare removes the public share symlink.
func (h *DeleteHandler) deleteShare(w http.ResponseWriter, r *http.Request, path string) bool {
	if err := service.DeletePublicShare(r.Context(), h.Config.PublicBaseDir, path); err != nil {
		httputil.HandlePathError(w, err, "public-share delete")
		return false
	}
	return true
}
