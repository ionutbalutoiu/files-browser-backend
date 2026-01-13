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

// MkdirResponse is the JSON response for mkdir requests.
type MkdirResponse struct {
	Created string `json:"created"`
}

// MkdirHandler handles directory creation requests.
type MkdirHandler struct {
	Config config.Config
}

// NewMkdirHandler creates a new mkdir handler.
func NewMkdirHandler(cfg config.Config) *MkdirHandler {
	return &MkdirHandler{Config: cfg}
}

// ServeHTTP handles POST /mkdir/<path>/ requests.
// The final path component is the directory to be created.
// Parent directories must already exist (no recursive mkdir).
//
// SECURITY CRITICAL:
// - Uses Lstat to avoid following symlinks
// - Validates path is strictly within base directory
// - Rejects path traversal, absolute paths, and symlink escapes
// - Rejects directory names with path separators or null bytes
func (h *MkdirHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "only POST method is allowed")
		return
	}

	// Extract target path from URL
	targetPath := strings.TrimPrefix(r.URL.Path, "/api/mkdir")
	targetPath = strings.TrimPrefix(targetPath, "/")
	targetPath = strings.TrimSuffix(targetPath, "/") // Normalize trailing slash

	// Resolve and validate target path for directory creation
	resolvedPath, virtualPath, err := pathutil.ResolveMkdirPath(h.Config.BaseDir, targetPath)
	if err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: mkdir path resolution failed: %v", err)
		ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Create the directory
	if err := fs.Mkdir(resolvedPath); err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: mkdir failed for %s: %v", resolvedPath, err)
		ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	log.Printf("OK: created directory %s", resolvedPath)
	JSONResponse(w, http.StatusCreated, MkdirResponse{Created: virtualPath + "/"})
}
