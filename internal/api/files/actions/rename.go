package actions

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/httputil"
	"files-browser-backend/internal/pathutil"
)

// RenameRequest is the JSON request for rename operations.
type RenameRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// RenameResponse is the JSON response for rename operations.
type RenameResponse struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Success bool   `json:"success"`
}

// RenameHandler handles POST /api/files/rename requests.
type RenameHandler struct {
	Config config.Config
}

// NewRenameHandler creates a new files rename handler.
func NewRenameHandler(cfg config.Config) *RenameHandler {
	return &RenameHandler{Config: cfg}
}

// ServeHTTP handles POST /api/files/rename requests.
// Request body: {"path": "dir/file.txt", "name": "newname.txt"}
//
// SECURITY CRITICAL:
// - Uses Lstat to avoid following symlinks
// - Validates path is within base directory
// - Validates new name contains no path separators
// - Rejects path traversal and absolute paths
func (h *RenameHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse JSON body
	var req RenameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Validate required fields
	if req.Path == "" {
		httputil.ErrorResponse(w, http.StatusBadRequest, "path field is required")
		return
	}
	if req.Name == "" {
		httputil.ErrorResponse(w, http.StatusBadRequest, "name field is required")
		return
	}

	// Validate new name doesn't contain path separators
	if filepath.Base(req.Name) != req.Name || req.Name == "." || req.Name == ".." {
		httputil.ErrorResponse(w, http.StatusBadRequest, "name must be a simple filename without path separators")
		return
	}

	// Build the destination path (same directory, new name)
	destPath := filepath.Join(filepath.Dir(req.Path), req.Name)

	// Resolve and validate paths for rename
	resolvedSource, resolvedDest, virtualSource, virtualDest, err := pathutil.ResolveMovePaths(h.Config.BaseDir, req.Path, destPath)
	if err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			httputil.ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: rename path resolution failed: %v", err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Perform rename
	if err := os.Rename(resolvedSource, resolvedDest); err != nil {
		if os.IsNotExist(err) {
			httputil.ErrorResponse(w, http.StatusNotFound, "source path does not exist")
			return
		}
		if os.IsPermission(err) {
			httputil.ErrorResponse(w, http.StatusForbidden, "permission denied")
			return
		}
		log.Printf("ERROR: rename failed from %s to %s: %v", resolvedSource, resolvedDest, err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "rename failed")
		return
	}

	log.Printf("OK: renamed %s to %s", resolvedSource, resolvedDest)
	httputil.JSONResponse(w, http.StatusOK, RenameResponse{
		From:    virtualSource,
		To:      virtualDest,
		Success: true,
	})
}
