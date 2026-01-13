package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/handlers"
)

// setupTestHandlerWithPublic creates a test configuration with both base and public directories.
func setupTestHandlerWithPublic(t *testing.T) (config.Config, string, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "share-test-base-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	publicDir, err := os.MkdirTemp("", "share-test-public-*")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create public temp dir: %v", err)
	}

	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       tmpDir,
		PublicBaseDir: publicDir,
		MaxUploadSize: 10 * 1024 * 1024,
	}

	return cfg, tmpDir, publicDir
}

func TestSharePublicFile(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := handlers.NewSharePublicHandler(cfg)

	// Create a file to share
	os.MkdirAll(filepath.Join(tmpDir, "photos", "2026"), 0755)
	testFile := filepath.Join(tmpDir, "photos", "2026", "pic.jpg")
	os.WriteFile(testFile, []byte("image data"), 0644)

	req := httptest.NewRequest(http.MethodPost, "/api/share-public/photos/2026/pic.jpg", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Check response
	var resp handlers.SharePublicResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Shared != "photos/2026/pic.jpg" {
		t.Errorf("expected shared='photos/2026/pic.jpg', got '%s'", resp.Shared)
	}

	// Verify symlink was created
	linkPath := filepath.Join(publicDir, "photos", "2026", "pic.jpg")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file")
	}

	// Verify symlink points to correct target
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	if target != testFile {
		t.Errorf("symlink target: expected %s, got %s", testFile, target)
	}
}

func TestSharePublicIdempotent(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := handlers.NewSharePublicHandler(cfg)

	// Create a file
	testFile := filepath.Join(tmpDir, "doc.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	// Share it twice
	req1 := httptest.NewRequest(http.MethodPost, "/api/share-public/doc.txt", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusCreated {
		t.Errorf("first share: expected 201, got %d", rr1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/share-public/doc.txt", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	// Should be idempotent - success again
	if rr2.Code != http.StatusCreated {
		t.Errorf("second share: expected 201 (idempotent), got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestSharePublicDirectory(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := handlers.NewSharePublicHandler(cfg)

	// Create a directory
	os.MkdirAll(filepath.Join(tmpDir, "mydir"), 0755)

	req := httptest.NewRequest(http.MethodPost, "/api/share-public/mydir", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for directory, got %d: %s", rr.Code, rr.Body.String())
	}

	// Check error message
	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["error"] != "only regular files can be shared publicly" {
		t.Errorf("unexpected error message: %s", resp["error"])
	}
}

func TestSharePublicNonExistent(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := handlers.NewSharePublicHandler(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/share-public/does-not-exist.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestSharePublicPathTraversal(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := handlers.NewSharePublicHandler(cfg)

	tests := []struct {
		name string
		path string
	}{
		{"double dot", "/api/share-public/../etc/passwd"},
		{"hidden traversal", "/api/share-public/foo/../../etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
				t.Errorf("expected 400 or 404, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestSharePublicSymlink(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := handlers.NewSharePublicHandler(cfg)

	// Create a regular file and a symlink to it
	realFile := filepath.Join(tmpDir, "real.txt")
	os.WriteFile(realFile, []byte("content"), 0644)
	symlinkPath := filepath.Join(tmpDir, "link.txt")
	os.Symlink(realFile, symlinkPath)

	req := httptest.NewRequest(http.MethodPost, "/api/share-public/link.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for symlink, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["error"] != "cannot share symlinks" {
		t.Errorf("unexpected error message: %s", resp["error"])
	}
}

func TestSharePublicMethodNotAllowed(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := handlers.NewSharePublicHandler(cfg)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/share-public/file.txt", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405, got %d", rr.Code)
			}
		})
	}
}

func TestSharePublicNotEnabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "share-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Config WITHOUT PublicBaseDir
	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       tmpDir,
		PublicBaseDir: "", // Not configured
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := handlers.NewSharePublicHandler(cfg)

	// Create a file
	testFile := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	req := httptest.NewRequest(http.MethodPost, "/api/share-public/file.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["error"] != "public sharing is not enabled (public-base-dir not configured)" {
		t.Errorf("unexpected error message: %s", resp["error"])
	}
}

func TestSharePublicEmptyPath(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := handlers.NewSharePublicHandler(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/share-public/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
