package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestServer creates a test server with a temporary base directory.
func setupTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "upload-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := Config{
		ListenAddr:    ":8080",
		BaseDir:       tmpDir,
		MaxUploadSize: 10 * 1024 * 1024, // 10MB for tests
		UploadPrefix:  "/upload",
		DeletePrefix:  "/delete",
		MkdirPrefix:   "/mkdir",
	}

	server, err := NewServer(cfg)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create server: %v", err)
	}

	return server, tmpDir
}

func TestPathResolution(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name      string
		path      string
		wantErr   bool
		errStatus int
		setupPath string // create this directory before test
	}{
		{
			name:    "simple path",
			path:    "photos/2026",
			wantErr: false,
		},
		{
			name:      "path traversal with ..",
			path:      "../etc/passwd",
			wantErr:   true,
			errStatus: http.StatusBadRequest,
		},
		{
			name:      "hidden path traversal",
			path:      "photos/../../etc",
			wantErr:   true,
			errStatus: http.StatusBadRequest,
		},
		{
			name:      "absolute path",
			path:      "/etc/passwd",
			wantErr:   true,
			errStatus: http.StatusBadRequest,
		},
		{
			name:    "empty path (root)",
			path:    "",
			wantErr: false,
		},
		{
			name:    "path with spaces",
			path:    "my photos/vacation",
			wantErr: false,
		},
		{
			name:    "deeply nested path",
			path:    "a/b/c/d/e/f",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupPath != "" {
				os.MkdirAll(filepath.Join(tmpDir, tt.setupPath), 0755)
			}

			_, err := server.resolveTargetDir(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				var pathErr *PathError
				if !errors.As(err, &pathErr) {
					t.Errorf("expected PathError, got %T", err)
					return
				}
				if pathErr.StatusCode != tt.errStatus {
					t.Errorf("expected status %d, got %d", tt.errStatus, pathErr.StatusCode)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestUploadSingleFile(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("hello world"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload/docs/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	server.HandleUpload(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp UploadResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Uploaded) != 1 || resp.Uploaded[0] != "test.txt" {
		t.Errorf("unexpected uploaded files: %v", resp.Uploaded)
	}

	// Verify file exists
	content, err := os.ReadFile(filepath.Join(tmpDir, "docs", "test.txt"))
	if err != nil {
		t.Fatalf("failed to read uploaded file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("unexpected file content: %s", content)
	}
}

func TestUploadMultipleFiles(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	files := []struct {
		name    string
		content string
	}{
		{"file1.txt", "content1"},
		{"file2.txt", "content2"},
		{"file3.txt", "content3"},
	}

	for _, f := range files {
		part, _ := writer.CreateFormFile("files", f.name)
		part.Write([]byte(f.content))
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload/batch/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	server.HandleUpload(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp UploadResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Uploaded) != 3 {
		t.Errorf("expected 3 uploaded files, got %d", len(resp.Uploaded))
	}
}

func TestRejectOverwrite(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Create existing file
	os.MkdirAll(filepath.Join(tmpDir, "existing"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "existing", "file.txt"), []byte("original"), 0644)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "file.txt")
	part.Write([]byte("new content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload/existing/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	server.HandleUpload(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", rr.Code)
	}

	var resp UploadResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Skipped) != 1 {
		t.Errorf("expected 1 skipped file, got %d", len(resp.Skipped))
	}

	// Verify original file unchanged
	content, _ := os.ReadFile(filepath.Join(tmpDir, "existing", "file.txt"))
	if string(content) != "original" {
		t.Errorf("original file was modified")
	}
}

func TestRejectEmptyFilename(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	// Create form file with empty name
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename=""`)
	h.Set("Content-Type", "application/octet-stream")
	part, _ := writer.CreatePart(h)
	part.Write([]byte("content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload/test/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	server.HandleUpload(rr, req)

	var resp UploadResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Uploaded) > 0 {
		t.Error("empty filename should be rejected")
	}
}

func TestRejectHiddenFiles(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", ".htaccess")
	part.Write([]byte("malicious content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload/test/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	server.HandleUpload(rr, req)

	var resp UploadResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Uploaded) > 0 {
		t.Error("hidden files should be rejected")
	}
	if len(resp.Errors) == 0 {
		t.Error("expected error for hidden file")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/upload/test/", nil)
		rr := httptest.NewRecorder()
		server.HandleUpload(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405, got %d", method, rr.Code)
		}
	}
}

func TestInvalidContentType(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodPost, "/upload/test/", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.HandleUpload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPartialSuccess(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Create one existing file
	os.MkdirAll(filepath.Join(tmpDir, "partial"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "partial", "existing.txt"), []byte("original"), 0644)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// New file - should succeed
	part1, _ := writer.CreateFormFile("file", "new.txt")
	part1.Write([]byte("new content"))

	// Existing file - should be skipped
	part2, _ := writer.CreateFormFile("file", "existing.txt")
	part2.Write([]byte("overwrite attempt"))

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload/partial/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	server.HandleUpload(rr, req)

	// Should return 201 because at least one file was uploaded
	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp UploadResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Uploaded) != 1 || resp.Uploaded[0] != "new.txt" {
		t.Errorf("unexpected uploaded: %v", resp.Uploaded)
	}
	if len(resp.Skipped) != 1 || resp.Skipped[0] != "existing.txt" {
		t.Errorf("unexpected skipped: %v", resp.Skipped)
	}
}

// ============================================================================
// DELETE TESTS
// ============================================================================

func TestDeleteFile(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Create a file to delete
	os.MkdirAll(filepath.Join(tmpDir, "docs"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "docs", "test.txt"), []byte("content"), 0644)

	req := httptest.NewRequest(http.MethodDelete, "/delete/docs/test.txt", nil)
	rr := httptest.NewRecorder()
	server.HandleDelete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify file is gone
	if _, err := os.Stat(filepath.Join(tmpDir, "docs", "test.txt")); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestDeleteEmptyDirectory(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Create an empty directory
	os.MkdirAll(filepath.Join(tmpDir, "empty-dir"), 0755)

	req := httptest.NewRequest(http.MethodDelete, "/delete/empty-dir/", nil)
	rr := httptest.NewRecorder()
	server.HandleDelete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify directory is gone
	if _, err := os.Stat(filepath.Join(tmpDir, "empty-dir")); !os.IsNotExist(err) {
		t.Error("directory should have been deleted")
	}
}

func TestDeleteNonEmptyDirectory(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Create directory with a file
	os.MkdirAll(filepath.Join(tmpDir, "non-empty"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "non-empty", "file.txt"), []byte("content"), 0644)

	req := httptest.NewRequest(http.MethodDelete, "/delete/non-empty/", nil)
	rr := httptest.NewRecorder()
	server.HandleDelete(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}

	// Directory should still exist
	if _, err := os.Stat(filepath.Join(tmpDir, "non-empty")); err != nil {
		t.Error("directory should still exist")
	}
}

func TestDeleteNonExistent(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodDelete, "/delete/does-not-exist.txt", nil)
	rr := httptest.NewRecorder()
	server.HandleDelete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestDeleteBaseDirectory(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Try to delete base directory with empty path
	req := httptest.NewRequest(http.MethodDelete, "/delete/", nil)
	rr := httptest.NewRecorder()
	server.HandleDelete(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDeletePathTraversal(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

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
			server.HandleDelete(rr, req)

			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
				t.Errorf("expected 400 or 404, got %d", rr.Code)
			}
		})
	}
}

func TestDeleteSymlink(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Create a real file and a symlink to it
	realFile := filepath.Join(tmpDir, "real.txt")
	os.WriteFile(realFile, []byte("content"), 0644)

	symlinkPath := filepath.Join(tmpDir, "link.txt")
	os.Symlink(realFile, symlinkPath)

	req := httptest.NewRequest(http.MethodDelete, "/delete/link.txt", nil)
	rr := httptest.NewRecorder()
	server.HandleDelete(rr, req)

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
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/delete/test.txt", nil)
		rr := httptest.NewRecorder()
		server.HandleDelete(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405, got %d", method, rr.Code)
		}
	}
}

func TestDeleteAbsolutePath(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodDelete, "/delete//etc/passwd", nil)
	rr := httptest.NewRecorder()
	server.HandleDelete(rr, req)

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Errorf("expected 400 or 404, got %d", rr.Code)
	}
}

// ============================================================================
// MKDIR TESTS
// ============================================================================

// MkdirResponse matches the JSON response structure
type MkdirTestResponse struct {
	Created string `json:"created"`
}

func TestMkdirSimple(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodPost, "/mkdir/newdir/", nil)
	rr := httptest.NewRecorder()
	server.HandleMkdir(rr, req)

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
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Create parent directory first
	os.MkdirAll(filepath.Join(tmpDir, "photos/2026"), 0755)

	req := httptest.NewRequest(http.MethodPost, "/mkdir/photos/2026/vacation/", nil)
	rr := httptest.NewRecorder()
	server.HandleMkdir(rr, req)

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
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Try to create nested directory without creating parent first
	req := httptest.NewRequest(http.MethodPost, "/mkdir/nonexistent/subdir/", nil)
	rr := httptest.NewRecorder()
	server.HandleMkdir(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMkdirAlreadyExists(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Create directory first
	os.MkdirAll(filepath.Join(tmpDir, "existing"), 0755)

	req := httptest.NewRequest(http.MethodPost, "/mkdir/existing/", nil)
	rr := httptest.NewRecorder()
	server.HandleMkdir(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMkdirFileExists(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Create a file with the target name
	os.WriteFile(filepath.Join(tmpDir, "myfile"), []byte("content"), 0644)

	req := httptest.NewRequest(http.MethodPost, "/mkdir/myfile/", nil)
	rr := httptest.NewRecorder()
	server.HandleMkdir(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMkdirBaseDirectory(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Try to create base directory (empty path)
	req := httptest.NewRequest(http.MethodPost, "/mkdir/", nil)
	rr := httptest.NewRecorder()
	server.HandleMkdir(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMkdirPathTraversal(t *testing.T) {
	server, tmpDir := setupTestServer(t)
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
			server.HandleMkdir(rr, req)

			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden {
				t.Errorf("expected 400 or 403, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestMkdirAbsolutePath(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodPost, "/mkdir//etc/passwd/", nil)
	rr := httptest.NewRecorder()
	server.HandleMkdir(rr, req)

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden && rr.Code != http.StatusNotFound {
		t.Errorf("expected 400, 403, or 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMkdirSymlinkParent(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

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
	server.HandleMkdir(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for symlink escape, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify no directory was created outside
	if _, err := os.Stat(filepath.Join(outsideDir, "newdir")); !os.IsNotExist(err) {
		t.Error("directory should not have been created outside base")
	}
}

func TestMkdirSymlinkTarget(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Create a real directory
	os.MkdirAll(filepath.Join(tmpDir, "realdir"), 0755)

	// Create a symlink
	symlinkPath := filepath.Join(tmpDir, "link")
	os.Symlink(filepath.Join(tmpDir, "realdir"), symlinkPath)

	// Try to create the symlink as a directory
	req := httptest.NewRequest(http.MethodPost, "/mkdir/link/", nil)
	rr := httptest.NewRecorder()
	server.HandleMkdir(rr, req)

	// Should fail because path already exists as symlink
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMkdirMethodNotAllowed(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/mkdir/test/", nil)
		rr := httptest.NewRecorder()
		server.HandleMkdir(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405, got %d", method, rr.Code)
		}
	}
}

func TestMkdirDirectoryWithSpaces(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodPost, "/mkdir/my%20folder/", nil)
	rr := httptest.NewRecorder()
	server.HandleMkdir(rr, req)

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

func TestMkdirNullByteRejection(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Test the resolveMkdirPath function directly since HTTP layer
	// rejects null bytes before reaching our handler
	_, _, err := server.resolveMkdirPath("test\x00evil")
	if err == nil {
		t.Error("null byte in path should be rejected")
	}

	var pathErr *PathError
	if !errors.As(err, &pathErr) {
		t.Errorf("expected PathError, got %T", err)
	}
	if pathErr.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", pathErr.StatusCode)
	}
}

func TestMkdirPermissions(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodPost, "/mkdir/permtest/", nil)
	rr := httptest.NewRecorder()
	server.HandleMkdir(rr, req)

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
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	// Create a file where we want the parent to be
	os.WriteFile(filepath.Join(tmpDir, "notadir"), []byte("content"), 0644)

	// Try to create subdirectory under the file
	req := httptest.NewRequest(http.MethodPost, "/mkdir/notadir/subdir/", nil)
	rr := httptest.NewRecorder()
	server.HandleMkdir(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
