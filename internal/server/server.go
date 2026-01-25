// Package server provides HTTP server setup and graceful shutdown.
package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"files-browser-backend/internal/api"
	"files-browser-backend/internal/config"
)

// Server wraps the HTTP server with configuration.
type Server struct {
	config     config.Config
	httpServer *http.Server
}

// New creates a new Server with the given configuration.
func New(cfg config.Config) *Server {
	mux := http.NewServeMux()

	// Register handlers
	mux.Handle("POST /api/upload/{path...}", api.NewUploadHandler(cfg))
	mux.Handle("DELETE /api/delete/{path...}", api.NewDeleteHandler(cfg))
	mux.Handle("POST /api/mkdir/{path...}", api.NewMkdirHandler(cfg))
	mux.Handle("POST /api/rename/{path...}", api.NewRenameHandler(cfg))
	mux.Handle("POST /api/mv/{path...}", api.NewMoveHandler(cfg))
	mux.Handle("POST /api/share-public/{path...}", api.NewSharePublicHandler(cfg))
	mux.Handle("GET /api/share-public-files/{path...}", api.NewSharePublicFilesHandler(cfg))
	mux.Handle("DELETE /api/share-public-delete", api.NewSharePublicDeleteHandler(cfg))
	mux.Handle("GET /api/health", api.NewHealthHandler())

	httpServer := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  0, // No timeout for large uploads
		WriteTimeout: 0, // No timeout for large uploads
		IdleTimeout:  120 * time.Second,
	}

	return &Server{
		config:     cfg,
		httpServer: httpServer,
	}
}

// Run starts the server and blocks until shutdown.
// It handles graceful shutdown on SIGINT and SIGTERM.
func (s *Server) Run() error {
	// Graceful shutdown handling
	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		s.httpServer.SetKeepAlivesEnabled(false)
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Fatalf("Could not gracefully shutdown the server: %v", err)
		}
		close(done)
	}()

	log.Printf("File server starting on %s", s.config.ListenAddr)
	log.Printf("Base directory: %s", s.config.BaseDir)
	if s.config.PublicBaseDir != "" {
		log.Printf("Public base directory: %s", s.config.PublicBaseDir)
	}
	log.Printf("Max upload size: %d bytes (%.2f GB)", s.config.MaxUploadSize, float64(s.config.MaxUploadSize)/(1024*1024*1024))

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	<-done
	log.Println("Server stopped")
	return nil
}
