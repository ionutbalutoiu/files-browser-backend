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

// TestDelete consolidates all delete handler tests using table-driven approach.
func TestDelete(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		setup          func(t *testing.T, baseDir string) // setup function to create test fixtures
		expectedStatus int
		verifyAfter    func(t *testing.T, baseDir string) // verification after request
	}{
		{
			name: "delete file",
			path: "docs/test.txt",
			setup: func(t *testing.T, baseDir string) {
				_ = os.MkdirAll(filepath.Join(baseDir, "docs"), 0755)
				_ = os.WriteFile(filepath.Join(baseDir, "docs", "test.txt"), []byte("content"), 0644)
			},
			expectedStatus: http.StatusNoContent,
			verifyAfter: func(t *testing.T, baseDir string) {
				if _, err := os.Stat(filepath.Join(baseDir, "docs", "test.txt")); !os.IsNotExist(err) {
					t.Error("file should have been deleted")
				}
			},
		},
		{
			name: "delete empty directory",
			path: "empty-dir",
			setup: func(t *testing.T, baseDir string) {
				_ = os.MkdirAll(filepath.Join(baseDir, "empty-dir"), 0755)
			},
			expectedStatus: http.StatusNoContent,
			verifyAfter: func(t *testing.T, baseDir string) {
				if _, err := os.Stat(filepath.Join(baseDir, "empty-dir")); !os.IsNotExist(err) {
					t.Error("directory should have been deleted")
				}
			},
		},
		{
			name: "delete non-empty directory fails",
			path: "non-empty",
			setup: func(t *testing.T, baseDir string) {
				_ = os.MkdirAll(filepath.Join(baseDir, "non-empty"), 0755)
				_ = os.WriteFile(filepath.Join(baseDir, "non-empty", "file.txt"), []byte("content"), 0644)
			},
			expectedStatus: http.StatusConflict,
			verifyAfter: func(t *testing.T, baseDir string) {
				if _, err := os.Stat(filepath.Join(baseDir, "non-empty")); err != nil {
					t.Error("directory should still exist")
				}
			},
		},
		{
			name:           "delete non-existent path",
			path:           "does-not-exist.txt",
			setup:          nil,
			expectedStatus: http.StatusNotFound,
			verifyAfter:    nil,
		},
		{
			name:           "delete missing path param",
			path:           "",
			setup:          nil,
			expectedStatus: http.StatusBadRequest,
			verifyAfter:    nil,
		},
		{
			name:           "path traversal double dot",
			path:           "../etc/passwd",
			setup:          nil,
			expectedStatus: http.StatusBadRequest,
			verifyAfter:    nil,
		},
		{
			name:           "path traversal hidden",
			path:           "foo/../../etc/passwd",
			setup:          nil,
			expectedStatus: http.StatusBadRequest,
			verifyAfter:    nil,
		},
		{
			name: "delete symlink rejected",
			path: "link.txt",
			setup: func(t *testing.T, baseDir string) {
				realFile := filepath.Join(baseDir, "real.txt")
				_ = os.WriteFile(realFile, []byte("content"), 0644)
				_ = os.Symlink(realFile, filepath.Join(baseDir, "link.txt"))
			},
			expectedStatus: http.StatusBadRequest,
			verifyAfter: func(t *testing.T, baseDir string) {
				// Both symlink and real file should still exist
				if _, err := os.Lstat(filepath.Join(baseDir, "link.txt")); err != nil {
					t.Error("symlink should still exist")
				}
				if _, err := os.Stat(filepath.Join(baseDir, "real.txt")); err != nil {
					t.Error("real file should still exist")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, tmpDir := setupTestHandler(t)
			defer func() { _ = os.RemoveAll(tmpDir) }()

			if tt.setup != nil {
				tt.setup(t, tmpDir)
			}

			handler := files.NewDeleteHandler(cfg)
			url := "/api/files"
			if tt.path != "" {
				url += "?path=" + tt.path
			}
			req := httptest.NewRequest(http.MethodDelete, url, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}

			if tt.verifyAfter != nil {
				tt.verifyAfter(t, tmpDir)
			}
		})
	}
}

// ============================================================================
// MOVE TESTS (POST /api/files/move)
// ============================================================================

