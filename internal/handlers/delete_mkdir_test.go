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

func TestDeleteFile(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	deleteHandler := handlers.NewDeleteHandler(cfg)

	// Create a file to delete
	os.MkdirAll(filepath.Join(tmpDir, "docs"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "docs", "test.txt"), []byte("content"), 0644)

	req := httptest.NewRequest(http.MethodDelete, "/delete/docs/test.txt", nil)
	rr := httptest.NewRecorder()
	deleteHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify file is gone
	if _, err := os.Stat(filepath.Join(tmpDir, "docs", "test.txt")); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestDeleteEmptyDirectory(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	deleteHandler := handlers.NewDeleteHandler(cfg)

	// Create an empty directory
	os.MkdirAll(filepath.Join(tmpDir, "empty-dir"), 0755)

	req := httptest.NewRequest(http.MethodDelete, "/delete/empty-dir/", nil)
	rr := httptest.NewRecorder()
	deleteHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify directory is gone
	if _, err := os.Stat(filepath.Join(tmpDir, "empty-dir")); !os.IsNotExist(err) {
		t.Error("directory should have been deleted")
	}
}

func TestDeleteNonEmptyDirectory(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	deleteHandler := handlers.NewDeleteHandler(cfg)

	// Create directory with a file
	os.MkdirAll(filepath.Join(tmpDir, "non-empty"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "non-empty", "file.txt"), []byte("content"), 0644)

	req := httptest.NewRequest(http.MethodDelete, "/delete/non-empty/", nil)
	rr := httptest.NewRecorder()
	deleteHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}

	// Directory should still exist
	if _, err := os.Stat(filepath.Join(tmpDir, "non-empty")); err != nil {
		t.Error("directory should still exist")
	}
}

func TestDeleteNonExistent(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	deleteHandler := handlers.NewDeleteHandler(cfg)

	req := httptest.NewRequest(http.MethodDelete, "/delete/does-not-exist.txt", nil)
	rr := httptest.NewRecorder()
	deleteHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestDeleteBaseDirectory(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	deleteHandler := handlers.NewDeleteHandler(cfg)

	// Try to delete base directory with empty path
	req := httptest.NewRequest(http.MethodDelete, "/delete/", nil)
	rr := httptest.NewRecorder()
	deleteHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDeletePathTraversal(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	deleteHandler := handlers.NewDeleteHandler(cfg)

	tests := []struct {
		name string
		path string
	}{
		{"double dot", "/delete/../etc/passwd"},
		{"encoded double dot", "/delete/..%2F..%2Fetc%2Fpasswd"},
		{"hidden traversal", "/delete/foo/../../etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, tt.path, nil)
			rr := httptest.NewRecorder()
			deleteHandler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
				t.Errorf("expected 400 or 404, got %d", rr.Code)
			}
		})
	}
}

func TestDeleteSymlink(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	deleteHandler := handlers.NewDeleteHandler(cfg)

	// Create a real file and a symlink to it
	realFile := filepath.Join(tmpDir, "real.txt")
	os.WriteFile(realFile, []byte("content"), 0644)

	symlinkPath := filepath.Join(tmpDir, "link.txt")
	os.Symlink(realFile, symlinkPath)

	req := httptest.NewRequest(http.MethodDelete, "/delete/link.txt", nil)
	rr := httptest.NewRecorder()
	deleteHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for symlink, got %d: %s", rr.Code, rr.Body.String())
	}

	// Both symlink and real file should still exist
	if _, err := os.Lstat(symlinkPath); err != nil {
		t.Error("symlink should still exist")
	}
	if _, err := os.Stat(realFile); err != nil {
		t.Error("real file should still exist")
	}
}

func TestDeleteMethodNotAllowed(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	deleteHandler := handlers.NewDeleteHandler(cfg)

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/delete/test.txt", nil)
		rr := httptest.NewRecorder()
		deleteHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405, got %d", method, rr.Code)
		}
	}
}

func TestDeleteAbsolutePath(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	deleteHandler := handlers.NewDeleteHandler(cfg)

	req := httptest.NewRequest(http.MethodDelete, "/delete//etc/passwd", nil)
	rr := httptest.NewRecorder()
	deleteHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Errorf("expected 400 or 404, got %d", rr.Code)
	}
}

// ============================================================================
// MKDIR TESTS
// ============================================================================

// MkdirTestResponse matches the JSON response structure
type MkdirTestResponse struct {
	Created string `json:"created"`
}

func setupMkdirHandler(t *testing.T) (config.Config, *handlers.MkdirHandler, string) {
	cfg, tmpDir := setupTestHandler(t)
	return cfg, handlers.NewMkdirHandler(cfg), tmpDir
}

func TestMkdirSimple(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodPost, "/mkdir/newdir/", nil)
	rr := httptest.NewRecorder()
	mkdirHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp MkdirTestResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Created != "newdir/" {
		t.Errorf("unexpected created path: %s", resp.Created)
	}

	// Verify directory exists
	info, err := os.Stat(filepath.Join(tmpDir, "newdir"))
	if err != nil {
		t.Fatalf("directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("path should be a directory")
	}
}

