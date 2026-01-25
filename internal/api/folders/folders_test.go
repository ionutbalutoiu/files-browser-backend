package folders_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"files-browser-backend/internal/api/folders"
	"files-browser-backend/internal/config"
)

// FolderTestResponse matches the JSON response structure
type FolderTestResponse struct {
	Created string `json:"created"`
}

// setupTestHandler creates a test configuration with a temporary base directory.
func setupTestHandler(t *testing.T) (config.Config, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "folders-test-*")
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

func TestFoldersCreateSimple(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	body, _ := json.Marshal(folders.CreateRequest{Path: "newdir"})

	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp FolderTestResponse
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

func TestFoldersCreateNested(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	// Create parent directory first
	_ = os.MkdirAll(filepath.Join(tmpDir, "photos/2026"), 0755)

	body, _ := json.Marshal(folders.CreateRequest{Path: "photos/2026/vacation"})

	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp FolderTestResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)

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

func TestFoldersCreateParentNotExist(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	body, _ := json.Marshal(folders.CreateRequest{Path: "nonexistent/subdir"})

	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFoldersCreateAlreadyExists(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	// Create directory first
	_ = os.MkdirAll(filepath.Join(tmpDir, "existing"), 0755)

	body, _ := json.Marshal(folders.CreateRequest{Path: "existing"})

	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFoldersCreateFileExists(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	// Create a file with the target name
	_ = os.WriteFile(filepath.Join(tmpDir, "myfile"), []byte("content"), 0644)

	body, _ := json.Marshal(folders.CreateRequest{Path: "myfile"})

	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFoldersCreateEmptyPath(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	body, _ := json.Marshal(folders.CreateRequest{Path: ""})

	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFoldersCreatePathTraversal(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	tests := []struct {
		name string
		path string
	}{
		{"double dot", "../escape"},
		{"hidden traversal", "foo/../../escape"},
		{"triple dot escape", "foo/../../../escape"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(folders.CreateRequest{Path: tt.path})

			req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden {
				t.Errorf("expected 400 or 403, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestFoldersCreateAbsolutePath(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	body, _ := json.Marshal(folders.CreateRequest{Path: "/etc/passwd"})

	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden && rr.Code != http.StatusNotFound {
		t.Errorf("expected 400, 403, or 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFoldersCreateSymlinkParent(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	// Create a directory outside base (simulated with another dir in temp)
	outsideDir, err := os.MkdirTemp("", "outside-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(outsideDir) }()

	// Create a symlink inside base pointing to outside
	symlinkPath := filepath.Join(tmpDir, "escape-link")
	_ = os.Symlink(outsideDir, symlinkPath)

	body, _ := json.Marshal(folders.CreateRequest{Path: "escape-link/newdir"})

	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for symlink escape, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify no directory was created outside
	if _, err := os.Stat(filepath.Join(outsideDir, "newdir")); !os.IsNotExist(err) {
		t.Error("directory should not have been created outside base")
	}
}

func TestFoldersCreateSymlinkTarget(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	// Create a real directory
	_ = os.MkdirAll(filepath.Join(tmpDir, "realdir"), 0755)

	// Create a symlink
	symlinkPath := filepath.Join(tmpDir, "link")
	_ = os.Symlink(filepath.Join(tmpDir, "realdir"), symlinkPath)

	body, _ := json.Marshal(folders.CreateRequest{Path: "link"})

	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should fail because path already exists as symlink
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFoldersCreateDirectoryWithSpaces(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	body, _ := json.Marshal(folders.CreateRequest{Path: "my folder"})

	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

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

func TestFoldersCreateParentIsFile(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	// Create a file where we want the parent to be
	_ = os.WriteFile(filepath.Join(tmpDir, "notadir"), []byte("content"), 0644)

	body, _ := json.Marshal(folders.CreateRequest{Path: "notadir/subdir"})

	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFoldersCreateInvalidJSON(t *testing.T) {
	cfg, tmpDir := setupTestHandler(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	handler := folders.NewCreateHandler(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
