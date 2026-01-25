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

// setupTestHandlerWithPublic creates a test configuration with both base and public directories.
func setupTestHandlerWithPublic(t *testing.T) (config.Config, string, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "share-test-base-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	publicDir, err := os.MkdirTemp("", "share-test-public-*")
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to create public temp dir: %v", err)
	}

	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       tmpDir,
		PublicBaseDir: publicDir,
		MaxUploadSize: 10 * 1024 * 1024,
	}

	return cfg, tmpDir, publicDir
}

// ============================================================================
// POST /api/public-shares (Create)
// ============================================================================

func TestPublicSharesCreateFile(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewCreateHandler(cfg)

	// Create a file to share
	_ = os.MkdirAll(filepath.Join(tmpDir, "photos", "2026"), 0755)
	testFile := filepath.Join(tmpDir, "photos", "2026", "pic.jpg")
	_ = os.WriteFile(testFile, []byte("image data"), 0644)

	body, _ := json.Marshal(publicshares.CreateRequest{Path: "photos/2026/pic.jpg"})
	req := httptest.NewRequest(http.MethodPost, "/api/public-shares", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Check response
	var resp publicshares.CreateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Path != "photos/2026/pic.jpg" {
		t.Errorf("expected path='photos/2026/pic.jpg', got '%s'", resp.Path)
	}
	if resp.ShareID == "" {
		t.Error("expected shareId to be set")
	}

	// Verify symlink was created
	linkPath := filepath.Join(publicDir, "photos", "2026", "pic.jpg")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file")
	}
}

