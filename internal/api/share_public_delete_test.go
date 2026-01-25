package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"files-browser-backend/internal/api"
	"files-browser-backend/internal/config"
)

func TestSharePublicDeleteHandler_Success(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := api.NewSharePublicDeleteHandler(cfg)

	// Create a target file and symlink in public dir
	targetFile := filepath.Join(tmpDir, "shared-file.txt")
	os.WriteFile(targetFile, []byte("content"), 0644)

	nestedDir := filepath.Join(publicDir, "photos", "2026")
	os.MkdirAll(nestedDir, 0755)

	linkPath := filepath.Join(nestedDir, "shared-file.txt")
	os.Symlink(targetFile, linkPath)

	// Create request body
	body, _ := json.Marshal(api.SharePublicDeleteRequest{
		Path: "photos/2026/shared-file.txt",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/share-public-delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Check response
	var resp api.SharePublicDeleteResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Deleted != "photos/2026/shared-file.txt" {
		t.Errorf("expected deleted='photos/2026/shared-file.txt', got '%s'", resp.Deleted)
	}

	// Verify symlink was deleted
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("symlink should be deleted")
	}

	// Verify empty directories were cleaned up
	if _, err := os.Stat(filepath.Join(publicDir, "photos", "2026")); !os.IsNotExist(err) {
		t.Error("2026 directory should be cleaned up")
	}
	if _, err := os.Stat(filepath.Join(publicDir, "photos")); !os.IsNotExist(err) {
		t.Error("photos directory should be cleaned up")
	}
}

func TestSharePublicDeleteHandler_NotFound(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := api.NewSharePublicDeleteHandler(cfg)

	body, _ := json.Marshal(api.SharePublicDeleteRequest{
		Path: "nonexistent.txt",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/share-public-delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSharePublicDeleteHandler_NotASymlink(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := api.NewSharePublicDeleteHandler(cfg)

	// Create a regular file in public dir (not a symlink)
	regularFile := filepath.Join(publicDir, "regular.txt")
	os.WriteFile(regularFile, []byte("content"), 0644)

	body, _ := json.Marshal(api.SharePublicDeleteRequest{
		Path: "regular.txt",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/share-public-delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

func TestSharePublicDeleteHandler_PathTraversal(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := api.NewSharePublicDeleteHandler(cfg)

	body, _ := json.Marshal(api.SharePublicDeleteRequest{
		Path: "../../../etc/passwd",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/share-public-delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSharePublicDeleteHandler_EmptyPath(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := api.NewSharePublicDeleteHandler(cfg)

	body, _ := json.Marshal(api.SharePublicDeleteRequest{
		Path: "",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/share-public-delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSharePublicDeleteHandler_MethodNotAllowed(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := api.NewSharePublicDeleteHandler(cfg)

	// Test with POST method (should fail)
	req := httptest.NewRequest(http.MethodPost, "/api/share-public-delete", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSharePublicDeleteHandler_PublicSharingDisabled(t *testing.T) {
	// Config without PublicBaseDir
	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       "/tmp/test",
		PublicBaseDir: "", // Empty = disabled
		MaxUploadSize: 10 * 1024 * 1024,
	}

	handler := api.NewSharePublicDeleteHandler(cfg)

	body, _ := json.Marshal(api.SharePublicDeleteRequest{
		Path: "some/file.txt",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/share-public-delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSharePublicDeleteHandler_InvalidJSON(t *testing.T) {
	cfg, tmpDir, publicDir := setupTestHandlerWithPublic(t)
	defer os.RemoveAll(tmpDir)
	defer os.RemoveAll(publicDir)

	handler := api.NewSharePublicDeleteHandler(cfg)

	req := httptest.NewRequest(http.MethodDelete, "/api/share-public-delete", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
