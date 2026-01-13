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

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/handlers"
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
	uploadHandler := handlers.NewUploadHandler(cfg)
	deleteHandler := handlers.NewDeleteHandler(cfg)
	mkdirHandler := handlers.NewMkdirHandler(cfg)
	renameHandler := handlers.NewRenameHandler(cfg)
	sharePublicHandler := handlers.NewSharePublicHandler(cfg)
	sharePublicFilesHandler := handlers.NewSharePublicFilesHandler(cfg)
	sharePublicDeleteHandler := handlers.NewSharePublicDeleteHandler(cfg)
	healthHandler := handlers.NewHealthHandler()

	mux.Handle("/api/upload/", uploadHandler)
	mux.Handle("/api/delete/", deleteHandler)
	mux.Handle("/api/mkdir/", mkdirHandler)
	mux.Handle("/api/rename/", renameHandler)
	mux.Handle("/api/share-public/", sharePublicHandler)
	mux.Handle("/api/share-public-files/", sharePublicFilesHandler)
	mux.Handle("/api/share-public-delete", sharePublicDeleteHandler)
	mux.Handle("/api/health", healthHandler)

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
