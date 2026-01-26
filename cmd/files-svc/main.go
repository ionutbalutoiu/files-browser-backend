// Package main provides the entry point for the files-svc service.
package main

import (
	"flag"
	"log"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/server"
)

func main() {
	cfg := parseFlags()

	validatedCfg, err := cfg.Validate()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	srv := server.New(validatedCfg)
	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// parseFlags parses command-line flags and returns the configuration.
func parseFlags() config.Config {
	cfg := config.DefaultConfig()

	flag.StringVar(&cfg.ListenAddr, "listen", cfg.ListenAddr,
		"Address to listen on (env: FILES_SVC_LISTEN_ADDR)")
	flag.StringVar(&cfg.BaseDir, "base-dir", cfg.BaseDir,
		"Base directory for file storage (env: FILES_SVC_BASE_DIR)")
	flag.StringVar(&cfg.PublicBaseDir, "public-base-dir", cfg.PublicBaseDir,
		"Base directory for public share symlinks (env: FILES_SVC_PUBLIC_BASE_DIR)")
	flag.Int64Var(&cfg.MaxUploadSize, "max-upload-size", cfg.MaxUploadSize,
		"Maximum upload size in bytes (env: FILES_SVC_MAX_UPLOAD_SIZE)")
	flag.Parse()

	return cfg
}
