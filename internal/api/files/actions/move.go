package actions

import (
	"errors"
	"net/http"
	"os"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/httputil"
	"files-browser-backend/internal/pathutil"
	"files-browser-backend/internal/service"
)

// MoveRequest is the JSON request body for moving files or directories.
type MoveRequest struct {
	// From is the source path relative to base directory (e.g., "docs/old.txt").
	From string `json:"from"`
	// To is the destination path relative to base directory (e.g., "archive/new.txt").
	To string `json:"to"`
}

// MoveResponse is the JSON response for move operations.
type MoveResponse struct {
	// From is the original path that was moved.
	From string `json:"from"`
	// To is the new path after the move.
	To string `json:"to"`
	// Success indicates whether the move operation completed successfully.
	Success bool `json:"success"`
}

// MoveHandler handles POST /api/files/move requests.
type MoveHandler struct {
	Config config.Config
}

// NewMoveHandler creates a new files move handler.
func NewMoveHandler(cfg config.Config) *MoveHandler {
	return &MoveHandler{Config: cfg}
}

// validateMoveRequest validates the required fields of a move request.
func validateMoveRequest(req MoveRequest) error {
	if req.From == "" {
		return errors.New("from field is required")
	}
	if req.To == "" {
		return errors.New("to field is required")
	}
	return nil
}

// ServeHTTP handles POST /api/files/move requests.
// Request body: {"from": "old/path", "to": "new/path"}
//
// SECURITY CRITICAL:
// - Uses Lstat to avoid following symlinks.
// - Validates both source and destination paths are within base directory.
// - Rejects path traversal, absolute paths, and symlink escapes.
// - Does not allow overwriting existing files.
func (h *MoveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req, err := httputil.DecodeJSON[MoveRequest](r)
	if err != nil {
		httputil.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := validateMoveRequest(req); err != nil {
		httputil.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	resolvedSource, resolvedDest, virtualSource, virtualDest, err := pathutil.ResolveMovePaths(
		h.Config.BaseDir, req.From, req.To,
	)
	if err != nil {
		httputil.HandlePathError(w, err, "move path resolution")
		return
	}

	// Deny move if source contains any public shares.
	if service.ContainsPublicShare(h.Config.BaseDir, h.Config.PublicBaseDir, resolvedSource) {
		httputil.ErrorResponse(w, http.StatusForbidden, "cannot move path containing public shares")
		return
	}

	if err := os.Rename(resolvedSource, resolvedDest); err != nil {
		httputil.HandleRenameError(w, err, "move")
		return
	}

	httputil.JSONResponse(w, http.StatusOK, MoveResponse{
		From:    virtualSource,
		To:      virtualDest,
		Success: true,
	})
}
