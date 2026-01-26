// Package folders provides HTTP handlers for directory operations.
package folders

import (
	"log"
	"net/http"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/httputil"
	"files-browser-backend/internal/pathutil"
	"files-browser-backend/internal/service"
)

// CreateRequest is the JSON request for creating a folder.
type CreateRequest struct {
	Path string `json:"path"`
}

// CreateResponse is the JSON response for folder creation.
type CreateResponse struct {
	Created string `json:"created"`
}

// CreateHandler handles directory creation requests.
type CreateHandler struct {
	Config config.Config
}

// NewCreateHandler creates a new folders create handler.
func NewCreateHandler(cfg config.Config) *CreateHandler {
	return &CreateHandler{Config: cfg}
}

// ServeHTTP handles POST /api/folders requests.
// The path is specified in the JSON body: {"path": "dir1/newdir"}
//
// SECURITY CRITICAL:
// - Uses Lstat to avoid following symlinks.
// - Validates path is strictly within base directory.
// - Rejects path traversal, absolute paths, and symlink escapes.
// - Rejects directory names with path separators or null bytes.
func (h *CreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req, ok := h.parseRequest(w, r)
	if !ok {
		return
	}

	resolvedPath, virtualPath, ok := h.resolvePath(w, req.Path)
	if !ok {
		return
	}

	if !h.createDirectory(w, r, resolvedPath) {
		return
	}

	log.Printf("OK: created directory %s", resolvedPath)
	httputil.JSONResponse(w, http.StatusCreated, CreateResponse{Created: virtualPath + "/"})
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

// resolvePath validates and resolves the target path for directory creation.
func (h *CreateHandler) resolvePath(w http.ResponseWriter, path string) (resolved, virtual string, ok bool) {
	resolved, virtual, err := pathutil.ResolveMkdirPath(h.Config.BaseDir, path)
	if err != nil {
		httputil.HandlePathError(w, err, "mkdir path resolution")
		return "", "", false
	}
	return resolved, virtual, true
}

// createDirectory creates the directory at the resolved path.
func (h *CreateHandler) createDirectory(w http.ResponseWriter, r *http.Request, path string) bool {
	if err := service.Mkdir(r.Context(), path); err != nil {
		httputil.HandlePathError(w, err, "mkdir")
		return false
	}
	return true
}
