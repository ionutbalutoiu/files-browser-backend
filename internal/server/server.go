// Package server provides HTTP server setup and graceful shutdown.
package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"files-browser-backend/internal/api"
	"files-browser-backend/internal/config"
)

const shutdownTimeout = 30 * time.Second
const readHeaderTimeout = 10 * time.Second
const maxHeaderBytes = 1 << 20 // 1 MiB

// Server wraps the HTTP server with configuration.
type Server struct {
	cfg        config.Config
	httpServer *http.Server
}

// New creates a new Server with the given configuration.
func New(cfg config.Config) *Server {
	mux := http.NewServeMux()
	api.RegisterRoutes(mux, cfg)

	return &Server{
		cfg: cfg,
		httpServer: &http.Server{
			Addr:              cfg.ListenAddr,
			Handler:           mux,
			IdleTimeout:       120 * time.Second,
			ReadHeaderTimeout: readHeaderTimeout,
			MaxHeaderBytes:    maxHeaderBytes,
			// ReadTimeout and WriteTimeout default to 0 (no timeout for large uploads).
		},
	}
}

// Run starts the server and blocks until shutdown.
// It handles graceful shutdown on SIGINT and SIGTERM.
func (s *Server) Run() error {
	shutdownErr := make(chan error, 1)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go s.handleShutdown(ctx, shutdownErr)

	s.logStartupInfo()

	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	if err := <-shutdownErr; err != nil {
		return err
	}
	log.Println("Server stopped")
	return nil
}

// handleShutdown waits for termination signals and gracefully shuts down the server.
func (s *Server) handleShutdown(signalCtx context.Context, errCh chan<- error) {
	<-signalCtx.Done()
	log.Println("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	errCh <- s.httpServer.Shutdown(ctx)
}

// logStartupInfo logs server configuration at startup.
func (s *Server) logStartupInfo() {
	log.Printf("File server starting on %s", s.cfg.ListenAddr)
	log.Printf("Base directory: %s", s.cfg.BaseDir)
	if s.cfg.PublicBaseDir != "" {
		log.Printf("Public base directory: %s", s.cfg.PublicBaseDir)
	}
	log.Printf("Max upload size: %d bytes (%.2f GB)",
		s.cfg.MaxUploadSize, float64(s.cfg.MaxUploadSize)/(1024*1024*1024))
}
