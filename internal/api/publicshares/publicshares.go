package publicshares

import (
	"files-browser-backend/internal/httputil"
	"net/http"
)

// sharingEnabled checks if public sharing is configured and returns an error response if not.
func sharingEnabled(publicBaseDir string, w http.ResponseWriter) bool {
	if publicBaseDir == "" {
		httputil.ErrorResponse(w, http.StatusNotImplemented, "public sharing is not enabled (public-base-dir not configured)")
		return false
	}
	return true
}