func TestMkdirNested(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create parent directory first
	os.MkdirAll(filepath.Join(tmpDir, "photos/2026"), 0755)

	req := httptest.NewRequest(http.MethodPost, "/mkdir/photos/2026/vacation/", nil)
	rr := httptest.NewRecorder()
	mkdirHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp MkdirTestResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Created != "photos/2026/vacation/" {
		t.Errorf("unexpected created path: %s", resp.Created)
	}

	// Verify directory exists
	info, err := os.Stat(filepath.Join(tmpDir, "photos/2026/vacation"))
	if err != nil {
		t.Fatalf("directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("path should be a directory")
	}
}

func TestMkdirParentNotExist(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	// Try to create nested directory without creating parent first
	req := httptest.NewRequest(http.MethodPost, "/mkdir/nonexistent/subdir/", nil)
	rr := httptest.NewRecorder()
	mkdirHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMkdirAlreadyExists(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create directory first
	os.MkdirAll(filepath.Join(tmpDir, "existing"), 0755)

	req := httptest.NewRequest(http.MethodPost, "/mkdir/existing/", nil)
	rr := httptest.NewRecorder()
	mkdirHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMkdirFileExists(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a file with the target name
	os.WriteFile(filepath.Join(tmpDir, "myfile"), []byte("content"), 0644)

	req := httptest.NewRequest(http.MethodPost, "/mkdir/myfile/", nil)
	rr := httptest.NewRecorder()
	mkdirHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMkdirBaseDirectory(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	// Try to create base directory (empty path)
	req := httptest.NewRequest(http.MethodPost, "/mkdir/", nil)
	rr := httptest.NewRecorder()
	mkdirHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMkdirPathTraversal(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name string
		path string
	}{
		{"double dot", "/mkdir/../escape/"},
		{"hidden traversal", "/mkdir/foo/../../escape/"},
		{"triple dot escape", "/mkdir/foo/../../../escape/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			rr := httptest.NewRecorder()
			mkdirHandler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden {
				t.Errorf("expected 400 or 403, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestMkdirAbsolutePath(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodPost, "/mkdir//etc/passwd/", nil)
	rr := httptest.NewRecorder()
	mkdirHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden && rr.Code != http.StatusNotFound {
		t.Errorf("expected 400, 403, or 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMkdirSymlinkParent(t *testing.T) {
	cfg, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)
	_ = cfg

	// Create a directory outside base (simulated with another dir in temp)
	outsideDir, err := os.MkdirTemp("", "outside-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outsideDir)

	// Create a symlink inside base pointing to outside
	symlinkPath := filepath.Join(tmpDir, "escape-link")
	os.Symlink(outsideDir, symlinkPath)

	// Try to create directory under the symlink
	req := httptest.NewRequest(http.MethodPost, "/mkdir/escape-link/newdir/", nil)
	rr := httptest.NewRecorder()
	mkdirHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for symlink escape, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify no directory was created outside
	if _, err := os.Stat(filepath.Join(outsideDir, "newdir")); !os.IsNotExist(err) {
		t.Error("directory should not have been created outside base")
	}
}

func TestMkdirSymlinkTarget(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a real directory
	os.MkdirAll(filepath.Join(tmpDir, "realdir"), 0755)

	// Create a symlink
	symlinkPath := filepath.Join(tmpDir, "link")
	os.Symlink(filepath.Join(tmpDir, "realdir"), symlinkPath)

	// Try to create the symlink as a directory
	req := httptest.NewRequest(http.MethodPost, "/mkdir/link/", nil)
	rr := httptest.NewRecorder()
	mkdirHandler.ServeHTTP(rr, req)

	// Should fail because path already exists as symlink
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMkdirMethodNotAllowed(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/mkdir/test/", nil)
		rr := httptest.NewRecorder()
		mkdirHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405, got %d", method, rr.Code)
		}
	}
}

func TestMkdirDirectoryWithSpaces(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodPost, "/mkdir/my%20folder/", nil)
	rr := httptest.NewRecorder()
	mkdirHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify directory with spaces exists
	info, err := os.Stat(filepath.Join(tmpDir, "my folder"))
	if err != nil {
		t.Fatalf("directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("path should be a directory")
	}
}

func TestMkdirPermissions(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodPost, "/mkdir/permtest/", nil)
	rr := httptest.NewRecorder()
	mkdirHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify directory has correct permissions (0755)
	info, err := os.Stat(filepath.Join(tmpDir, "permtest"))
	if err != nil {
		t.Fatal(err)
	}

	// On Unix, check permissions (masked by umask in practice)
	perm := info.Mode().Perm()
	if perm&0700 != 0700 {
		t.Errorf("owner should have rwx, got %o", perm)
	}
}

func TestMkdirParentIsFile(t *testing.T) {
	_, mkdirHandler, tmpDir := setupMkdirHandler(t)
	defer os.RemoveAll(tmpDir)

	// Create a file where we want the parent to be
	os.WriteFile(filepath.Join(tmpDir, "notadir"), []byte("content"), 0644)

	// Try to create subdirectory under the file
	req := httptest.NewRequest(http.MethodPost, "/mkdir/notadir/subdir/", nil)
	rr := httptest.NewRecorder()
	mkdirHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
