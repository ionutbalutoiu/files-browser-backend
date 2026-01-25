package api

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/pathutil"
)

// MoveResponse is the JSON response for move requests.
type MoveResponse struct {
	Source  string `json:"source"`
	Dest    string `json:"dest"`
	Success bool   `json:"success"`
}

// MoveHandler handles file/directory move requests.
type MoveHandler struct {
	Config config.Config
}

// NewMoveHandler creates a new move handler.
func NewMoveHandler(cfg config.Config) *MoveHandler {
	return &MoveHandler{Config: cfg}
}

// ServeHTTP handles POST /api/mv/<sourcePath>?dest=<destPath> requests.
// Moves a file or directory to a new location within the base directory.
//
// SECURITY CRITICAL:
// - Uses Lstat to avoid following symlinks
// - Validates both source and destination paths are within base directory
// - Rejects path traversal, absolute paths, and symlink escapes
// - Does not allow overwriting existing files
func (h *MoveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow POST or PATCH methods
	if r.Method != http.MethodPost && r.Method != http.MethodPatch {
		ErrorResponse(w, http.StatusMethodNotAllowed, "only POST or PATCH methods are allowed")
		return
	}

	// Extract source path from URL
	sourcePath := strings.TrimPrefix(r.URL.Path, "/api/mv")
	sourcePath = strings.TrimPrefix(sourcePath, "/")
	sourcePath = strings.TrimSuffix(sourcePath, "/") // Normalize trailing slash

	// Get destination path from query parameter
	destPath := r.URL.Query().Get("dest")
	if destPath == "" {
		ErrorResponse(w, http.StatusBadRequest, "dest query parameter is required")
		return
	}

	// Resolve and validate paths for move
	resolvedSource, resolvedDest, virtualSource, virtualDest, err := pathutil.ResolveMovePaths(h.Config.BaseDir, sourcePath, destPath)
	if err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: move path resolution failed: %v", err)
		ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Perform move
	if err := os.Rename(resolvedSource, resolvedDest); err != nil {
		if os.IsNotExist(err) {
			ErrorResponse(w, http.StatusNotFound, "source path does not exist")
			return
		}
		if os.IsPermission(err) {
			ErrorResponse(w, http.StatusForbidden, "permission denied")
			return
		}
		log.Printf("ERROR: move failed from %s to %s: %v", resolvedSource, resolvedDest, err)
		ErrorResponse(w, http.StatusInternalServerError, "move failed")
		return
	}

	log.Printf("OK: moved %s to %s", resolvedSource, resolvedDest)
	JSONResponse(w, http.StatusOK, MoveResponse{
		Source:  virtualSource,
		Dest:    virtualDest,
		Success: true,
	})
}
