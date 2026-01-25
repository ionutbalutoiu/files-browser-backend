package api

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/pathutil"
	"files-browser-backend/internal/service"
)

// SharePublicResponse is the JSON response for share-public requests.
type SharePublicResponse struct {
	Shared string `json:"shared"`
}

// SharePublicHandler handles public share requests.
type SharePublicHandler struct {
	Config config.Config
}

// NewSharePublicHandler creates a new share-public handler.
func NewSharePublicHandler(cfg config.Config) *SharePublicHandler {
	return &SharePublicHandler{Config: cfg}
}

// ServeHTTP handles POST /share-public/<path> requests.
// Creates a symlink in the public base directory pointing to the source file.
//
// SECURITY CRITICAL:
// - Only regular files can be shared (not directories or symlinks)
// - Uses Lstat to avoid following symlinks
// - Validates path is strictly within base directory
// - Validates symlink destination is within public base directory
// - Rejects path traversal and absolute paths
func (h *SharePublicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		ErrorResponse(w, http.StatusMethodNotAllowed, "only POST method is allowed")
		return
	}

	// Check if public sharing is enabled
	if h.Config.PublicBaseDir == "" {
		ErrorResponse(w, http.StatusNotImplemented, "public sharing is not enabled (public-base-dir not configured)")
		return
	}

	// Extract target path from URL
	targetPath := strings.TrimPrefix(r.URL.Path, "/api/share-public")
	targetPath = strings.TrimPrefix(targetPath, "/")
	targetPath = strings.TrimSuffix(targetPath, "/") // Normalize trailing slash

	// Resolve and validate target path for sharing
	resolvedPath, virtualPath, err := pathutil.ResolveSharePublicPath(h.Config.BaseDir, targetPath)
	if err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: share-public path resolution failed: %v", err)
		ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Create the public share symlink
	if err := service.SharePublic(resolvedPath, h.Config.PublicBaseDir, virtualPath); err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: share-public failed for %s: %v", resolvedPath, err)
		ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	log.Printf("OK: created public share for %s", resolvedPath)
	JSONResponse(w, http.StatusCreated, SharePublicResponse{Shared: virtualPath})
}
