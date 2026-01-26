package files

import (
	"net/http"
	"path/filepath"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/httputil"
	"files-browser-backend/internal/pathutil"
	"files-browser-backend/internal/service"
)

// DeleteHandler handles DELETE /api/files?path=... requests.
type DeleteHandler struct {
	Config config.Config
}

// NewDeleteHandler creates a new files DELETE handler.
func NewDeleteHandler(cfg config.Config) *DeleteHandler {
	return &DeleteHandler{Config: cfg}
}

// ServeHTTP handles DELETE /api/files?path=<path> requests.
// Security: Uses Lstat to avoid following symlinks, validates path is strictly
// within base directory, and refuses to delete the base directory itself.
func (h *DeleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		httputil.ErrorResponse(w, http.StatusBadRequest, "path query parameter is required")
		return
	}

	resolvedPath, err := pathutil.ResolveDeletePath(h.Config.BaseDir, path)
	if err != nil {
		httputil.HandlePathError(w, err, "delete path resolution")
		return
	}

	if err := service.Delete(r.Context(), resolvedPath); err != nil {
		httputil.HandlePathError(w, err, "delete")
		return
	}

	// Clean up associated public share symlink if it exists (best-effort).
	relPath := filepath.Clean(path)
	service.DeletePublicShareIfExists(r.Context(), h.Config.PublicBaseDir, relPath)

	w.WriteHeader(http.StatusNoContent)
}
