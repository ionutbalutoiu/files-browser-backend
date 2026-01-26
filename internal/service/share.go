package service

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"files-browser-backend/internal/pathutil"
)

// SharePublic creates a symlink in publicBaseDir pointing to the source file.
// The symlink mirrors the same relative directory structure.
// Returns nil on success, or an error with appropriate status code.
// The context can be used for cancellation.
func SharePublic(ctx context.Context, sourceAbsPath, publicBaseDir, relPath string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("operation cancelled: %w", err)
	}

	linkPath, err := validateShareLinkPath(publicBaseDir, relPath)
	if err != nil {
		return err
	}

	if err := ensurePublicLinkDir(linkPath); err != nil {
		return err
	}

	exists, err := checkExistingLink(linkPath, sourceAbsPath)
	if err != nil {
		return err
	}
	if exists {
		// Link already exists with same target (idempotent).
		return nil
	}

	return createSymlink(sourceAbsPath, linkPath)
}

// DeletePublicShare deletes a public share symlink and cleans up empty parent directories.
// relPath is the relative path within publicBaseDir to the symlink.
//
// SECURITY:
// - Validates relPath is safe (no path traversal, no absolute paths)
// - Verifies the resolved path stays within publicBaseDir
// - Only deletes symlinks (not regular files or directories)
// - Never removes publicBaseDir itself during cleanup
//
// The context can be used for cancellation.
func DeletePublicShare(ctx context.Context, publicBaseDir, relPath string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("operation cancelled: %w", err)
	}

	linkAbs, cleanPublicBaseDir, err := validatePublicSharePath(publicBaseDir, relPath)
	if err != nil {
		return err
	}

	if err := verifySymlink(linkAbs); err != nil {
		return err
	}

	if err := removeSymlink(linkAbs); err != nil {
		return err
	}

	// Clean up empty parent directories (best-effort, don't fail if cleanup fails).
	cleanupEmptyParents(linkAbs, cleanPublicBaseDir)

	return nil
}

// ListSharePublicFiles returns a sorted list of all publicly shared files
// under publicBaseDir. It includes symlinks pointing to regular files and
// regular files directly present. Directories and broken/invalid symlinks
// are skipped.
// The context can be used for cancellation.
func ListSharePublicFiles(ctx context.Context, publicBaseDir string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("operation cancelled: %w", err)
	}
	var files []string

	err := filepath.WalkDir(publicBaseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip entries we can't access.
			return nil
		}

		// Skip the root directory itself.
		if path == publicBaseDir {
			return nil
		}

		// Skip directories (but continue walking into them).
		if d.IsDir() {
			return nil
		}

		// Use Lstat to get info without following symlinks.
		info, err := os.Lstat(path)
		if err != nil {
			return nil // Skip on error.
		}

		// Check if it's a symlink.
		if info.Mode()&os.ModeSymlink != 0 {
			// Follow the symlink to check target.
			targetInfo, err := os.Stat(path)
			if err != nil {
				// Broken symlink or inaccessible target - skip.
				return nil
			}
			if !targetInfo.Mode().IsRegular() {
				// Symlink points to non-regular file (e.g., directory) - skip.
				return nil
			}
			// Valid symlink to regular file - include it.
		} else if info.Mode().IsRegular() {
			// Regular file - include it.
		} else {
			// Something else (device, socket, etc.) - skip.
			return nil
		}

		// Compute relative path.
		relPath, err := filepath.Rel(publicBaseDir, path)
		if err != nil {
			return nil // Skip on error.
		}

		// Defensive check: reject paths that escape (should never happen if rooted).
		if strings.HasPrefix(relPath, "..") {
			return nil
		}

		// Convert to forward slashes for consistent API output.
		relPath = filepath.ToSlash(relPath)

		files = append(files, relPath)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort lexicographically for deterministic output.
	sort.Strings(files)

	return files, nil
}

// validateShareLinkPath validates and returns the absolute link path for a public share.
func validateShareLinkPath(publicBaseDir, relPath string) (string, error) {
	linkPath := filepath.Join(publicBaseDir, relPath)
	linkPath = filepath.Clean(linkPath)

	// CRITICAL: Verify link path stays within publicBaseDir.
	relLink, err := filepath.Rel(publicBaseDir, linkPath)
	if err != nil || strings.HasPrefix(relLink, "..") {
		return "", &pathutil.PathError{
			StatusCode: 400,
			Message:    "invalid path: escapes public base directory",
		}
	}

	return linkPath, nil
}

// ensurePublicLinkDir creates the parent directories for a public share link.
func ensurePublicLinkDir(linkPath string) error {
	linkDir := filepath.Dir(linkPath)
	if err := os.MkdirAll(linkDir, 0755); err != nil {
		if os.IsPermission(err) {
			return &pathutil.PathError{
				StatusCode: 403,
				Message:    "permission denied creating public directory",
			}
		}
		return fmt.Errorf("create public directory structure: %w", err)
	}
	return nil
}

