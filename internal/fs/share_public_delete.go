package fs

import (
	"os"
	"path/filepath"
	"strings"

	"files-browser-backend/internal/pathutil"
)

// DeletePublicShare deletes a public share symlink and cleans up empty parent directories.
// relPath is the relative path within publicBaseDir to the symlink.
//
// SECURITY:
// - Validates relPath is safe (no path traversal, no absolute paths)
// - Verifies the resolved path stays within publicBaseDir
// - Only deletes symlinks (not regular files or directories)
// - Never removes publicBaseDir itself during cleanup
func DeletePublicShare(publicBaseDir, relPath string) error {
	// Validate publicBaseDir is set
	if publicBaseDir == "" {
		return &pathutil.PathError{
			StatusCode: 400,
			Message:    "public-base-dir is not configured",
		}
	}

	// Reject empty path
	if relPath == "" {
		return &pathutil.PathError{
			StatusCode: 400,
			Message:    "path is required",
		}
	}

	// Clean the path to normalize . and .. components
	cleanPath := filepath.Clean(relPath)

	// Reject paths containing .. after cleaning
	if strings.Contains(cleanPath, "..") {
		return &pathutil.PathError{
			StatusCode: 400,
			Message:    "invalid path: contains parent directory reference",
		}
	}

	// Reject absolute paths
	if filepath.IsAbs(cleanPath) {
		return &pathutil.PathError{
			StatusCode: 400,
			Message:    "invalid path: absolute paths not allowed",
		}
	}

	// Reject paths that are just "." after cleaning
	if cleanPath == "." {
		return &pathutil.PathError{
			StatusCode: 400,
			Message:    "invalid path: cannot delete base directory",
		}
	}

	// Compute absolute path to the symlink
	cleanPublicBaseDir := filepath.Clean(publicBaseDir)
	linkAbs := filepath.Join(cleanPublicBaseDir, cleanPath)

	// CRITICAL: Verify the link path stays within publicBaseDir
	relLink, err := filepath.Rel(cleanPublicBaseDir, linkAbs)
	if err != nil || strings.HasPrefix(relLink, "..") || relLink == "." {
		return &pathutil.PathError{
			StatusCode: 400,
			Message:    "invalid path: escapes public base directory",
		}
	}

	// Use Lstat to get info without following symlinks
	info, err := os.Lstat(linkAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return &pathutil.PathError{
				StatusCode: 404,
				Message:    "path does not exist",
			}
		}
		return &pathutil.PathError{
			StatusCode: 500,
			Message:    "failed to stat path",
		}
	}

	// Only allow deleting symlinks
	if info.Mode()&os.ModeSymlink == 0 {
		if info.IsDir() {
			return &pathutil.PathError{
				StatusCode: 400,
				Message:    "path is a directory, not a symlink",
			}
		}
		return &pathutil.PathError{
			StatusCode: 400,
			Message:    "path is not a symlink",
		}
	}

	// Delete the symlink (os.Remove removes the link, not the target)
	if err := os.Remove(linkAbs); err != nil {
		if os.IsNotExist(err) {
			return &pathutil.PathError{
				StatusCode: 404,
				Message:    "path does not exist",
			}
		}
		if os.IsPermission(err) {
			return &pathutil.PathError{
				StatusCode: 403,
				Message:    "permission denied",
			}
		}
		return &pathutil.PathError{
			StatusCode: 500,
			Message:    "failed to delete symlink",
		}
	}

	// Clean up empty parent directories (best-effort, don't fail if cleanup fails)
	cleanupEmptyParents(linkAbs, cleanPublicBaseDir)

	return nil
}

// cleanupEmptyParents removes empty parent directories starting from the parent of
// deletedPath up to (but NOT including) stopAt.
// This is best-effort: errors are ignored since the main operation (symlink deletion)
// already succeeded.
func cleanupEmptyParents(deletedPath, stopAt string) {
	dir := filepath.Dir(deletedPath)
	stopAt = filepath.Clean(stopAt)

	for {
		// Clean to ensure consistent comparison
		dir = filepath.Clean(dir)

		// Stop if we've reached or would delete stopAt
		if dir == stopAt {
			return
		}

		// Safety: also stop if dir is somehow a prefix or equal to stopAt
		// or if we've gone above it
		rel, err := filepath.Rel(stopAt, dir)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return
		}

		// Check if directory is empty
		isEmpty, err := isDirEmpty(dir)
		if err != nil || !isEmpty {
			// Stop if error or directory is not empty
			return
		}

		// Remove the empty directory
		if err := os.Remove(dir); err != nil {
			// Stop on any error (permission, not exists, etc.)
			return
		}

		// Move to parent
		dir = filepath.Dir(dir)
	}
}

// isDirEmpty checks if a directory is empty.
func isDirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Read at most 1 entry
	names, err := f.Readdirnames(1)
	if err != nil && err.Error() != "EOF" {
		return false, err
	}

	return len(names) == 0, nil
}
