// Package service provides filesystem operations for file upload, deletion, and directory creation.
package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"

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
// The context can be used for cancellation of long-running uploads.
func SaveFile(ctx context.Context, fh *multipart.FileHeader, targetDir, baseDir string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("operation cancelled: %w", err)
	}

	// Validate filename.
	filename, err := pathutil.ValidateFilename(fh.Filename)
	if err != nil {
		return &FileError{Message: err.Error()}
	}

	// Construct destination path.
	destPath := filepath.Join(targetDir, filename)

	// Final safety check: ensure destination is within base directory.
	if err := pathutil.ValidateDestination(baseDir, destPath); err != nil {
		return &FileError{Message: "invalid destination path"}
	}

	// Check if file already exists (reject overwrites).
	if _, err := os.Stat(destPath); err == nil {
		return &FileError{Message: "file already exists", IsConflict: true}
	}

	// Open uploaded file for reading.
	src, err := fh.Open()
	if err != nil {
		return fmt.Errorf("open uploaded file: %w", err)
	}
	defer func() {
		if err := src.Close(); err != nil {
			log.Printf("WARN: failed to close source file: %v", err)
		}
	}()

	return writeAndSyncFile(src, destPath)
}

// writeAndSyncFile creates a file at destPath, copies content from src, syncs to disk,
// and cleans up on any error.
func writeAndSyncFile(src io.Reader, destPath string) error {
	// Create destination file with exclusive flag (O_EXCL prevents race condition).
	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return &FileError{Message: "file already exists", IsConflict: true}
		}
		return fmt.Errorf("create destination file: %w", err)
	}

	// cleanup closes the file and removes it on error.
	cleanup := func(writeErr error) error {
		if closeErr := dst.Close(); closeErr != nil {
			log.Printf("WARN: failed to close destination file during cleanup: %v", closeErr)
		}
		if removeErr := os.Remove(destPath); removeErr != nil {
			log.Printf("WARN: failed to remove file during cleanup: %v", removeErr)
		}
		return writeErr
	}

	// Stream copy from source to destination.
	if _, err := io.Copy(dst, src); err != nil {
		return cleanup(fmt.Errorf("write file: %w", err))
	}

	// Sync to ensure data is flushed to disk.
	if err := dst.Sync(); err != nil {
		return cleanup(fmt.Errorf("sync file: %w", err))
	}

	if err := dst.Close(); err != nil {
		if removeErr := os.Remove(destPath); removeErr != nil {
			log.Printf("WARN: failed to remove file during cleanup: %v", removeErr)
		}
		return fmt.Errorf("close file: %w", err)
	}

	return nil
}

// EnsureDir creates a directory if it doesn't exist.
// The context can be used for cancellation.
func EnsureDir(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("operation cancelled: %w", err)
	}
	return os.MkdirAll(path, 0755)
}

// Delete removes a file or empty directory.
// For directories, it verifies they are empty before deletion.
// The context can be used for cancellation.
func Delete(ctx context.Context, targetPath string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("operation cancelled: %w", err)
	}
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
		// For directories, verify empty before deletion.
		entries, err := os.ReadDir(targetPath)
		if err != nil {
			return fmt.Errorf("read directory: %w", err)
		}
		if len(entries) > 0 {
			return &pathutil.PathError{
				StatusCode: 409,
				Message:    "directory is not empty",
			}
		}
	}

	// Perform the deletion.
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
// The context can be used for cancellation.
func Mkdir(ctx context.Context, targetPath string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("operation cancelled: %w", err)
	}
	// Check if target already exists using Lstat (don't follow symlinks).
	info, err := os.Lstat(targetPath)
	if err == nil {
		// Path exists.
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
		return fmt.Errorf("check target path: %w", err)
	}

	// Create directory with safe permissions (0755 = rwxr-xr-x).
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
		return fmt.Errorf("create directory: %w", err)
	}

	return nil
}

// isDirEmpty checks if a directory is empty.
func isDirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("WARN: failed to close directory handle: %v", err)
		}
	}()

	// Read at most 1 entry.
	names, err := f.Readdirnames(1)
	if err != nil && err != io.EOF {
		return false, err
	}

	return len(names) == 0, nil
}
