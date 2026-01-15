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

// MoveTestResponse matches the JSON response structure
type MoveTestResponse struct {
	Source  string `json:"source"`
	Dest    string `json:"dest"`
	Success bool   `json:"success"`
}

func setupMoveHandler(t *testing.T) (config.Config, *handlers.MoveHandler, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "move-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       tmpDir,
		MaxUploadSize: 10 * 1024 * 1024,
	}

	return cfg, handlers.NewMoveHandler(cfg), tmpDir
}

func TestMoveFileSuccess(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create destination directory
	destDir := filepath.Join(tmpDir, "destdir")
	if err := os.Mkdir(destDir, 0755); err != nil {
		t.Fatalf("failed to create dest directory: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/mv/source.txt?dest=destdir/moved.txt", nil)
	rr := httptest.NewRecorder()
	moveHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp MoveTestResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
	if resp.Source != "source.txt" {
		t.Errorf("unexpected source path: %s", resp.Source)
	}
	if resp.Dest != "destdir/moved.txt" {
		t.Errorf("unexpected dest path: %s", resp.Dest)
	}

	// Verify source file is gone
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("source file should not exist")
	}

	// Verify destination file exists with correct content
	newPath := filepath.Join(destDir, "moved.txt")
	content, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("failed to read moved file: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestMoveDirectorySuccess(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create source directory with a file inside
	srcDir := filepath.Join(tmpDir, "srcdir")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create destination parent directory
	destParent := filepath.Join(tmpDir, "destparent")
	if err := os.Mkdir(destParent, 0755); err != nil {
		t.Fatalf("failed to create dest parent: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/mv/srcdir?dest=destparent/moveddir", nil)
	rr := httptest.NewRecorder()
	moveHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp MoveTestResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}

	// Verify source directory is gone
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Error("source directory should not exist")
	}

	// Verify destination directory exists with file inside
	movedDir := filepath.Join(destParent, "moveddir")
	info, err := os.Stat(movedDir)
	if err != nil {
		t.Fatalf("failed to stat moved directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("moved path should be a directory")
	}

	// Verify file inside is intact
	content, err := os.ReadFile(filepath.Join(movedDir, "file.txt"))
	if err != nil {
		t.Fatalf("failed to read file in moved directory: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestMoveToSameDirectory(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create source file
	srcPath := filepath.Join(tmpDir, "original.txt")
	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Move to same directory with different name (like rename)
	req := httptest.NewRequest(http.MethodPost, "/api/mv/original.txt?dest=renamed.txt", nil)
	rr := httptest.NewRecorder()
	moveHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify source is gone
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("source file should not exist")
	}

	// Verify destination exists
	destPath := filepath.Join(tmpDir, "renamed.txt")
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("destination should exist: %v", err)
	}
}

func TestMoveFromNestedPath(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create nested source
	srcDir := filepath.Join(tmpDir, "deep", "nested")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create nested directories: %v", err)
	}
	srcPath := filepath.Join(srcDir, "file.txt")
	if err := os.WriteFile(srcPath, []byte("nested content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/mv/deep/nested/file.txt?dest=file.txt", nil)
	rr := httptest.NewRecorder()
	moveHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp MoveTestResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Source != "deep/nested/file.txt" {
		t.Errorf("unexpected source path: %s", resp.Source)
	}
	if resp.Dest != "file.txt" {
		t.Errorf("unexpected dest path: %s", resp.Dest)
	}

	// Verify move happened
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("source file should not exist")
	}
	destPath := filepath.Join(tmpDir, "file.txt")
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read moved file: %v", err)
	}
	if string(content) != "nested content" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestMoveNotFound(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodPost, "/api/mv/nonexistent.txt?dest=new.txt", nil)
	rr := httptest.NewRecorder()
	moveHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ErrorTestResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == "" {
		t.Error("expected error message")
	}
}

