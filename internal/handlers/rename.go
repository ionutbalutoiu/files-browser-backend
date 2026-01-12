package handlers

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/pathutil"
)

// RenameResponse is the JSON response for rename requests.
type RenameResponse struct {
	Old     string `json:"old"`
	New     string `json:"new"`
	Success bool   `json:"success"`
}

// RenameHandler handles file/directory rename requests.
type RenameHandler struct {
	Config config.Config
}

// NewRenameHandler creates a new rename handler.
func NewRenameHandler(cfg config.Config) *RenameHandler {
	return &RenameHandler{Config: cfg}
}

// ServeHTTP handles POST /rename/<oldPath>?newName=<newName> requests.
// Renames a file or directory within the base directory.
//
// SECURITY CRITICAL:
// - Uses Lstat to avoid following symlinks
// - Validates both old path and new name are within base directory
// - Rejects path traversal, absolute paths, and symlink escapes
// - Does not allow overwriting existing files
func (h *RenameHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow POST or PATCH methods
	if r.Method != http.MethodPost && r.Method != http.MethodPatch {
		ErrorResponse(w, http.StatusMethodNotAllowed, "only POST or PATCH methods are allowed")
		return
	}

	// Extract old path from URL
	oldPath := strings.TrimPrefix(r.URL.Path, "/rename")
	oldPath = strings.TrimPrefix(oldPath, "/")
	oldPath = strings.TrimSuffix(oldPath, "/") // Normalize trailing slash

	// Get new name from query parameter
	newName := r.URL.Query().Get("newName")
	if newName == "" {
		ErrorResponse(w, http.StatusBadRequest, "newName query parameter is required")
		return
	}

	// Resolve and validate paths for rename
	resolvedOld, resolvedNew, virtualOld, virtualNew, err := pathutil.ResolveRenamePaths(h.Config.BaseDir, oldPath, newName)
	if err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: rename path resolution failed: %v", err)
		ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Perform rename
	if err := os.Rename(resolvedOld, resolvedNew); err != nil {
		if os.IsNotExist(err) {
			ErrorResponse(w, http.StatusNotFound, "source path does not exist")
			return
		}
		if os.IsPermission(err) {
			ErrorResponse(w, http.StatusForbidden, "permission denied")
			return
		}
		log.Printf("ERROR: rename failed from %s to %s: %v", resolvedOld, resolvedNew, err)
		ErrorResponse(w, http.StatusInternalServerError, "rename failed")
		return
	}

	log.Printf("OK: renamed %s to %s", resolvedOld, resolvedNew)
	JSONResponse(w, http.StatusOK, RenameResponse{
		Old:     virtualOld,
		New:     virtualNew,
		Success: true,
	})
}
