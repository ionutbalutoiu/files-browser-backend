package files

import (
	"errors"
	"log"
	"net/http"

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
	// Extract target path from query parameter
	targetPath := r.URL.Query().Get("path")
	if targetPath == "" {
		httputil.ErrorResponse(w, http.StatusBadRequest, "path query parameter is required")
		return
	}

	// Resolve and validate target path for deletion
	resolvedPath, err := pathutil.ResolveDeletePath(h.Config.BaseDir, targetPath)
	if err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			httputil.ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: delete path resolution failed: %v", err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Perform deletion
	if err := service.Delete(resolvedPath); err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			httputil.ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: deletion failed for %s: %v", resolvedPath, err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	log.Printf("OK: deleted %s", resolvedPath)
	w.WriteHeader(http.StatusNoContent)
}
