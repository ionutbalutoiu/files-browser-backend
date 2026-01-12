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

// RenameTestResponse matches the JSON response structure
type RenameTestResponse struct {
	Old     string `json:"old"`
	New     string `json:"new"`
	Success bool   `json:"success"`
}

// ErrorTestResponse matches the JSON error response structure
type ErrorTestResponse struct {
	Error string `json:"error"`
}

func setupRenameHandler(t *testing.T) (config.Config, *handlers.RenameHandler, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "rename-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       tmpDir,
		MaxUploadSize: 10 * 1024 * 1024,
	}

	return cfg, handlers.NewRenameHandler(cfg), tmpDir
}

func TestRenameFileSuccess(t *testing.T) {
	_, renameHandler, tmpDir := setupRenameHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a file to rename
	oldPath := filepath.Join(tmpDir, "oldfile.txt")
	if err := os.WriteFile(oldPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/rename/oldfile.txt?newName=newfile.txt", nil)
	rr := httptest.NewRecorder()
	renameHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp RenameTestResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
	if resp.Old != "oldfile.txt" {
		t.Errorf("unexpected old path: %s", resp.Old)
	}
	if resp.New != "newfile.txt" {
		t.Errorf("unexpected new path: %s", resp.New)
	}

	// Verify old file is gone
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old file should not exist")
	}

	// Verify new file exists with correct content
	newPath := filepath.Join(tmpDir, "newfile.txt")
	content, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("failed to read renamed file: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestRenameDirectorySuccess(t *testing.T) {
	_, renameHandler, tmpDir := setupRenameHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a directory with a file inside
	oldDir := filepath.Join(tmpDir, "olddir")
	if err := os.Mkdir(oldDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/rename/olddir?newName=newdir", nil)
	rr := httptest.NewRecorder()
	renameHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp RenameTestResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
	if resp.Old != "olddir" {
		t.Errorf("unexpected old path: %s", resp.Old)
	}
	if resp.New != "newdir" {
		t.Errorf("unexpected new path: %s", resp.New)
	}

	// Verify old directory is gone
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("old directory should not exist")
	}

	// Verify new directory exists with file inside
	newDir := filepath.Join(tmpDir, "newdir")
	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("failed to stat renamed directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("renamed path should be a directory")
	}

	// Verify file inside is intact
	content, err := os.ReadFile(filepath.Join(newDir, "file.txt"))
	if err != nil {
		t.Fatalf("failed to read file in renamed directory: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestRenameNestedPath(t *testing.T) {
	_, renameHandler, tmpDir := setupRenameHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a nested file
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	oldPath := filepath.Join(subDir, "oldfile.txt")
	if err := os.WriteFile(oldPath, []byte("nested content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/rename/subdir/oldfile.txt?newName=newfile.txt", nil)
	rr := httptest.NewRecorder()
	renameHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp RenameTestResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Old != "subdir/oldfile.txt" {
		t.Errorf("unexpected old path: %s", resp.Old)
	}
	if resp.New != "subdir/newfile.txt" {
		t.Errorf("unexpected new path: %s", resp.New)
	}

	// Verify rename happened in the nested location
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old file should not exist")
	}
	newPath := filepath.Join(subDir, "newfile.txt")
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("new file should exist: %v", err)
	}
}

func TestRenameNotFound(t *testing.T) {
	_, renameHandler, tmpDir := setupRenameHandler(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodPost, "/rename/nonexistent.txt?newName=newfile.txt", nil)
	rr := httptest.NewRecorder()
	renameHandler.ServeHTTP(rr, req)

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

func TestRenameDestinationExists(t *testing.T) {
	_, renameHandler, tmpDir := setupRenameHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create source file
	oldPath := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(oldPath, []byte("source"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create destination file
	newPath := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(newPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/rename/source.txt?newName=existing.txt", nil)
	rr := httptest.NewRecorder()
	renameHandler.ServeHTTP(rr, req)

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
	sourceContent, _ := os.ReadFile(oldPath)
	if string(sourceContent) != "source" {
		t.Error("source file should be unchanged")
	}
	existingContent, _ := os.ReadFile(newPath)
	if string(existingContent) != "existing" {
		t.Error("existing file should be unchanged")
	}
}

func TestRenameMissingNewName(t *testing.T) {
	_, renameHandler, tmpDir := setupRenameHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a file
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/rename/file.txt", nil)
	rr := httptest.NewRecorder()
	renameHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRenamePathTraversal(t *testing.T) {
	_, renameHandler, tmpDir := setupRenameHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a file
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		newName string
	}{
		{"traversal in path", "/rename/../etc/passwd?newName=new.txt", ""},
		{"traversal in newName", "/rename/file.txt?newName=../outside.txt", "../outside.txt"},
		{"path separator in newName", "/rename/file.txt?newName=sub/file.txt", "sub/file.txt"},
		{"absolute newName", "/rename/file.txt?newName=/etc/passwd", "/etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			rr := httptest.NewRecorder()
			renameHandler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
				t.Errorf("expected 400 or 404, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestRenameMethodNotAllowed(t *testing.T) {
	_, renameHandler, tmpDir := setupRenameHandler(t)
	defer os.RemoveAll(tmpDir)

	methods := []string{http.MethodGet, http.MethodDelete, http.MethodPut}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/rename/file.txt?newName=new.txt", nil)
			rr := httptest.NewRecorder()
			renameHandler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s: expected 405, got %d", method, rr.Code)
			}
		})
	}
}

func TestRenamePatchMethod(t *testing.T) {
	_, renameHandler, tmpDir := setupRenameHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a file
	oldPath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(oldPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// PATCH should work too
	req := httptest.NewRequest(http.MethodPatch, "/rename/file.txt?newName=renamed.txt", nil)
	rr := httptest.NewRecorder()
	renameHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRenameSymlink(t *testing.T) {
	_, renameHandler, tmpDir := setupRenameHandler(t)
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

	req := httptest.NewRequest(http.MethodPost, "/rename/link.txt?newName=renamed.txt", nil)
	rr := httptest.NewRecorder()
	renameHandler.ServeHTTP(rr, req)

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

func TestRenameEmptyPath(t *testing.T) {
	_, renameHandler, tmpDir := setupRenameHandler(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodPost, "/rename/?newName=something.txt", nil)
	rr := httptest.NewRecorder()
	renameHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRenameInvalidNewName(t *testing.T) {
	_, renameHandler, tmpDir := setupRenameHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a file
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		newName string
	}{
		{"dot", "."},
		{"double dot", ".."},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/rename/file.txt?newName="+tt.newName, nil)
			rr := httptest.NewRecorder()
			renameHandler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}
