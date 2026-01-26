package health

import (
	"log"
	"net/http"
)

// Handler handles health check requests.
type Handler struct{}

// NewHandler creates a new health handler.
func NewHandler() *Handler {
	return &Handler{}
}

// ServeHTTP handles GET /healthz requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Printf("WARN: failed to write health response: %v", err)
	}
}
