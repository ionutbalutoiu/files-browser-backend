package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/fs"
	"files-browser-backend/internal/pathutil"
)

// SharePublicDeleteRequest is the JSON request for deleting a public share.
type SharePublicDeleteRequest struct {
	Path string `json:"path"`
}

// SharePublicDeleteResponse is the JSON response for successful deletion.
type SharePublicDeleteResponse struct {
	Deleted string `json:"deleted"`
}

// SharePublicDeleteHandler handles requests to delete public share symlinks.
type SharePublicDeleteHandler struct {
	Config config.Config
}

// NewSharePublicDeleteHandler creates a new handler for deleting public shares.
func NewSharePublicDeleteHandler(cfg config.Config) *SharePublicDeleteHandler {
	return &SharePublicDeleteHandler{Config: cfg}
}

// ServeHTTP handles DELETE /api/share-public-delete requests.
// Deletes a public share symlink and cleans up empty parent directories.
//
// Request body (JSON):
//
//	{ "path": "dir1/dir2/my-file.txt" }
//
// SECURITY:
// - Validates path is safe (no path traversal, no absolute paths)
// - Only deletes symlinks (not regular files or directories)
// - Never removes public-base-dir itself during cleanup
func (h *SharePublicDeleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow DELETE method
	if r.Method != http.MethodDelete {
		ErrorResponse(w, http.StatusMethodNotAllowed, "only DELETE method is allowed")
		return
	}

	// Check if public sharing is enabled
	if h.Config.PublicBaseDir == "" {
		ErrorResponse(w, http.StatusNotImplemented, "public sharing is not enabled (public-base-dir not configured)")
		return
	}

	// Parse JSON body
	var req SharePublicDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Validate path is provided
	if req.Path == "" {
		ErrorResponse(w, http.StatusBadRequest, "path is required")
		return
	}

	// Delete the public share
	if err := fs.DeletePublicShare(h.Config.PublicBaseDir, req.Path); err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: share-public-delete failed for %s: %v", req.Path, err)
		ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	log.Printf("OK: deleted public share for %s", req.Path)
	JSONResponse(w, http.StatusOK, SharePublicDeleteResponse{Deleted: req.Path})
}
