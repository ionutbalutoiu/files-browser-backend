package actions

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/httputil"
	"files-browser-backend/internal/pathutil"
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

// ServeHTTP handles POST /api/files/move requests.
// Request body: {"from": "old/path", "to": "new/path"}
//
// SECURITY CRITICAL:
// - Uses Lstat to avoid following symlinks
// - Validates both source and destination paths are within base directory
// - Rejects path traversal, absolute paths, and symlink escapes
// - Does not allow overwriting existing files
func (h *MoveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse JSON body
	var req MoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Validate required fields
	if req.From == "" {
		httputil.ErrorResponse(w, http.StatusBadRequest, "from field is required")
		return
	}
	if req.To == "" {
		httputil.ErrorResponse(w, http.StatusBadRequest, "to field is required")
		return
	}

	// Resolve and validate paths for move
	resolvedSource, resolvedDest, virtualSource, virtualDest, err := pathutil.ResolveMovePaths(h.Config.BaseDir, req.From, req.To)
	if err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			httputil.ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: move path resolution failed: %v", err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Perform move
	if err := os.Rename(resolvedSource, resolvedDest); err != nil {
		if os.IsNotExist(err) {
			httputil.ErrorResponse(w, http.StatusNotFound, "source path does not exist")
			return
		}
		if os.IsPermission(err) {
			httputil.ErrorResponse(w, http.StatusForbidden, "permission denied")
			return
		}
		log.Printf("ERROR: move failed from %s to %s: %v", resolvedSource, resolvedDest, err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "move failed")
		return
	}

	log.Printf("OK: moved %s to %s", resolvedSource, resolvedDest)
	httputil.JSONResponse(w, http.StatusOK, MoveResponse{
		From:    virtualSource,
		To:      virtualDest,
		Success: true,
	})
}
