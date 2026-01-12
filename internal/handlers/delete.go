package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/fs"
	"files-browser-backend/internal/pathutil"
)

// DeleteHandler handles file/directory deletion requests.
type DeleteHandler struct {
	Config config.Config
}

// NewDeleteHandler creates a new delete handler.
func NewDeleteHandler(cfg config.Config) *DeleteHandler {
	return &DeleteHandler{Config: cfg}
}

// ServeHTTP handles DELETE /delete/<path> requests.
// Security: Uses Lstat to avoid following symlinks, validates path is strictly
// within base directory, and refuses to delete the base directory itself.
func (h *DeleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow DELETE method
	if r.Method != http.MethodDelete {
		ErrorResponse(w, http.StatusMethodNotAllowed, "only DELETE method is allowed")
		return
	}

	// Extract target path from URL
	targetPath := strings.TrimPrefix(r.URL.Path, "/delete")
	targetPath = strings.TrimPrefix(targetPath, "/")
	targetPath = strings.TrimSuffix(targetPath, "/") // Normalize trailing slash

	// Resolve and validate target path for deletion
	resolvedPath, err := pathutil.ResolveDeletePath(h.Config.BaseDir, targetPath)
	if err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: delete path resolution failed: %v", err)
		ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Perform deletion
	if err := fs.Delete(resolvedPath); err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: deletion failed for %s: %v", resolvedPath, err)
		ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	log.Printf("OK: deleted %s", resolvedPath)
	w.WriteHeader(http.StatusNoContent)
}
