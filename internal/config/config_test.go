package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRejectsEmptyListenAddr(t *testing.T) {
	baseDir := t.TempDir()
	cfg := Config{
		ListenAddr:    "",
		BaseDir:       baseDir,
		PublicBaseDir: "",
		MaxUploadSize: 1024,
	}

	_, err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty listen address")
	}
	if !strings.Contains(err.Error(), "listen address is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsNonPositiveMaxUploadSize(t *testing.T) {
	baseDir := t.TempDir()
	cfg := Config{
		ListenAddr:    ":8080",
		BaseDir:       baseDir,
		PublicBaseDir: "",
		MaxUploadSize: 0,
	}

	_, err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for non-positive max upload size")
	}
	if !strings.Contains(err.Error(), "max upload size must be greater than zero") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateResolvesAndCreatesPublicBaseDir(t *testing.T) {
	baseDir := t.TempDir()
	parent := t.TempDir()
	publicDir := filepath.Join(parent, "public")

	cfg := Config{
		ListenAddr:    ":8080",
		BaseDir:       baseDir,
		PublicBaseDir: publicDir,
		MaxUploadSize: 1024,
	}

	validated, err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}

	if validated.BaseDir != baseDir {
		t.Fatalf("expected base dir %q, got %q", baseDir, validated.BaseDir)
	}
	if validated.PublicBaseDir != publicDir {
		t.Fatalf("expected public base dir %q, got %q", publicDir, validated.PublicBaseDir)
	}
	info, err := os.Stat(publicDir)
	if err != nil {
		t.Fatalf("public base dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("public base dir should be directory")
	}
}
