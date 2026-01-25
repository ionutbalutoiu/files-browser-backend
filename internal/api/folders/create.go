package folders

import (
	"encoding/json"
	"errors"
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
// - Uses Lstat to avoid following symlinks
// - Validates path is strictly within base directory
// - Rejects path traversal, absolute paths, and symlink escapes
// - Rejects directory names with path separators or null bytes
func (h *CreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	// Resolve and validate target path for directory creation
	resolvedPath, virtualPath, err := pathutil.ResolveMkdirPath(h.Config.BaseDir, req.Path)
	if err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			httputil.ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: mkdir path resolution failed: %v", err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Create the directory
	if err := service.Mkdir(resolvedPath); err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			httputil.ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: mkdir failed for %s: %v", resolvedPath, err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	log.Printf("OK: created directory %s", resolvedPath)
	httputil.JSONResponse(w, http.StatusCreated, CreateResponse{Created: virtualPath + "/"})
}
