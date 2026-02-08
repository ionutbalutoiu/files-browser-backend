// Package config provides configuration loading and defaults for the file service.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Environment variable names.
const (
	envListenAddr    = "FILES_SVC_LISTEN_ADDR"
	envBaseDir       = "FILES_SVC_BASE_DIR"
	envPublicBaseDir = "FILES_SVC_PUBLIC_BASE_DIR"
	envMaxUploadSize = "FILES_SVC_MAX_UPLOAD_SIZE"
)

// Default configuration values.
const (
	defaultListenAddr    = ":8080"
	defaultBaseDir       = "/srv/files"
	defaultPublicBaseDir = "/srv/files-public"
	defaultMaxUploadSize = 2 * 1024 * 1024 * 1024 // 2GB
)

// Config holds the service configuration.
type Config struct {
	ListenAddr    string
	BaseDir       string
	PublicBaseDir string
	MaxUploadSize int64
}

// DefaultConfig returns a Config with default values.
// ListenAddr is read from FILES_SVC_LISTEN_ADDR environment variable,
// falling back to :8080 if not set.
// BaseDir is read from FILES_SVC_BASE_DIR environment variable,
// falling back to /srv/files if not set.
// PublicBaseDir is read from FILES_SVC_PUBLIC_BASE_DIR environment variable,
// falling back to /srv/files-public if not set.
// MaxUploadSize is read from FILES_SVC_MAX_UPLOAD_SIZE environment variable,
// falling back to 2GB if not set.
func DefaultConfig() Config {
	return Config{
		ListenAddr:    envString(envListenAddr, defaultListenAddr),
		BaseDir:       envString(envBaseDir, defaultBaseDir),
		PublicBaseDir: envString(envPublicBaseDir, defaultPublicBaseDir),
		MaxUploadSize: envInt64(envMaxUploadSize, defaultMaxUploadSize),
	}
}

// Validate checks the configuration and resolves the base directory path.
// It returns the validated config with an absolute BaseDir path.
func (c Config) Validate() (Config, error) {
	if c.ListenAddr == "" {
		return c, fmt.Errorf("listen address is required")
	}
	if c.MaxUploadSize <= 0 {
		return c, fmt.Errorf("max upload size must be greater than zero")
	}

	absBase, err := resolveDir(c.BaseDir)
	if err != nil {
		return c, fmt.Errorf("base directory: %w", err)
	}
	c.BaseDir = absBase

	if c.PublicBaseDir != "" {
		absPublic, err := ensureDir(c.PublicBaseDir)
		if err != nil {
			return c, fmt.Errorf("public base directory: %w", err)
		}
		c.PublicBaseDir = absPublic
	}

	return c, nil
}

// envString returns the value of the environment variable or the fallback if not set.
func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt64 returns the value of the environment variable parsed as int64, or the fallback if not set or invalid.
func envInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

// resolveDir resolves path to absolute and validates it exists as a directory.
func resolveDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	return abs, nil
}

// ensureDir resolves path to absolute, creates it if needed, and validates it's a directory.
func ensureDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	return abs, nil
}
