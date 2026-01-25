package files_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"files-browser-backend/internal/api/files"
	"files-browser-backend/internal/api/files/actions"
)

// ErrorTestResponse matches the JSON error response structure
type ErrorTestResponse struct {
	Error string `json:"error"`
}

// ============================================================================
// DELETE TESTS
// ============================================================================

func TestDeleteFile(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := files.NewDeleteHandler(cfg)

	// Create a file to delete
	_ = os.MkdirAll(filepath.Join(tmpDir, "docs"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "docs", "test.txt"), []byte("content"), 0644)

	req := httptest.NewRequest(http.MethodDelete, "/api/files?path=docs/test.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

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
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := files.NewDeleteHandler(cfg)

	// Create an empty directory
	_ = os.MkdirAll(filepath.Join(tmpDir, "empty-dir"), 0755)

	req := httptest.NewRequest(http.MethodDelete, "/api/files?path=empty-dir", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

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
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := files.NewDeleteHandler(cfg)

	// Create directory with a file
	_ = os.MkdirAll(filepath.Join(tmpDir, "non-empty"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "non-empty", "file.txt"), []byte("content"), 0644)

	req := httptest.NewRequest(http.MethodDelete, "/api/files?path=non-empty", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

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
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := files.NewDeleteHandler(cfg)

	req := httptest.NewRequest(http.MethodDelete, "/api/files?path=does-not-exist.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestDeleteMissingPathParam(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := files.NewDeleteHandler(cfg)

	req := httptest.NewRequest(http.MethodDelete, "/api/files", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDeletePathTraversal(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := files.NewDeleteHandler(cfg)

	tests := []struct {
		name string
		path string
	}{
		{"double dot", "../etc/passwd"},
		{"hidden traversal", "foo/../../etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/files?path="+tt.path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
				t.Errorf("expected 400 or 404, got %d", rr.Code)
			}
		})
	}
}

func TestDeleteSymlink(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := files.NewDeleteHandler(cfg)

	// Create a real file and a symlink to it
	realFile := filepath.Join(tmpDir, "real.txt")
	_ = os.WriteFile(realFile, []byte("content"), 0644)

	symlinkPath := filepath.Join(tmpDir, "link.txt")
	_ = os.Symlink(realFile, symlinkPath)

	req := httptest.NewRequest(http.MethodDelete, "/api/files?path=link.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

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

// ============================================================================
// MOVE TESTS (POST /api/files/move)
// ============================================================================

func TestMoveFileSuccess(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := actions.NewMoveHandler(cfg)

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

	body, _ := json.Marshal(actions.MoveRequest{
		From: "source.txt",
		To:   "destdir/moved.txt",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/move", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp actions.MoveResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
	if resp.From != "source.txt" {
		t.Errorf("unexpected source path: %s", resp.From)
	}
	if resp.To != "destdir/moved.txt" {
		t.Errorf("unexpected dest path: %s", resp.To)
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

func TestMoveRenameInPlace(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := actions.NewMoveHandler(cfg)

	// Create source file
	srcPath := filepath.Join(tmpDir, "original.txt")
	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	body, _ := json.Marshal(actions.MoveRequest{
		From: "original.txt",
		To:   "renamed.txt",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/move", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

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

func TestMoveNotFound(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := actions.NewMoveHandler(cfg)

	body, _ := json.Marshal(actions.MoveRequest{
		From: "nonexistent.txt",
		To:   "new.txt",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/move", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMoveDestinationExists(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := actions.NewMoveHandler(cfg)

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

	body, _ := json.Marshal(actions.MoveRequest{
		From: "source.txt",
		To:   "existing.txt",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/move", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

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

func TestMoveMissingFromField(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := actions.NewMoveHandler(cfg)

	body, _ := json.Marshal(actions.MoveRequest{
		From: "",
		To:   "file.txt",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/move", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMoveMissingToField(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := actions.NewMoveHandler(cfg)

	// Create a file
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	body, _ := json.Marshal(actions.MoveRequest{
		From: "file.txt",
		To:   "",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/move", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMovePathTraversal(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := actions.NewMoveHandler(cfg)

	// Create a file
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name string
		from string
		to   string
	}{
		{"traversal in from", "../etc/passwd", "new.txt"},
		{"traversal in to", "file.txt", "../outside.txt"},
		{"absolute from", "/etc/passwd", "new.txt"},
		{"absolute to", "file.txt", "/etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(actions.MoveRequest{
				From: tt.from,
				To:   tt.to,
			})

			req := httptest.NewRequest(http.MethodPost, "/api/files/move", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
				t.Errorf("expected 400 or 404, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestMoveSymlink(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := actions.NewMoveHandler(cfg)

	// Create a real file and a symlink
	realFile := filepath.Join(tmpDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create real file: %v", err)
	}

	symlinkPath := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(realFile, symlinkPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	body, _ := json.Marshal(actions.MoveRequest{
		From: "link.txt",
		To:   "moved.txt",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/move", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

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

func TestMoveInvalidJSON(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := actions.NewMoveHandler(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/files/move", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