// TestMove consolidates all move handler tests using table-driven approach.
func TestMove(t *testing.T) {
	tests := []struct {
		name           string
		from           string
		to             string
		body           string // raw body for invalid JSON test
		setup          func(t *testing.T, baseDir string)
		expectedStatus int
		verifyAfter    func(t *testing.T, baseDir string)
	}{
		{
			name: "move file to directory",
			from: "source.txt",
			to:   "destdir/moved.txt",
			setup: func(t *testing.T, baseDir string) {
				_ = os.WriteFile(filepath.Join(baseDir, "source.txt"), []byte("content"), 0644)
				_ = os.Mkdir(filepath.Join(baseDir, "destdir"), 0755)
			},
			expectedStatus: http.StatusOK,
			verifyAfter: func(t *testing.T, baseDir string) {
				if _, err := os.Stat(filepath.Join(baseDir, "source.txt")); !os.IsNotExist(err) {
					t.Error("source file should not exist")
				}
				content, err := os.ReadFile(filepath.Join(baseDir, "destdir", "moved.txt"))
				if err != nil {
					t.Fatalf("failed to read moved file: %v", err)
				}
				if string(content) != "content" {
					t.Errorf("unexpected content: %s", content)
				}
			},
		},
		{
			name: "rename in place",
			from: "original.txt",
			to:   "renamed.txt",
			setup: func(t *testing.T, baseDir string) {
				_ = os.WriteFile(filepath.Join(baseDir, "original.txt"), []byte("content"), 0644)
			},
			expectedStatus: http.StatusOK,
			verifyAfter: func(t *testing.T, baseDir string) {
				if _, err := os.Stat(filepath.Join(baseDir, "original.txt")); !os.IsNotExist(err) {
					t.Error("source file should not exist")
				}
				if _, err := os.Stat(filepath.Join(baseDir, "renamed.txt")); err != nil {
					t.Errorf("destination should exist: %v", err)
				}
			},
		},
		{
			name:           "move non-existent file",
			from:           "nonexistent.txt",
			to:             "new.txt",
			setup:          nil,
			expectedStatus: http.StatusNotFound,
			verifyAfter:    nil,
		},
		{
			name: "destination exists",
			from: "source.txt",
			to:   "existing.txt",
			setup: func(t *testing.T, baseDir string) {
				_ = os.WriteFile(filepath.Join(baseDir, "source.txt"), []byte("source"), 0644)
				_ = os.WriteFile(filepath.Join(baseDir, "existing.txt"), []byte("existing"), 0644)
			},
			expectedStatus: http.StatusConflict,
			verifyAfter: func(t *testing.T, baseDir string) {
				sourceContent, _ := os.ReadFile(filepath.Join(baseDir, "source.txt"))
				if string(sourceContent) != "source" {
					t.Error("source file should be unchanged")
				}
				existingContent, _ := os.ReadFile(filepath.Join(baseDir, "existing.txt"))
				if string(existingContent) != "existing" {
					t.Error("existing file should be unchanged")
				}
			},
		},
		{
			name:           "missing from field",
			from:           "",
			to:             "file.txt",
			setup:          nil,
			expectedStatus: http.StatusBadRequest,
			verifyAfter:    nil,
		},
		{
			name: "missing to field",
			from: "file.txt",
			to:   "",
			setup: func(t *testing.T, baseDir string) {
				_ = os.WriteFile(filepath.Join(baseDir, "file.txt"), []byte("content"), 0644)
			},
			expectedStatus: http.StatusBadRequest,
			verifyAfter:    nil,
		},
		{
			name: "path traversal in from",
			from: "../etc/passwd",
			to:   "new.txt",
			setup: func(t *testing.T, baseDir string) {
				_ = os.WriteFile(filepath.Join(baseDir, "file.txt"), []byte("content"), 0644)
			},
			expectedStatus: http.StatusBadRequest,
			verifyAfter:    nil,
		},
		{
			name: "path traversal in to",
			from: "file.txt",
			to:   "../outside.txt",
			setup: func(t *testing.T, baseDir string) {
				_ = os.WriteFile(filepath.Join(baseDir, "file.txt"), []byte("content"), 0644)
			},
			expectedStatus: http.StatusBadRequest,
			verifyAfter:    nil,
		},
		{
			name: "absolute path in from",
			from: "/etc/passwd",
			to:   "new.txt",
			setup: func(t *testing.T, baseDir string) {
				_ = os.WriteFile(filepath.Join(baseDir, "file.txt"), []byte("content"), 0644)
			},
			expectedStatus: http.StatusBadRequest,
			verifyAfter:    nil,
		},
		{
			name: "absolute path in to",
			from: "file.txt",
			to:   "/etc/passwd",
			setup: func(t *testing.T, baseDir string) {
				_ = os.WriteFile(filepath.Join(baseDir, "file.txt"), []byte("content"), 0644)
			},
			expectedStatus: http.StatusBadRequest,
			verifyAfter:    nil,
		},
		{
			name: "move symlink rejected",
			from: "link.txt",
			to:   "moved.txt",
			setup: func(t *testing.T, baseDir string) {
				realFile := filepath.Join(baseDir, "real.txt")
				_ = os.WriteFile(realFile, []byte("content"), 0644)
				_ = os.Symlink(realFile, filepath.Join(baseDir, "link.txt"))
			},
			expectedStatus: http.StatusBadRequest,
			verifyAfter: func(t *testing.T, baseDir string) {
				if _, err := os.Lstat(filepath.Join(baseDir, "link.txt")); err != nil {
					t.Error("symlink should still exist")
				}
				if _, err := os.Stat(filepath.Join(baseDir, "real.txt")); err != nil {
					t.Error("real file should still exist")
				}
			},
		},
		{
			name:           "invalid JSON",
			body:           "invalid json",
			setup:          nil,
			expectedStatus: http.StatusBadRequest,
			verifyAfter:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, tmpDir := setupTestHandler(t)
			defer func() { _ = os.RemoveAll(tmpDir) }()

			if tt.setup != nil {
				tt.setup(t, tmpDir)
			}

			handler := actions.NewMoveHandler(cfg)

			var body []byte
			if tt.body != "" {
				body = []byte(tt.body)
			} else {
				body, _ = json.Marshal(actions.MoveRequest{
					From: tt.from,
					To:   tt.to,
				})
			}

			req := httptest.NewRequest(http.MethodPost, "/api/files/move", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}

			if tt.verifyAfter != nil {
				tt.verifyAfter(t, tmpDir)
			}
		})
	}
}
