// Package fs provides filesystem operations for file upload, deletion, and directory creation.
package fs

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"files-browser-backend/internal/pathutil"
)

// FileError represents a file processing error.
type FileError struct {
	Message    string
	IsConflict bool
}

func (e *FileError) Error() string {
	return e.Message
}

// SaveFile saves a single uploaded file to the target directory.
// It validates the filename, prevents overwrites, and ensures atomic writes.
func SaveFile(fh *multipart.FileHeader, targetDir, baseDir string) error {
	// Validate filename
	filename, err := pathutil.ValidateFilename(fh.Filename)
	if err != nil {
		return &FileError{Message: err.Error()}
	}

	// Construct destination path
	destPath := filepath.Join(targetDir, filename)

	// Final safety check: ensure destination is within base directory
	if err := pathutil.ValidateDestination(baseDir, destPath); err != nil {
		return &FileError{Message: "invalid destination path"}
	}

	// Check if file already exists (reject overwrites)
	if _, err := os.Stat(destPath); err == nil {
		return &FileError{Message: "file already exists", IsConflict: true}
	}

	// Open uploaded file for reading
	src, err := fh.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Create destination file with exclusive flag (O_EXCL prevents race condition)
	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return &FileError{Message: "file already exists", IsConflict: true}
		}
		return fmt.Errorf("failed to create destination file: %w", err)
	}

	// Stream copy from source to destination
	_, err = io.Copy(dst, src)
	if err != nil {
		dst.Close()
		os.Remove(destPath) // Clean up partial file
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Sync to ensure data is flushed to disk
	if err := dst.Sync(); err != nil {
		dst.Close()
		os.Remove(destPath)
		return fmt.Errorf("failed to sync file: %w", err)
	}

	if err := dst.Close(); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("failed to close file: %w", err)
	}

	return nil
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// Delete removes a file or empty directory.
// For directories, it verifies they are empty before deletion.
func Delete(targetPath string) error {
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &pathutil.PathError{
				StatusCode: 404,
				Message:    "path does not exist",
			}
		}
		return err
	}

	if info.IsDir() {
		// For directories, verify empty before deletion
		entries, err := os.ReadDir(targetPath)
		if err != nil {
			return fmt.Errorf("failed to read directory: %w", err)
		}
		if len(entries) > 0 {
			return &pathutil.PathError{
				StatusCode: 409,
				Message:    "directory is not empty",
			}
		}
	}

	// Perform the deletion
	if err := os.Remove(targetPath); err != nil {
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
		return err
	}

	return nil
}

// Mkdir creates a new directory with safe permissions.
// SECURITY: Never follows symlinks, verifies target doesn't already exist.
func Mkdir(targetPath string) error {
	// Check if target already exists using Lstat (don't follow symlinks)
	info, err := os.Lstat(targetPath)
	if err == nil {
		// Path exists
		if info.IsDir() {
			return &pathutil.PathError{
				StatusCode: 409,
				Message:    "directory already exists",
			}
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return &pathutil.PathError{
				StatusCode: 409,
				Message:    "path exists as symlink",
			}
		}
		return &pathutil.PathError{
			StatusCode: 409,
			Message:    "path already exists as file",
		}
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check target path: %w", err)
	}

	// Create directory with safe permissions (0755 = rwxr-xr-x)
	const dirPermissions = 0755
	if err := os.Mkdir(targetPath, dirPermissions); err != nil {
		if os.IsExist(err) {
			return &pathutil.PathError{
				StatusCode: 409,
				Message:    "directory already exists",
			}
		}
		if os.IsPermission(err) {
			return &pathutil.PathError{
				StatusCode: 403,
				Message:    "permission denied",
			}
		}
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

// SharePublic creates a symlink in publicBaseDir pointing to the source file.
// The symlink mirrors the same relative directory structure.
// Returns nil on success, or an error with appropriate status code.
func SharePublic(sourceAbsPath, publicBaseDir, relPath string) error {
	// Compute link path in public directory
	linkPath := filepath.Join(publicBaseDir, relPath)
	linkPath = filepath.Clean(linkPath)

	// CRITICAL: Verify link path stays within publicBaseDir
	relLink, err := filepath.Rel(publicBaseDir, linkPath)
	if err != nil || strings.HasPrefix(relLink, "..") {
		return &pathutil.PathError{
			StatusCode: 400,
			Message:    "invalid path: escapes public base directory",
		}
	}

	// Ensure parent directories exist in public directory
	linkDir := filepath.Dir(linkPath)
	if err := os.MkdirAll(linkDir, 0755); err != nil {
		if os.IsPermission(err) {
			return &pathutil.PathError{
				StatusCode: 403,
				Message:    "permission denied creating public directory",
			}
		}
		return fmt.Errorf("failed to create public directory structure: %w", err)
	}

	// Check if link already exists
	info, err := os.Lstat(linkPath)
	if err == nil {
		// Something exists at the link path
		if info.Mode()&os.ModeSymlink != 0 {
			// It's a symlink - check if it points to the same target (idempotent)
			existingTarget, err := os.Readlink(linkPath)
			if err == nil && existingTarget == sourceAbsPath {
				// Same target, treat as success (idempotent)
				return nil
			}
			// Different target - conflict
			return &pathutil.PathError{
				StatusCode: 409,
				Message:    "public share already exists with different target",
			}
		}
		// Not a symlink - something else exists there
		return &pathutil.PathError{
			StatusCode: 409,
			Message:    "path already exists in public directory",
		}
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check link path: %w", err)
	}

	// Create symlink pointing to the absolute source path
	if err := os.Symlink(sourceAbsPath, linkPath); err != nil {
		if os.IsExist(err) {
			// Race condition - try again or return conflict
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
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return nil
}