// checkExistingLink checks if a link already exists at the path.
// Returns (true, nil) if the existing link points to the same target (idempotent).
// Returns (false, nil) if no link exists.
// Returns (false, error) if there's a conflict or error.
func checkExistingLink(linkPath, sourceAbsPath string) (bool, error) {
	info, err := os.Lstat(linkPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check link path: %w", err)
	}

	// Something exists at the link path.
	if info.Mode()&os.ModeSymlink != 0 {
		// It's a symlink - check if it points to the same target (idempotent).
		existingTarget, err := os.Readlink(linkPath)
		if err == nil && existingTarget == sourceAbsPath {
			// Same target, treat as success (idempotent).
			return true, nil
		}
		// Different target - conflict.
		return false, &pathutil.PathError{
			StatusCode: 409,
			Message:    "public share already exists with different target",
		}
	}

	// Not a symlink - something else exists there.
	return false, &pathutil.PathError{
		StatusCode: 409,
		Message:    "path already exists in public directory",
	}
}

// createSymlink creates a symlink at linkPath pointing to sourceAbsPath.
func createSymlink(sourceAbsPath, linkPath string) error {
	if err := os.Symlink(sourceAbsPath, linkPath); err != nil {
		if os.IsExist(err) {
			// Race condition - try again or return conflict.
			return &pathutil.PathError{
				StatusCode: 409,
				Message:    "public share already exists",
			}
		}
		if os.IsPermission(err) {
			return &pathutil.PathError{
				StatusCode: 403,
				Message:    "permission denied creating symlink",
			}
		}
		return fmt.Errorf("create symlink: %w", err)
	}
	return nil
}

// validatePublicSharePath validates the path for deleting a public share.
// Returns the absolute link path and the cleaned public base directory.
func validatePublicSharePath(publicBaseDir, relPath string) (string, string, error) {
	if publicBaseDir == "" {
		return "", "", &pathutil.PathError{
			StatusCode: 400,
			Message:    "public-base-dir is not configured",
		}
	}

	if relPath == "" {
		return "", "", &pathutil.PathError{
			StatusCode: 400,
			Message:    "path is required",
		}
	}

	// Clean the path to normalize . and .. components.
	cleanPath := filepath.Clean(relPath)

	// Reject paths containing .. after cleaning.
	if strings.Contains(cleanPath, "..") {
		return "", "", &pathutil.PathError{
			StatusCode: 400,
			Message:    "invalid path: contains parent directory reference",
		}
	}

	// Reject absolute paths.
	if filepath.IsAbs(cleanPath) {
		return "", "", &pathutil.PathError{
			StatusCode: 400,
			Message:    "invalid path: absolute paths not allowed",
		}
	}

	// Reject paths that are just "." after cleaning.
	if cleanPath == "." {
		return "", "", &pathutil.PathError{
			StatusCode: 400,
			Message:    "invalid path: cannot delete base directory",
		}
	}

	// Compute absolute path to the symlink.
	cleanPublicBaseDir := filepath.Clean(publicBaseDir)
	linkAbs := filepath.Join(cleanPublicBaseDir, cleanPath)

	// CRITICAL: Verify the link path stays within publicBaseDir.
	relLink, err := filepath.Rel(cleanPublicBaseDir, linkAbs)
	if err != nil || strings.HasPrefix(relLink, "..") || relLink == "." {
		return "", "", &pathutil.PathError{
			StatusCode: 400,
			Message:    "invalid path: escapes public base directory",
		}
	}

	return linkAbs, cleanPublicBaseDir, nil
}

// verifySymlink verifies that the path is a symlink.
func verifySymlink(linkAbs string) error {
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

	return nil
}

// removeSymlink removes a symlink at the given path.
func removeSymlink(linkAbs string) error {
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
		// Clean to ensure consistent comparison.
		dir = filepath.Clean(dir)

		// Stop if we've reached or would delete stopAt.
		if dir == stopAt {
			return
		}

		// Safety: also stop if dir is somehow a prefix or equal to stopAt
		// or if we've gone above it.
		rel, err := filepath.Rel(stopAt, dir)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return
		}

		// Check if directory is empty.
		isEmpty, err := isDirEmpty(dir)
		if err != nil || !isEmpty {
			// Stop if error or directory is not empty.
			return
		}

		// Remove the empty directory.
		if err := os.Remove(dir); err != nil {
			// Stop on any error (permission, not exists, etc.).
			return
		}

		// Move to parent.
		dir = filepath.Dir(dir)
	}
}
