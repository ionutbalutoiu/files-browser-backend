// Package api provides HTTP route registration for the files-browser API.
package api

import (
	"net/http"

	"files-browser-backend/internal/api/files"
	"files-browser-backend/internal/api/files/actions"
	"files-browser-backend/internal/api/folders"
	"files-browser-backend/internal/api/health"
	"files-browser-backend/internal/api/publicshares"
	"files-browser-backend/internal/config"
)

// RegisterRoutes registers all API routes on the given mux.
func RegisterRoutes(mux *http.ServeMux, cfg config.Config) {
	// Health
	mux.Handle("GET /healthz", health.NewHandler())

	// Files
	mux.Handle("PUT /api/files", files.NewUploadHandler(cfg))
	mux.Handle("DELETE /api/files", files.NewDeleteHandler(cfg))

	// File actions (action sub-resources)
	mux.Handle("POST /api/files/move", actions.NewMoveHandler(cfg))
	mux.Handle("POST /api/files/rename", actions.NewRenameHandler(cfg))

	// Folders
	mux.Handle("POST /api/folders", folders.NewCreateHandler(cfg))

	// Public shares
	mux.Handle("GET /api/public-shares", publicshares.NewListHandler(cfg))
	mux.Handle("POST /api/public-shares", publicshares.NewCreateHandler(cfg))
	mux.Handle("DELETE /api/public-shares", publicshares.NewDeleteHandler(cfg))
}
