// Package publicshares_test provides tests for the public shares API handlers.
package publicshares_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"files-browser-backend/internal/api/publicshares"
	"files-browser-backend/internal/config"
)

// testEnv holds the test environment configuration.
type testEnv struct {
	createHandler *publicshares.CreateHandler
	deleteHandler *publicshares.DeleteHandler
	listHandler   *publicshares.ListHandler
	baseDir       string
	publicDir     string
}

// setupTest creates a test environment with temporary directories.
func setupTest(t *testing.T) testEnv {
	t.Helper()
	baseDir := t.TempDir()
	publicDir := t.TempDir()
	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       baseDir,
		PublicBaseDir: publicDir,
		MaxUploadSize: 10 * 1024 * 1024,
	}
	return testEnv{
		createHandler: publicshares.NewCreateHandler(cfg),
		deleteHandler: publicshares.NewDeleteHandler(cfg),
		listHandler:   publicshares.NewListHandler(cfg),
		baseDir:       baseDir,
		publicDir:     publicDir,
	}
}

// setupTestDisabled creates a test environment without public sharing enabled.
func setupTestDisabled(t *testing.T) testEnv {
	t.Helper()
	baseDir := t.TempDir()
	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       baseDir,
		PublicBaseDir: "", // Not configured
		MaxUploadSize: 10 * 1024 * 1024,
	}
	return testEnv{
		createHandler: publicshares.NewCreateHandler(cfg),
		deleteHandler: publicshares.NewDeleteHandler(cfg),
		listHandler:   publicshares.NewListHandler(cfg),
		baseDir:       baseDir,
		publicDir:     "",
	}
}

// doCreate executes a create public share request.
func (e testEnv) doCreate(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(publicshares.CreateRequest{Path: path})
	return e.doCreateRaw(t, body)
}

// doCreateRaw executes a create request with raw body content.
func (e testEnv) doCreateRaw(t *testing.T, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/public-shares", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	e.createHandler.ServeHTTP(rr, req)
	return rr
}

// doDelete executes a delete public share request.
func (e testEnv) doDelete(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/public-shares?path="+path, nil)
	rr := httptest.NewRecorder()
	e.deleteHandler.ServeHTTP(rr, req)
	return rr
}

// doList executes a list public shares request.
func (e testEnv) doList(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/public-shares", nil)
	rr := httptest.NewRecorder()
	e.listHandler.ServeHTTP(rr, req)
	return rr
}

// decodeCreateResponse parses the JSON response for create requests.
func decodeCreateResponse(t *testing.T, rr *httptest.ResponseRecorder) publicshares.CreateResponse {
	t.Helper()
	var resp publicshares.CreateResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	return resp
}

// decodeListResponse parses the JSON response for list requests.
func decodeListResponse(t *testing.T, rr *httptest.ResponseRecorder) []string {
	t.Helper()
	var files []string
	if err := json.NewDecoder(rr.Body).Decode(&files); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	return files
}

// decodeErrorResponse parses the JSON error response.
func decodeErrorResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	return resp
}

// assertSymlinkExists verifies a symlink exists at the given path.
func assertSymlinkExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("symlink should exist: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file")
	}
}

// assertSymlinkNotExists verifies no symlink exists at the given path.
func assertSymlinkNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Error("symlink should not exist")
	}
}

// ============================================================================
// POST /api/public-shares (Create)
// ============================================================================

func TestCreateFile(t *testing.T) {
	env := setupTest(t)

	// Create a file to share.
	_ = os.MkdirAll(filepath.Join(env.baseDir, "photos", "2026"), 0755)
	testFile := filepath.Join(env.baseDir, "photos", "2026", "pic.jpg")
	_ = os.WriteFile(testFile, []byte("image data"), 0644)

	rr := env.doCreate(t, "photos/2026/pic.jpg")

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeCreateResponse(t, rr)
	if resp.Path != "photos/2026/pic.jpg" {
		t.Errorf("expected path='photos/2026/pic.jpg', got '%s'", resp.Path)
	}
	if resp.ShareID == "" {
		t.Error("expected shareId to be set")
	}

	linkPath := filepath.Join(env.publicDir, "photos", "2026", "pic.jpg")
	assertSymlinkExists(t, linkPath)
}

