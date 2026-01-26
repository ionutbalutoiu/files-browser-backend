// Package publicshares provides HTTP handlers for public file sharing operations.
package publicshares

import (
	"encoding/base64"
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

// CreateRequest is the JSON request body for creating a public share.
type CreateRequest struct {
	// Path is the file path relative to base directory to share publicly (e.g., "docs/file.txt").
	Path string `json:"path"`
}

// CreateResponse is the JSON response for a successfully created public share.
type CreateResponse struct {
	// ShareID is the URL-safe base64-encoded identifier for the public share.
	ShareID string `json:"shareId"`
	// Path is the relative path of the shared file within the public directory.
	Path string `json:"path"`
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
	if !sharingEnabled(h.Config.PublicBaseDir, w) {
		return
	}
	req, ok := h.parseRequest(w, r)
	if !ok {
		return
	}
	resolvedPath, virtualPath, ok := h.resolvePath(w, req.Path)
	if !ok {
		return
	}
	if !h.createShare(w, r, resolvedPath, virtualPath) {
		return
	}
	log.Printf("OK: created public share for %s", resolvedPath)
	httputil.JSONResponse(w, http.StatusCreated, CreateResponse{
		ShareID: encodeShareID(virtualPath),
		Path:    virtualPath,
	})
}

// parseRequest decodes and validates the JSON request body.
func (h *CreateHandler) parseRequest(w http.ResponseWriter, r *http.Request) (CreateRequest, bool) {
	req, err := httputil.DecodeJSON[CreateRequest](r)
	if err != nil {
		httputil.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return CreateRequest{}, false
	}
	if req.Path == "" {
		httputil.ErrorResponse(w, http.StatusBadRequest, "path is required")
		return CreateRequest{}, false
	}
	return req, true
}

// resolvePath validates and resolves the target path for sharing.
func (h *CreateHandler) resolvePath(w http.ResponseWriter, path string) (resolved, virtual string, ok bool) {
	resolved, virtual, err := pathutil.ResolveSharePublicPath(h.Config.BaseDir, path)
	if err != nil {
		httputil.HandlePathError(w, err, "share-public path resolution")
		return "", "", false
	}
	return resolved, virtual, true
}

// createShare creates the public share symlink at the resolved path.
func (h *CreateHandler) createShare(w http.ResponseWriter, r *http.Request, resolved, virtual string) bool {
	if err := service.SharePublic(r.Context(), resolved, h.Config.PublicBaseDir, virtual); err != nil {
		httputil.HandlePathError(w, err, "share-public")
		return false
	}
	return true
}
