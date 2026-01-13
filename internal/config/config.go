// Package config provides configuration loading and defaults for the file service.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds the service configuration.
type Config struct {
	ListenAddr    string
	BaseDir       string
	PublicBaseDir string
	MaxUploadSize int64
}

// DefaultConfig returns a Config with default values.
// BaseDir is read from FILES_SVC_UPLOAD_BASE_DIR environment variable,
// falling back to /srv/files if not set.
// PublicBaseDir is read from FILES_SVC_PUBLIC_BASE_DIR environment variable.
// MaxUploadSize is read from FILES_SVC_MAX_UPLOAD_SIZE environment variable,
// falling back to 2GB if not set.
func DefaultConfig() Config {
	baseDir := os.Getenv("FILES_SVC_UPLOAD_BASE_DIR")
	if baseDir == "" {
		baseDir = "/srv/files"
	}

	publicBaseDir := os.Getenv("FILES_SVC_PUBLIC_BASE_DIR")

	maxUploadSize := int64(2 * 1024 * 1024 * 1024) // 2GB default
	if envSize := os.Getenv("FILES_SVC_MAX_UPLOAD_SIZE"); envSize != "" {
		if parsed, err := strconv.ParseInt(envSize, 10, 64); err == nil {
			maxUploadSize = parsed
		}
	}

	return Config{
		ListenAddr:    ":8080",
		BaseDir:       baseDir,
		PublicBaseDir: publicBaseDir,
		MaxUploadSize: maxUploadSize,
	}
}

// Validate checks the configuration and resolves the base directory path.
// It returns the validated config with an absolute BaseDir path.
func (c Config) Validate() (Config, error) {
	// Resolve and validate base directory
	absBase, err := filepath.Abs(c.BaseDir)
	if err != nil {
		return c, fmt.Errorf("invalid base directory: %w", err)
	}

	// Ensure base directory exists
	info, err := os.Stat(absBase)
	if err != nil {
		return c, fmt.Errorf("base directory error: %w", err)
	}
	if !info.IsDir() {
		return c, fmt.Errorf("base path is not a directory: %s", absBase)
	}

	c.BaseDir = absBase

	// Validate and resolve public base directory if set
	if c.PublicBaseDir != "" {
		absPublic, err := filepath.Abs(c.PublicBaseDir)
		if err != nil {
			return c, fmt.Errorf("invalid public base directory: %w", err)
		}

		// Create public base directory if it doesn't exist
		if err := os.MkdirAll(absPublic, 0755); err != nil {
			return c, fmt.Errorf("failed to create public base directory: %w", err)
		}

		// Verify it's a directory
		info, err := os.Stat(absPublic)
		if err != nil {
			return c, fmt.Errorf("public base directory error: %w", err)
		}
		if !info.IsDir() {
			return c, fmt.Errorf("public base path is not a directory: %s", absPublic)
		}

		c.PublicBaseDir = absPublic
	}

	return c, nil
}
