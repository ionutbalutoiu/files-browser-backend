package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"files-browser-backend/internal/pathutil"
	"files-browser-backend/internal/service"
)

func TestDeletePublicShare_SuccessWithNestedCleanup(t *testing.T) {
	// Create temp public base dir
	publicDir := t.TempDir()

	// Create target file (outside public dir, simulates real shared file)
	targetDir := t.TempDir()
	targetFile := filepath.Join(targetDir, "real-file.txt")
	if err := os.WriteFile(targetFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Create nested directory structure in public dir
	// publicDir/dir1/dir2/dir3/my-file.txt -> targetFile
	nestedDir := filepath.Join(publicDir, "dir1", "dir2", "dir3")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested dirs: %v", err)
	}

	linkPath := filepath.Join(nestedDir, "my-file.txt")
	if err := os.Symlink(targetFile, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Verify symlink exists
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("symlink should exist before delete: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected symlink")
	}

	// Delete the public share
	err = service.DeletePublicShare(context.Background(), publicDir, "dir1/dir2/dir3/my-file.txt")
	if err != nil {
		t.Fatalf("DeletePublicShare failed: %v", err)
	}

	// Verify symlink is removed
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("symlink should be removed")
	}

	// Verify all empty parent directories are removed
	if _, err := os.Stat(filepath.Join(publicDir, "dir1", "dir2", "dir3")); !os.IsNotExist(err) {
		t.Error("dir3 should be removed (empty)")
	}
	if _, err := os.Stat(filepath.Join(publicDir, "dir1", "dir2")); !os.IsNotExist(err) {
		t.Error("dir2 should be removed (empty)")
	}
	if _, err := os.Stat(filepath.Join(publicDir, "dir1")); !os.IsNotExist(err) {
		t.Error("dir1 should be removed (empty)")
	}

	// Verify publicDir still exists
	if _, err := os.Stat(publicDir); err != nil {
		t.Error("publicDir should NOT be removed")
	}

	// Verify target file is NOT deleted (only symlink)
	if _, err := os.Stat(targetFile); err != nil {
		t.Error("target file should NOT be deleted")
	}
}

func TestDeletePublicShare_CleanupStopsAtNonEmptyDir(t *testing.T) {
	publicDir := t.TempDir()

	// Create target file
	targetDir := t.TempDir()
	targetFile := filepath.Join(targetDir, "real-file.txt")
	_ = os.WriteFile(targetFile, []byte("content"), 0644)

	// Create structure: publicDir/dir1/dir2/my-file.txt -> targetFile
	// Also create: publicDir/dir1/dir2/other-file.txt (another file to keep dir2 non-empty)
	nestedDir := filepath.Join(publicDir, "dir1", "dir2")
	_ = os.MkdirAll(nestedDir, 0755)

	linkPath := filepath.Join(nestedDir, "my-file.txt")
	_ = os.Symlink(targetFile, linkPath)

	// Create another file in dir2 to make it non-empty after symlink deletion
	otherFile := filepath.Join(nestedDir, "other-file.txt")
	_ = os.WriteFile(otherFile, []byte("other"), 0644)

	// Delete the public share
	err := service.DeletePublicShare(context.Background(), publicDir, "dir1/dir2/my-file.txt")
	if err != nil {
		t.Fatalf("DeletePublicShare failed: %v", err)
	}

	// Verify symlink is removed
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("symlink should be removed")
	}

	// dir2 should still exist (has other-file.txt)
	if _, err := os.Stat(filepath.Join(publicDir, "dir1", "dir2")); err != nil {
		t.Error("dir2 should still exist (not empty)")
	}

	// dir1 should still exist (contains dir2)
	if _, err := os.Stat(filepath.Join(publicDir, "dir1")); err != nil {
		t.Error("dir1 should still exist")
	}

	// other-file.txt should still exist
	if _, err := os.Stat(otherFile); err != nil {
		t.Error("other-file.txt should still exist")
	}
}

func TestDeletePublicShare_NotASymlink(t *testing.T) {
	publicDir := t.TempDir()

	// Create a regular file (not a symlink)
	regularFile := filepath.Join(publicDir, "regular.txt")
	_ = os.WriteFile(regularFile, []byte("content"), 0644)

	// Try to delete - should fail
	err := service.DeletePublicShare(context.Background(), publicDir, "regular.txt")
	if err == nil {
		t.Fatal("expected error when deleting regular file")
	}

	pathErr, ok := err.(*pathutil.PathError)
	if !ok {
		t.Fatalf("expected PathError, got %T", err)
	}

	if pathErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", pathErr.StatusCode)
	}

	if pathErr.Message != "path is not a symlink" {
		t.Errorf("unexpected error message: %s", pathErr.Message)
	}

	// File should NOT be deleted
	if _, err := os.Stat(regularFile); err != nil {
		t.Error("regular file should NOT be deleted")
	}
}