func TestMoveDestinationExists(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("source"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create destination file
	destPath := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(destPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/mv/source.txt?dest=existing.txt", nil)
	rr := httptest.NewRecorder()
	moveHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ErrorTestResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != "destination already exists" {
		t.Errorf("unexpected error message: %s", resp.Error)
	}

	// Verify both files still exist with original content
	sourceContent, _ := os.ReadFile(srcPath)
	if string(sourceContent) != "source" {
		t.Error("source file should be unchanged")
	}
	existingContent, _ := os.ReadFile(destPath)
	if string(existingContent) != "existing" {
		t.Error("existing file should be unchanged")
	}
}

func TestMoveDestinationParentNotFound(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Try to move to non-existent parent directory
	req := httptest.NewRequest(http.MethodPost, "/api/mv/source.txt?dest=nonexistent/file.txt", nil)
	rr := httptest.NewRecorder()
	moveHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify source file still exists
	if _, err := os.Stat(srcPath); err != nil {
		t.Error("source file should still exist")
	}
}

func TestMoveMissingDestParam(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a file
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/mv/file.txt", nil)
	rr := httptest.NewRecorder()
	moveHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMovePathTraversal(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a file
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name   string
		path   string
		status int
	}{
		{"traversal in source", "/api/mv/../etc/passwd?dest=new.txt", http.StatusBadRequest},
		{"traversal in dest", "/api/mv/file.txt?dest=../outside.txt", http.StatusBadRequest},
		{"absolute source", "/api/mv//etc/passwd?dest=new.txt", http.StatusBadRequest},
		{"absolute dest", "/api/mv/file.txt?dest=/etc/passwd", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			rr := httptest.NewRecorder()
			moveHandler.ServeHTTP(rr, req)

			if rr.Code != tt.status && rr.Code != http.StatusNotFound {
				t.Errorf("expected %d or 404, got %d: %s", tt.status, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestMoveMethodNotAllowed(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	methods := []string{http.MethodGet, http.MethodDelete, http.MethodPut}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/mv/file.txt?dest=new.txt", nil)
			rr := httptest.NewRecorder()
			moveHandler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s: expected 405, got %d", method, rr.Code)
			}
		})
	}
}

func TestMovePatchMethod(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a file
	srcPath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// PATCH should work too
	req := httptest.NewRequest(http.MethodPatch, "/api/mv/file.txt?dest=moved.txt", nil)
	rr := httptest.NewRecorder()
	moveHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMoveSymlink(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a real file and a symlink
	realFile := filepath.Join(tmpDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create real file: %v", err)
	}

	symlinkPath := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(realFile, symlinkPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/mv/link.txt?dest=moved.txt", nil)
	rr := httptest.NewRecorder()
	moveHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for symlink, got %d: %s", rr.Code, rr.Body.String())
	}

	// Symlink and real file should still exist
	if _, err := os.Lstat(symlinkPath); err != nil {
		t.Error("symlink should still exist")
	}
	if _, err := os.Stat(realFile); err != nil {
		t.Error("real file should still exist")
	}
}

func TestMoveEmptyPath(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name string
		path string
	}{
		{"empty source", "/api/mv/?dest=file.txt"},
		{"empty dest", "/api/mv/file.txt?dest="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			rr := httptest.NewRecorder()
			moveHandler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestMoveToSymlinkParent(t *testing.T) {
	_, moveHandler, tmpDir := setupMoveHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a real directory and a symlink to it
	realDir := filepath.Join(tmpDir, "realdir")
	if err := os.Mkdir(realDir, 0755); err != nil {
		t.Fatalf("failed to create real directory: %v", err)
	}

	symlinkDir := filepath.Join(tmpDir, "linkdir")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Try to move to symlink directory
	req := httptest.NewRequest(http.MethodPost, "/api/mv/source.txt?dest=linkdir/file.txt", nil)
	rr := httptest.NewRecorder()
	moveHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for symlink parent, got %d: %s", rr.Code, rr.Body.String())
	}

	// Source should still exist
	if _, err := os.Stat(srcPath); err != nil {
		t.Error("source file should still exist")
	}
}
