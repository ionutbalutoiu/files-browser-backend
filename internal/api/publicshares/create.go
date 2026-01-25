package publicshares

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/httputil"
	"files-browser-backend/internal/pathutil"
	"files-browser-backend/internal/service"
)

// encodeShareID encodes a relative path to a URL-safe base64 shareId.
func encodeShareID(path string) string {
	return base64.URLEncoding.EncodeToString([]byte(path))
}

// CreateRequest is the JSON request for creating a public share.
type CreateRequest struct {
	Path string `json:"path"`
}

// CreateResponse is the JSON response for a public share.
type CreateResponse struct {
	ShareID string `json:"shareId"`
	Path    string `json:"path"`
}

// CreateHandler handles POST /api/public-shares requests.
type CreateHandler struct {
	Config config.Config
}

// NewCreateHandler creates a new public shares create handler.
func NewCreateHandler(cfg config.Config) *CreateHandler {
	return &CreateHandler{Config: cfg}
}

// ServeHTTP handles POST /api/public-shares requests.
// Creates a symlink in the public base directory pointing to the source file.
// Request body: {"path": "dir1/file.txt"}
//
// SECURITY CRITICAL:
// - Only regular files can be shared (not directories or symlinks)
// - Uses Lstat to avoid following symlinks
// - Validates path is strictly within base directory
// - Validates symlink destination is within public base directory
// - Rejects path traversal and absolute paths
func (h *CreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if public sharing is enabled
	if h.Config.PublicBaseDir == "" {
		httputil.ErrorResponse(w, http.StatusNotImplemented, "public sharing is not enabled (public-base-dir not configured)")
		return
	}

	// Parse JSON body
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Validate path is provided
	if req.Path == "" {
		httputil.ErrorResponse(w, http.StatusBadRequest, "path is required")
		return
	}

	// Resolve and validate target path for sharing
	resolvedPath, virtualPath, err := pathutil.ResolveSharePublicPath(h.Config.BaseDir, req.Path)
	if err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			httputil.ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: share-public path resolution failed: %v", err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Create the public share symlink
	if err := service.SharePublic(resolvedPath, h.Config.PublicBaseDir, virtualPath); err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			httputil.ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: share-public failed for %s: %v", resolvedPath, err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	log.Printf("OK: created public share for %s", resolvedPath)
	httputil.JSONResponse(w, http.StatusCreated, CreateResponse{
		ShareID: encodeShareID(virtualPath),
		Path:    virtualPath,
	})
}