func TestCreateIdempotent(t *testing.T) {
	env := setupTest(t)

	// Create a file.
	testFile := filepath.Join(env.baseDir, "doc.txt")
	_ = os.WriteFile(testFile, []byte("content"), 0644)

	// Share it twice.
	rr1 := env.doCreate(t, "doc.txt")
	if rr1.Code != http.StatusCreated {
		t.Errorf("first share: expected 201, got %d", rr1.Code)
	}

	rr2 := env.doCreate(t, "doc.txt")
	if rr2.Code != http.StatusCreated {
		t.Errorf("second share: expected 201 (idempotent), got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestCreateDirectory(t *testing.T) {
	env := setupTest(t)

	// Create a directory.
	_ = os.MkdirAll(filepath.Join(env.baseDir, "mydir"), 0755)

	rr := env.doCreate(t, "mydir")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for directory, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateNonExistent(t *testing.T) {
	env := setupTest(t)

	rr := env.doCreate(t, "does-not-exist.txt")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestCreatePathTraversal(t *testing.T) {
	env := setupTest(t)

	tests := []struct {
		name string
		path string
	}{
		{"double dot", "../etc/passwd"},
		{"hidden traversal", "foo/../../etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := env.doCreate(t, tt.path)

			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
				t.Errorf("expected 400 or 404, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestCreateSymlink(t *testing.T) {
	env := setupTest(t)

	// Create a regular file and a symlink to it.
	realFile := filepath.Join(env.baseDir, "real.txt")
	_ = os.WriteFile(realFile, []byte("content"), 0644)
	symlinkPath := filepath.Join(env.baseDir, "link.txt")
	_ = os.Symlink(realFile, symlinkPath)

	rr := env.doCreate(t, "link.txt")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for symlink, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateNotEnabled(t *testing.T) {
	env := setupTestDisabled(t)

	// Create a file.
	testFile := filepath.Join(env.baseDir, "file.txt")
	_ = os.WriteFile(testFile, []byte("content"), 0644)

	rr := env.doCreate(t, "file.txt")

	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateEmptyPath(t *testing.T) {
	env := setupTest(t)

	rr := env.doCreate(t, "")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateInvalidJSON(t *testing.T) {
	env := setupTest(t)

	rr := env.doCreateRaw(t, []byte("invalid json"))

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ============================================================================
// DELETE /api/public-shares?path=...
// ============================================================================

func TestDeleteSuccess(t *testing.T) {
	env := setupTest(t)

	// Create a target file and symlink in public dir.
	targetFile := filepath.Join(env.baseDir, "shared-file.txt")
	_ = os.WriteFile(targetFile, []byte("content"), 0644)

	nestedDir := filepath.Join(env.publicDir, "photos", "2026")
	_ = os.MkdirAll(nestedDir, 0755)

	linkPath := filepath.Join(nestedDir, "shared-file.txt")
	_ = os.Symlink(targetFile, linkPath)

	rr := env.doDelete(t, "photos/2026/shared-file.txt")

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}

	assertSymlinkNotExists(t, linkPath)
}

func TestDeleteNotFound(t *testing.T) {
	env := setupTest(t)

	rr := env.doDelete(t, "nonexistent.txt")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteNotASymlink(t *testing.T) {
	env := setupTest(t)

	// Create a regular file in public dir (not a symlink).
	regularFile := filepath.Join(env.publicDir, "regular.txt")
	_ = os.WriteFile(regularFile, []byte("content"), 0644)

	rr := env.doDelete(t, "regular.txt")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	// File should NOT be deleted.
	if _, err := os.Stat(regularFile); err != nil {
		t.Error("regular file should NOT be deleted")
	}
}

func TestDeletePathTraversal(t *testing.T) {
	env := setupTest(t)

	rr := env.doDelete(t, "../etc/passwd")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteEmptyPath(t *testing.T) {
	env := setupTest(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/public-shares", nil)
	rr := httptest.NewRecorder()
	env.deleteHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteNotEnabled(t *testing.T) {
	env := setupTestDisabled(t)

	rr := env.doDelete(t, "some/file.txt")

	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ============================================================================
// GET /api/public-shares (List)
// ============================================================================

func TestListEmpty(t *testing.T) {
	env := setupTest(t)

	rr := env.doList(t)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	files := decodeListResponse(t, rr)
	if len(files) != 0 {
		t.Errorf("expected empty array, got %v", files)
	}
}

func TestListWithSymlinks(t *testing.T) {
	env := setupTest(t)

	// Create real files in baseDir.
	_ = os.MkdirAll(filepath.Join(env.baseDir, "photos", "2026"), 0755)
	realFile1 := filepath.Join(env.baseDir, "photos", "2026", "pic.jpg")
	_ = os.WriteFile(realFile1, []byte("image"), 0644)

	realFile2 := filepath.Join(env.baseDir, "doc.txt")
	_ = os.WriteFile(realFile2, []byte("text"), 0644)

	// Create symlinks in publicDir mirroring the structure.
	_ = os.MkdirAll(filepath.Join(env.publicDir, "photos", "2026"), 0755)
	_ = os.Symlink(realFile1, filepath.Join(env.publicDir, "photos", "2026", "pic.jpg"))
	_ = os.Symlink(realFile2, filepath.Join(env.publicDir, "doc.txt"))

	rr := env.doList(t)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	files := decodeListResponse(t, rr)

	// Should be sorted: doc.txt before photos/2026/pic.jpg.
	expected := []string{"doc.txt", "photos/2026/pic.jpg"}
	if len(files) != len(expected) {
		t.Fatalf("expected %d files, got %d: %v", len(expected), len(files), files)
	}
	for i, f := range expected {
		if files[i] != f {
			t.Errorf("expected files[%d]=%s, got %s", i, f, files[i])
		}
	}
}

func TestListBrokenSymlink(t *testing.T) {
	env := setupTest(t)

	// Create a broken symlink.
	brokenLink := filepath.Join(env.publicDir, "broken.txt")
	_ = os.Symlink("/nonexistent/file.txt", brokenLink)

	// Create a valid regular file.
	validFile := filepath.Join(env.publicDir, "valid.txt")
	_ = os.WriteFile(validFile, []byte("content"), 0644)

	rr := env.doList(t)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	files := decodeListResponse(t, rr)

	// Only valid.txt should be included, not broken.txt.
	if len(files) != 1 || files[0] != "valid.txt" {
		t.Errorf("expected ['valid.txt'], got %v", files)
	}
}

func TestListExcludesDirectories(t *testing.T) {
	env := setupTest(t)

	// Create a directory (should be excluded from list).
	_ = os.MkdirAll(filepath.Join(env.publicDir, "subdir"), 0755)

	// Create a file inside the directory.
	_ = os.WriteFile(filepath.Join(env.publicDir, "subdir", "file.txt"), []byte("content"), 0644)

	rr := env.doList(t)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	files := decodeListResponse(t, rr)

	// Only the file should be listed, not the directory.
	if len(files) != 1 || files[0] != "subdir/file.txt" {
		t.Errorf("expected ['subdir/file.txt'], got %v", files)
	}
}

func TestListSymlinkToDirectory(t *testing.T) {
	env := setupTest(t)

	// Create a directory in baseDir.
	targetDir := filepath.Join(env.baseDir, "mydir")
	_ = os.MkdirAll(targetDir, 0755)

	// Create a symlink to the directory in publicDir (should be excluded).
	_ = os.Symlink(targetDir, filepath.Join(env.publicDir, "link-to-dir"))

	// Create a valid file symlink.
	realFile := filepath.Join(env.baseDir, "file.txt")
	_ = os.WriteFile(realFile, []byte("content"), 0644)
	_ = os.Symlink(realFile, filepath.Join(env.publicDir, "file.txt"))

	rr := env.doList(t)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	files := decodeListResponse(t, rr)

	// Only file.txt should be listed, not link-to-dir.
	if len(files) != 1 || files[0] != "file.txt" {
		t.Errorf("expected ['file.txt'], got %v", files)
	}
}

func TestListNotEnabled(t *testing.T) {
	env := setupTestDisabled(t)

	rr := env.doList(t)

	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeErrorResponse(t, rr)
	if resp["error"] != "public sharing is not enabled (public-base-dir not configured)" {
		t.Errorf("unexpected error message: %s", resp["error"])
	}
}

func TestListSorted(t *testing.T) {
	env := setupTest(t)

	// Create files in non-alphabetical order.
	_ = os.WriteFile(filepath.Join(env.publicDir, "zebra.txt"), []byte("z"), 0644)
	_ = os.WriteFile(filepath.Join(env.publicDir, "apple.txt"), []byte("a"), 0644)
	_ = os.WriteFile(filepath.Join(env.publicDir, "mango.txt"), []byte("m"), 0644)

	rr := env.doList(t)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	files := decodeListResponse(t, rr)

	expected := []string{"apple.txt", "mango.txt", "zebra.txt"}
	if len(files) != len(expected) {
		t.Fatalf("expected %d files, got %d: %v", len(expected), len(files), files)
	}
	for i, f := range expected {
		if files[i] != f {
			t.Errorf("expected files[%d]=%s, got %s", i, f, files[i])
		}
	}
}
