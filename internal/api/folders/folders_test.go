// Package folders_test provides tests for the folders API handlers.
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

// testResponse matches the JSON response structure for folder creation.
type testResponse struct {
	Created string `json:"created"`
	Error   string `json:"error"`
}

// testEnv holds the test environment configuration.
type testEnv struct {
	handler *folders.CreateHandler
	baseDir string
}

// setupTest creates a test environment with a temporary base directory.
func setupTest(t *testing.T) testEnv {
	t.Helper()
	baseDir := t.TempDir()
	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       baseDir,
		MaxUploadSize: 10 * 1024 * 1024,
	}
	return testEnv{
		handler: folders.NewCreateHandler(cfg),
		baseDir: baseDir,
	}
}

// doRequest executes a folder creation request and returns the response.
func (e testEnv) doRequest(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(folders.CreateRequest{Path: path})
	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, req)
	return rr
}

// doRawRequest executes a request with raw body content.
func (e testEnv) doRawRequest(t *testing.T, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/folders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, req)
	return rr
}

// decodeResponse parses the JSON response body.
func decodeResponse(t *testing.T, rr *httptest.ResponseRecorder) testResponse {
	t.Helper()
	var resp testResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// assertDirExists verifies a directory exists at the given path.
func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("path should be a directory")
	}
}

// assertDirNotExists verifies no directory exists at the given path.
func assertDirNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("directory should not exist")
	}
}

func TestCreateSimple(t *testing.T) {
	env := setupTest(t)

	rr := env.doRequest(t, "newdir")

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeResponse(t, rr)
	if resp.Created != "newdir/" {
		t.Errorf("expected created=newdir/, got %s", resp.Created)
	}

	assertDirExists(t, filepath.Join(env.baseDir, "newdir"))
}

func TestCreateNested(t *testing.T) {
	env := setupTest(t)
	_ = os.MkdirAll(filepath.Join(env.baseDir, "photos/2026"), 0755)

	rr := env.doRequest(t, "photos/2026/vacation")

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeResponse(t, rr)
	if resp.Created != "photos/2026/vacation/" {
		t.Errorf("expected created=photos/2026/vacation/, got %s", resp.Created)
	}

	assertDirExists(t, filepath.Join(env.baseDir, "photos/2026/vacation"))
}

func TestCreateParentNotExist(t *testing.T) {
	env := setupTest(t)

	rr := env.doRequest(t, "nonexistent/subdir")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateAlreadyExists(t *testing.T) {
	env := setupTest(t)
	_ = os.MkdirAll(filepath.Join(env.baseDir, "existing"), 0755)

	rr := env.doRequest(t, "existing")

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateFileExists(t *testing.T) {
	env := setupTest(t)
	_ = os.WriteFile(filepath.Join(env.baseDir, "myfile"), []byte("content"), 0644)

	rr := env.doRequest(t, "myfile")

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateEmptyPath(t *testing.T) {
	env := setupTest(t)

	rr := env.doRequest(t, "")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreatePathTraversal(t *testing.T) {
	env := setupTest(t)

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
			rr := env.doRequest(t, tt.path)

			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden {
				t.Errorf("expected 400 or 403, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestCreateAbsolutePath(t *testing.T) {
	env := setupTest(t)

	rr := env.doRequest(t, "/etc/passwd")

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden && rr.Code != http.StatusNotFound {
		t.Errorf("expected 400, 403, or 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateSymlinkParent(t *testing.T) {
	env := setupTest(t)

	// Create a directory outside base.
	outsideDir := t.TempDir()

	// Create a symlink inside base pointing to outside.
	symlinkPath := filepath.Join(env.baseDir, "escape-link")
	_ = os.Symlink(outsideDir, symlinkPath)

	rr := env.doRequest(t, "escape-link/newdir")

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for symlink escape, got %d: %s", rr.Code, rr.Body.String())
	}

	assertDirNotExists(t, filepath.Join(outsideDir, "newdir"))
}

func TestCreateSymlinkTarget(t *testing.T) {
	env := setupTest(t)

	// Create a real directory and a symlink to it.
	_ = os.MkdirAll(filepath.Join(env.baseDir, "realdir"), 0755)
	_ = os.Symlink(filepath.Join(env.baseDir, "realdir"), filepath.Join(env.baseDir, "link"))

	rr := env.doRequest(t, "link")

	// Should fail because path already exists as symlink.
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateDirectoryWithSpaces(t *testing.T) {
	env := setupTest(t)

	rr := env.doRequest(t, "my folder")

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	assertDirExists(t, filepath.Join(env.baseDir, "my folder"))
}

func TestCreateParentIsFile(t *testing.T) {
	env := setupTest(t)
	_ = os.WriteFile(filepath.Join(env.baseDir, "notadir"), []byte("content"), 0644)

	rr := env.doRequest(t, "notadir/subdir")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateInvalidJSON(t *testing.T) {
	env := setupTest(t)

	rr := env.doRawRequest(t, []byte("invalid json"))

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
