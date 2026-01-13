package handlers_test

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

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/handlers"
	"files-browser-backend/internal/pathutil"
)

// setupTestHandler creates a test configuration and handlers with a temporary base directory.
func setupTestHandler(t *testing.T) (config.Config, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "upload-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       tmpDir,
		MaxUploadSize: 10 * 1024 * 1024, // 10MB for tests
	}

	return cfg, tmpDir
}

func TestPathResolution(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
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

			_, err := pathutil.ResolveTargetDir(cfg.BaseDir, tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				var pathErr *pathutil.PathError
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
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	uploadHandler := handlers.NewUploadHandler(cfg)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("hello world"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/docs/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	uploadHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp handlers.UploadResponse
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
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	uploadHandler := handlers.NewUploadHandler(cfg)

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

	req := httptest.NewRequest(http.MethodPost, "/api/upload/batch/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	uploadHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp handlers.UploadResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Uploaded) != 3 {
		t.Errorf("expected 3 uploaded files, got %d", len(resp.Uploaded))
	}
}

func TestRejectOverwrite(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	uploadHandler := handlers.NewUploadHandler(cfg)

	// Create existing file
	os.MkdirAll(filepath.Join(tmpDir, "existing"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "existing", "file.txt"), []byte("original"), 0644)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "file.txt")
	part.Write([]byte("new content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/existing/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	uploadHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", rr.Code)
	}

	var resp handlers.UploadResponse
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
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	uploadHandler := handlers.NewUploadHandler(cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	// Create form file with empty name
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename=""`)
	h.Set("Content-Type", "application/octet-stream")
	part, _ := writer.CreatePart(h)
	part.Write([]byte("content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/test/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	uploadHandler.ServeHTTP(rr, req)

	var resp handlers.UploadResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Uploaded) > 0 {
		t.Error("empty filename should be rejected")
	}
}

func TestRejectHiddenFiles(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	uploadHandler := handlers.NewUploadHandler(cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", ".htaccess")
	part.Write([]byte("malicious content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/test/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	uploadHandler.ServeHTTP(rr, req)

	var resp handlers.UploadResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Uploaded) > 0 {
		t.Error("hidden files should be rejected")
	}
	if len(resp.Errors) == 0 {
		t.Error("expected error for hidden file")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	uploadHandler := handlers.NewUploadHandler(cfg)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/api/upload/test/", nil)
		rr := httptest.NewRecorder()
		uploadHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405, got %d", method, rr.Code)
		}
	}
}

func TestInvalidContentType(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	uploadHandler := handlers.NewUploadHandler(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/upload/test/", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	uploadHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPartialSuccess(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer os.RemoveAll(tmpDir)

	uploadHandler := handlers.NewUploadHandler(cfg)

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

	req := httptest.NewRequest(http.MethodPost, "/api/upload/partial/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	uploadHandler.ServeHTTP(rr, req)

	// Should return 201 because at least one file was uploaded
	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp handlers.UploadResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Uploaded) != 1 || resp.Uploaded[0] != "new.txt" {
		t.Errorf("unexpected uploaded: %v", resp.Uploaded)
	}
	if len(resp.Skipped) != 1 || resp.Skipped[0] != "existing.txt" {
		t.Errorf("unexpected skipped: %v", resp.Skipped)
	}
}
