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

func TestPublicShareFilesEmpty(t *testing.T) {
	publicDir := t.TempDir()

	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       t.TempDir(),
		PublicBaseDir: publicDir,
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := handlers.NewSharePublicFilesHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/share-public-files/", nil)
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

func TestPublicShareFilesWithSymlinks(t *testing.T) {
	baseDir := t.TempDir()
	publicDir := t.TempDir()

	// Create real files in baseDir
	os.MkdirAll(filepath.Join(baseDir, "photos", "2026"), 0755)
	realFile1 := filepath.Join(baseDir, "photos", "2026", "pic.jpg")
	os.WriteFile(realFile1, []byte("image"), 0644)

	realFile2 := filepath.Join(baseDir, "doc.txt")
	os.WriteFile(realFile2, []byte("text"), 0644)

	// Create symlinks in publicDir mirroring the structure
	os.MkdirAll(filepath.Join(publicDir, "photos", "2026"), 0755)
	os.Symlink(realFile1, filepath.Join(publicDir, "photos", "2026", "pic.jpg"))
	os.Symlink(realFile2, filepath.Join(publicDir, "doc.txt"))

	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       baseDir,
		PublicBaseDir: publicDir,
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := handlers.NewSharePublicFilesHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/share-public-files", nil)
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

func TestPublicShareFilesBrokenSymlink(t *testing.T) {
	publicDir := t.TempDir()

	// Create a broken symlink
	brokenLink := filepath.Join(publicDir, "broken.txt")
	os.Symlink("/nonexistent/file.txt", brokenLink)

	// Create a valid regular file
	validFile := filepath.Join(publicDir, "valid.txt")
	os.WriteFile(validFile, []byte("content"), 0644)

	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       t.TempDir(),
		PublicBaseDir: publicDir,
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := handlers.NewSharePublicFilesHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/share-public-files", nil)
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

func TestPublicShareFilesExcludesDirectories(t *testing.T) {
	publicDir := t.TempDir()

	// Create a directory (should be excluded from list)
	os.MkdirAll(filepath.Join(publicDir, "subdir"), 0755)

	// Create a file inside the directory
	os.WriteFile(filepath.Join(publicDir, "subdir", "file.txt"), []byte("content"), 0644)

	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       t.TempDir(),
		PublicBaseDir: publicDir,
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := handlers.NewSharePublicFilesHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/share-public-files", nil)
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

func TestPublicShareFilesSymlinkToDirectory(t *testing.T) {
	baseDir := t.TempDir()
	publicDir := t.TempDir()

	// Create a directory in baseDir
	targetDir := filepath.Join(baseDir, "mydir")
	os.MkdirAll(targetDir, 0755)

	// Create a symlink to the directory in publicDir (should be excluded)
	os.Symlink(targetDir, filepath.Join(publicDir, "link-to-dir"))

	// Create a valid file symlink
	realFile := filepath.Join(baseDir, "file.txt")
	os.WriteFile(realFile, []byte("content"), 0644)
	os.Symlink(realFile, filepath.Join(publicDir, "file.txt"))

	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       baseDir,
		PublicBaseDir: publicDir,
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := handlers.NewSharePublicFilesHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/share-public-files", nil)
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

func TestPublicShareFilesNotEnabled(t *testing.T) {
	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       t.TempDir(),
		PublicBaseDir: "", // Not configured
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := handlers.NewSharePublicFilesHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/share-public-files", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["error"] != "public sharing is not enabled (public-base-dir not configured)" {
		t.Errorf("unexpected error message: %s", resp["error"])
	}
}

func TestPublicShareFilesMethodNotAllowed(t *testing.T) {
	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       t.TempDir(),
		PublicBaseDir: t.TempDir(),
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := handlers.NewSharePublicFilesHandler(cfg)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/share-public-files", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405, got %d", rr.Code)
			}
		})
	}
}

func TestPublicShareFilesSorted(t *testing.T) {
	publicDir := t.TempDir()

	// Create files in non-alphabetical order
	os.WriteFile(filepath.Join(publicDir, "zebra.txt"), []byte("z"), 0644)
	os.WriteFile(filepath.Join(publicDir, "apple.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(publicDir, "mango.txt"), []byte("m"), 0644)

	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       t.TempDir(),
		PublicBaseDir: publicDir,
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := handlers.NewSharePublicFilesHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/share-public-files", nil)
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
