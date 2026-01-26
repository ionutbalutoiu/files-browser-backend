package actions

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/httputil"
	"files-browser-backend/internal/pathutil"
	"files-browser-backend/internal/service"
)

// RenameRequest is the JSON request body for renaming files or directories.
type RenameRequest struct {
	// Path is the current path relative to base directory (e.g., "docs/oldname.txt").
	Path string `json:"path"`
	// Name is the new name for the file or directory (no path separators allowed).
	Name string `json:"name"`
}

// RenameResponse is the JSON response for rename operations.
type RenameResponse struct {
	// From is the original path before renaming.
	From string `json:"from"`
	// To is the new path after renaming.
	To string `json:"to"`
	// Success indicates whether the rename operation completed successfully.
	Success bool `json:"success"`
}

// RenameHandler handles POST /api/files/rename requests.
type RenameHandler struct {
	Config config.Config
}

// NewRenameHandler creates a new files rename handler.
func NewRenameHandler(cfg config.Config) *RenameHandler {
	return &RenameHandler{Config: cfg}
}

// validateRenameRequest validates the required fields and name format of a rename request.
func validateRenameRequest(req RenameRequest) error {
	if req.Path == "" {
		return errors.New("path field is required")
	}
	if req.Name == "" {
		return errors.New("name field is required")
	}
	if filepath.Base(req.Name) != req.Name || req.Name == "." || req.Name == ".." {
		return errors.New("name must be a simple filename without path separators")
	}
	return nil
}

// ServeHTTP handles POST /api/files/rename requests.
// Request body: {"path": "dir/file.txt", "name": "newname.txt"}
//
// SECURITY CRITICAL:
// - Uses Lstat to avoid following symlinks.
// - Validates path is within base directory.
// - Validates new name contains no path separators.
// - Rejects path traversal and absolute paths.
func (h *RenameHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req, err := httputil.DecodeJSON[RenameRequest](r)
	if err != nil {
		httputil.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := validateRenameRequest(req); err != nil {
		httputil.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	destPath := filepath.Join(filepath.Dir(req.Path), req.Name)
	resolvedSource, resolvedDest, virtualSource, virtualDest, err := pathutil.ResolveMovePaths(
		h.Config.BaseDir, req.Path, destPath,
	)
	if err != nil {
		httputil.HandlePathError(w, err, "rename path resolution")
		return
	}

	// Deny rename if source contains any public shares.
	if service.ContainsPublicShare(h.Config.BaseDir, h.Config.PublicBaseDir, resolvedSource) {
		httputil.ErrorResponse(w, http.StatusForbidden, "cannot rename path containing public shares")
		return
	}

	if err := os.Rename(resolvedSource, resolvedDest); err != nil {
		httputil.HandleRenameError(w, err, "rename")
		return
	}

	httputil.JSONResponse(w, http.StatusOK, RenameResponse{
		From:    virtualSource,
		To:      virtualDest,
		Success: true,
	})
}