func TestPublicSharesCreateIdempotent(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewCreateHandler(cfg)

	// Create a file
	testFile := filepath.Join(tmpDir, "doc.txt")
	_ = os.WriteFile(testFile, []byte("content"), 0644)

	body, _ := json.Marshal(publicshares.CreateRequest{Path: "doc.txt"})

	// Share it twice
	req1 := httptest.NewRequest(http.MethodPost, "/api/public-shares", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusCreated {
		t.Errorf("first share: expected 201, got %d", rr1.Code)
	}

	body2, _ := json.Marshal(publicshares.CreateRequest{Path: "doc.txt"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/public-shares", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	// Should be idempotent - success again
	if rr2.Code != http.StatusCreated {
		t.Errorf("second share: expected 201 (idempotent), got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestPublicSharesCreateDirectory(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewCreateHandler(cfg)

	// Create a directory
	_ = os.MkdirAll(filepath.Join(tmpDir, "mydir"), 0755)

	body, _ := json.Marshal(publicshares.CreateRequest{Path: "mydir"})
	req := httptest.NewRequest(http.MethodPost, "/api/public-shares", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for directory, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPublicSharesCreateNonExistent(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewCreateHandler(cfg)

	body, _ := json.Marshal(publicshares.CreateRequest{Path: "does-not-exist.txt"})
	req := httptest.NewRequest(http.MethodPost, "/api/public-shares", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestPublicSharesCreatePathTraversal(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewCreateHandler(cfg)

	tests := []struct {
		name string
		path string
	}{
		{"double dot", "../etc/passwd"},
		{"hidden traversal", "foo/../../etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(publicshares.CreateRequest{Path: tt.path})
			req := httptest.NewRequest(http.MethodPost, "/api/public-shares", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
				t.Errorf("expected 400 or 404, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestPublicSharesCreateSymlink(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewCreateHandler(cfg)

	// Create a regular file and a symlink to it
	realFile := filepath.Join(tmpDir, "real.txt")
	_ = os.WriteFile(realFile, []byte("content"), 0644)
	symlinkPath := filepath.Join(tmpDir, "link.txt")
	_ = os.Symlink(realFile, symlinkPath)

	body, _ := json.Marshal(publicshares.CreateRequest{Path: "link.txt"})
	req := httptest.NewRequest(http.MethodPost, "/api/public-shares", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for symlink, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPublicSharesCreateNotEnabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "share-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Config WITHOUT PublicBaseDir
	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       tmpDir,
		PublicBaseDir: "", // Not configured
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := publicshares.NewCreateHandler(cfg)

	// Create a file
	testFile := filepath.Join(tmpDir, "file.txt")
	_ = os.WriteFile(testFile, []byte("content"), 0644)

	body, _ := json.Marshal(publicshares.CreateRequest{Path: "file.txt"})
	req := httptest.NewRequest(http.MethodPost, "/api/public-shares", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPublicSharesCreateEmptyPath(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewCreateHandler(cfg)

	body, _ := json.Marshal(publicshares.CreateRequest{Path: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/public-shares", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPublicSharesCreateInvalidJSON(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewCreateHandler(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/public-shares", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ============================================================================
// DELETE /api/public-shares?path=...
// ============================================================================

func TestPublicSharesDeleteSuccess(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewDeleteHandler(cfg)

	// Create a target file and symlink in public dir
	targetFile := filepath.Join(tmpDir, "shared-file.txt")
	_ = os.WriteFile(targetFile, []byte("content"), 0644)

	nestedDir := filepath.Join(publicDir, "photos", "2026")
	_ = os.MkdirAll(nestedDir, 0755)

	linkPath := filepath.Join(nestedDir, "shared-file.txt")
	_ = os.Symlink(targetFile, linkPath)

	req := httptest.NewRequest(http.MethodDelete, "/api/public-shares?path=photos/2026/shared-file.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify symlink was deleted
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("symlink should be deleted")
	}
}

func TestPublicSharesDeleteNotFound(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewDeleteHandler(cfg)

	req := httptest.NewRequest(http.MethodDelete, "/api/public-shares?path=nonexistent.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPublicSharesDeleteNotASymlink(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewDeleteHandler(cfg)

	// Create a regular file in public dir (not a symlink)
	regularFile := filepath.Join(publicDir, "regular.txt")
	_ = os.WriteFile(regularFile, []byte("content"), 0644)

	req := httptest.NewRequest(http.MethodDelete, "/api/public-shares?path=regular.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	// File should NOT be deleted
	if _, err := os.Stat(regularFile); err != nil {
		t.Error("regular file should NOT be deleted")
	}
}

func TestPublicSharesDeletePathTraversal(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewDeleteHandler(cfg)

	req := httptest.NewRequest(http.MethodDelete, "/api/public-shares?path=../etc/passwd", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPublicSharesDeleteEmptyPath(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewDeleteHandler(cfg)

	req := httptest.NewRequest(http.MethodDelete, "/api/public-shares", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPublicSharesDeleteNotEnabled(t *testing.T) {
	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       "/tmp/test",
		PublicBaseDir: "", // Empty = disabled
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := publicshares.NewDeleteHandler(cfg)

	req := httptest.NewRequest(http.MethodDelete, "/api/public-shares?path=some/file.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ============================================================================
// GET /api/public-shares (List)
// ============================================================================

func TestPublicSharesListEmpty(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	handler := publicshares.NewListHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/public-shares", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var files []string
	if err := json.Unmarshal(rr.Body.Bytes(), &files); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected empty array, got %v", files)
	}
}

func TestPublicSharesListWithSymlinks(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	// Create real files in baseDir
	_ = os.MkdirAll(filepath.Join(tmpDir, "photos", "2026"), 0755)
	realFile1 := filepath.Join(tmpDir, "photos", "2026", "pic.jpg")
	_ = os.WriteFile(realFile1, []byte("image"), 0644)

	realFile2 := filepath.Join(tmpDir, "doc.txt")
	_ = os.WriteFile(realFile2, []byte("text"), 0644)

	// Create symlinks in publicDir mirroring the structure
	_ = os.MkdirAll(filepath.Join(publicDir, "photos", "2026"), 0755)
	_ = os.Symlink(realFile1, filepath.Join(publicDir, "photos", "2026", "pic.jpg"))
	_ = os.Symlink(realFile2, filepath.Join(publicDir, "doc.txt"))

	handler := publicshares.NewListHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/public-shares", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var files []string
	if err := json.Unmarshal(rr.Body.Bytes(), &files); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should be sorted: doc.txt before photos/2026/pic.jpg
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

func TestPublicSharesListBrokenSymlink(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	// Create a broken symlink
	brokenLink := filepath.Join(publicDir, "broken.txt")
	_ = os.Symlink("/nonexistent/file.txt", brokenLink)

	// Create a valid regular file
	validFile := filepath.Join(publicDir, "valid.txt")
	_ = os.WriteFile(validFile, []byte("content"), 0644)

	handler := publicshares.NewListHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/public-shares", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var files []string
	if err := json.Unmarshal(rr.Body.Bytes(), &files); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Only valid.txt should be included, not broken.txt
	if len(files) != 1 || files[0] != "valid.txt" {
		t.Errorf("expected ['valid.txt'], got %v", files)
	}
}

func TestPublicSharesListExcludesDirectories(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	// Create a directory (should be excluded from list)
	_ = os.MkdirAll(filepath.Join(publicDir, "subdir"), 0755)

	// Create a file inside the directory
	_ = os.WriteFile(filepath.Join(publicDir, "subdir", "file.txt"), []byte("content"), 0644)

	handler := publicshares.NewListHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/public-shares", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var files []string
	if err := json.Unmarshal(rr.Body.Bytes(), &files); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Only the file should be listed, not the directory
	if len(files) != 1 || files[0] != "subdir/file.txt" {
		t.Errorf("expected ['subdir/file.txt'], got %v", files)
	}
}

func TestPublicSharesListSymlinkToDirectory(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	// Create a directory in baseDir
	targetDir := filepath.Join(tmpDir, "mydir")
	_ = os.MkdirAll(targetDir, 0755)

	// Create a symlink to the directory in publicDir (should be excluded)
	_ = os.Symlink(targetDir, filepath.Join(publicDir, "link-to-dir"))

	// Create a valid file symlink
	realFile := filepath.Join(tmpDir, "file.txt")
	_ = os.WriteFile(realFile, []byte("content"), 0644)
	_ = os.Symlink(realFile, filepath.Join(publicDir, "file.txt"))

	handler := publicshares.NewListHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/public-shares", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var files []string
	if err := json.Unmarshal(rr.Body.Bytes(), &files); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Only file.txt should be listed, not link-to-dir
	if len(files) != 1 || files[0] != "file.txt" {
		t.Errorf("expected ['file.txt'], got %v", files)
	}
}

func TestPublicSharesListNotEnabled(t *testing.T) {
	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       "/tmp/test",
		PublicBaseDir: "", // Not configured
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := publicshares.NewListHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/public-shares", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["error"] != "public sharing is not enabled (public-base-dir not configured)" {
		t.Errorf("unexpected error message: %s", resp["error"])
	}
}

func TestPublicSharesListSorted(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()
	defer func() { _ = os.RemoveAll(publicDir) }()

	// Create files in non-alphabetical order
	_ = os.WriteFile(filepath.Join(publicDir, "zebra.txt"), []byte("z"), 0644)
	_ = os.WriteFile(filepath.Join(publicDir, "apple.txt"), []byte("a"), 0644)
	_ = os.WriteFile(filepath.Join(publicDir, "mango.txt"), []byte("m"), 0644)

	handler := publicshares.NewListHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/public-shares", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var files []string
	if err := json.Unmarshal(rr.Body.Bytes(), &files); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

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