func TestDeletePublicShare_DirectoryNotSymlink(t *testing.T) {
	publicDir := t.TempDir()

	// Create a directory
	dir := filepath.Join(publicDir, "somedir")
	_ = os.Mkdir(dir, 0755)

	// Try to delete - should fail
	err := service.DeletePublicShare(context.Background(), publicDir, "somedir")
	if err == nil {
		t.Fatal("expected error when deleting directory")
	}

	pathErr, ok := err.(*pathutil.PathError)
	if !ok {
		t.Fatalf("expected PathError, got %T", err)
	}

	if pathErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", pathErr.StatusCode)
	}

	if pathErr.Message != "path is a directory, not a symlink" {
		t.Errorf("unexpected error message: %s", pathErr.Message)
	}

	// Directory should NOT be deleted
	if _, err := os.Stat(dir); err != nil {
		t.Error("directory should NOT be deleted")
	}
}

func TestDeletePublicShare_NotFound(t *testing.T) {
	publicDir := t.TempDir()

	err := service.DeletePublicShare(context.Background(), publicDir, "nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}

	pathErr, ok := err.(*pathutil.PathError)
	if !ok {
		t.Fatalf("expected PathError, got %T", err)
	}

	if pathErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", pathErr.StatusCode)
	}
}

func TestDeletePublicShare_PathTraversal(t *testing.T) {
	publicDir := t.TempDir()

	testCases := []struct {
		name string
		path string
	}{
		{"dot-dot", "../outside.txt"},
		{"nested-dot-dot", "dir1/../../../outside.txt"},
		{"absolute-path", "/etc/passwd"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.DeletePublicShare(context.Background(), publicDir, tc.path)
			if err == nil {
				t.Fatal("expected error for path traversal")
			}

			pathErr, ok := err.(*pathutil.PathError)
			if !ok {
				t.Fatalf("expected PathError, got %T", err)
			}

			if pathErr.StatusCode != 400 {
				t.Errorf("expected status 400, got %d", pathErr.StatusCode)
			}
		})
	}
}

func TestDeletePublicShare_EmptyPath(t *testing.T) {
	publicDir := t.TempDir()

	err := service.DeletePublicShare(context.Background(), publicDir, "")
	if err == nil {
		t.Fatal("expected error for empty path")
	}

	pathErr, ok := err.(*pathutil.PathError)
	if !ok {
		t.Fatalf("expected PathError, got %T", err)
	}

	if pathErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", pathErr.StatusCode)
	}
}

func TestDeletePublicShare_EmptyPublicBaseDir(t *testing.T) {
	err := service.DeletePublicShare(context.Background(), "", "some/path.txt")
	if err == nil {
		t.Fatal("expected error for empty public base dir")
	}

	pathErr, ok := err.(*pathutil.PathError)
	if !ok {
		t.Fatalf("expected PathError, got %T", err)
	}

	if pathErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", pathErr.StatusCode)
	}

	if pathErr.Message != "public-base-dir is not configured" {
		t.Errorf("unexpected error message: %s", pathErr.Message)
	}
}

func TestDeletePublicShare_DotPath(t *testing.T) {
	publicDir := t.TempDir()

	// Try to delete with "." path (would delete base directory)
	err := service.DeletePublicShare(context.Background(), publicDir, ".")
	if err == nil {
		t.Fatal("expected error for '.' path")
	}

	pathErr, ok := err.(*pathutil.PathError)
	if !ok {
		t.Fatalf("expected PathError, got %T", err)
	}

	if pathErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", pathErr.StatusCode)
	}
}

func TestDeletePublicShare_DeepNestedCleanup(t *testing.T) {
	publicDir := t.TempDir()

	// Create target file
	targetDir := t.TempDir()
	targetFile := filepath.Join(targetDir, "deep-file.txt")
	_ = os.WriteFile(targetFile, []byte("content"), 0644)

	// Create very deep structure
	deepPath := filepath.Join(publicDir, "a", "b", "c", "d", "e", "f")
	_ = os.MkdirAll(deepPath, 0755)

	linkPath := filepath.Join(deepPath, "file.txt")
	_ = os.Symlink(targetFile, linkPath)

	// Delete
	err := service.DeletePublicShare(context.Background(), publicDir, "a/b/c/d/e/f/file.txt")
	if err != nil {
		t.Fatalf("DeletePublicShare failed: %v", err)
	}

	// All empty directories should be removed
	for _, subdir := range []string{"a/b/c/d/e/f", "a/b/c/d/e", "a/b/c/d", "a/b/c", "a/b", "a"} {
		if _, err := os.Stat(filepath.Join(publicDir, subdir)); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", subdir)
		}
	}

	// publicDir should still exist
	if _, err := os.Stat(publicDir); err != nil {
		t.Error("publicDir should NOT be removed")
	}
}

func TestDeletePublicShare_RootLevelSymlink(t *testing.T) {
	publicDir := t.TempDir()

	// Create target file
	targetDir := t.TempDir()
	targetFile := filepath.Join(targetDir, "root-file.txt")
	_ = os.WriteFile(targetFile, []byte("content"), 0644)

	// Create symlink directly in publicDir (no nested dirs)
	linkPath := filepath.Join(publicDir, "my-file.txt")
	_ = os.Symlink(targetFile, linkPath)

	// Delete
	err := service.DeletePublicShare(context.Background(), publicDir, "my-file.txt")
	if err != nil {
		t.Fatalf("DeletePublicShare failed: %v", err)
	}

	// Symlink should be removed
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("symlink should be removed")
	}

	// publicDir should still exist
	if _, err := os.Stat(publicDir); err != nil {
		t.Error("publicDir should NOT be removed")
	}
}
