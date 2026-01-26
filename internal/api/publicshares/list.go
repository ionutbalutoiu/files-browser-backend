package publicshares

import (
	"net/http"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/httputil"
	"files-browser-backend/internal/service"
)

// ListHandler handles GET /api/public-shares requests.
type ListHandler struct {
	Config config.Config
}

// NewListHandler creates a new public shares list handler.
func NewListHandler(cfg config.Config) *ListHandler {
	return &ListHandler{Config: cfg}
}

// ServeHTTP handles GET /api/public-shares requests.
// Returns a JSON array of relative paths to all publicly shared files.
func (h *ListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !sharingEnabled(h.Config.PublicBaseDir, w) {
		return
	}
	files, ok := h.listFiles(w, r)
	if !ok {
		return
	}
	httputil.JSONResponse(w, http.StatusOK, files)
}

// listFiles retrieves all publicly shared files.
func (h *ListHandler) listFiles(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	files, err := service.ListSharePublicFiles(r.Context(), h.Config.PublicBaseDir)
	if err != nil {
		httputil.HandlePathError(w, err, "list public shares")
		return nil, false
	}
	// API boundary: return [] instead of null for empty results.
	if files == nil {
		return []string{}, true
	}
	return files, true
}
