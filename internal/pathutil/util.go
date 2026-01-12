// Package pathutil provides path validation and normalization utilities.
package pathutil

import (
	"os"
	"path/filepath"
	"strings"
)

// PathError represents a path validation error with HTTP status code.
type PathError struct {
	StatusCode int
	Message    string
}

func (e *PathError) Error() string {
	return e.Message
}

// ResolveTargetDir validates and resolves a target directory path for uploads.
// It ensures the path is safe and within the base directory.
func ResolveTargetDir(baseDir, urlPath string) (string, error) {
	// Clean the path to remove any . or .. components
	cleanPath := filepath.Clean(urlPath)

	// Reject paths that try to escape using ..
	if strings.Contains(cleanPath, "..") {
		return "", &PathError{
			StatusCode: 400,
			Message:    "invalid path: contains parent directory reference",
		}
	}

	// Reject absolute paths
	if filepath.IsAbs(cleanPath) {
		return "", &PathError{
			StatusCode: 400,
			Message:    "invalid path: absolute paths not allowed",
		}
	}

	// Construct full target path
	targetDir := filepath.Join(baseDir, cleanPath)

	// Resolve any symlinks to get the real path
	realTarget, err := filepath.EvalSymlinks(targetDir)
	if err != nil {
		// If path doesn't exist, check parent directory
		if os.IsNotExist(err) {
			// Verify the path would still be under base if created
			relPath, relErr := filepath.Rel(baseDir, targetDir)
			if relErr != nil || strings.HasPrefix(relPath, "..") {
				return "", &PathError{
					StatusCode: 400,
					Message:    "invalid path: escapes base directory",
				}
			}
			return targetDir, nil
		}
		return "", &PathError{
			StatusCode: 404,
			Message:    "invalid target path",
		}
	}

	// Verify resolved path is within base directory
	relPath, err := filepath.Rel(baseDir, realTarget)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", &PathError{
			StatusCode: 400,
			Message:    "invalid path: escapes base directory",
		}
	}

	return realTarget, nil
}

// ResolveDeletePath validates and resolves a path for deletion.
// SECURITY CRITICAL: This function prevents path traversal and symlink escape.
// It uses Lstat (not Stat) to avoid following symlinks.
func ResolveDeletePath(baseDir, urlPath string) (string, error) {
	// Reject empty path (would delete base directory)
	if urlPath == "" || urlPath == "." {
		return "", &PathError{
			StatusCode: 403,
			Message:    "cannot delete base directory",
		}
	}

	// Clean the path to normalize . and .. components
	cleanPath := filepath.Clean(urlPath)

	// Reject paths containing .. after cleaning
	if strings.Contains(cleanPath, "..") {
		return "", &PathError{
			StatusCode: 400,
			Message:    "invalid path: contains parent directory reference",
		}
	}

	// Reject absolute paths
	if filepath.IsAbs(cleanPath) {
		return "", &PathError{
			StatusCode: 400,
			Message:    "invalid path: absolute paths not allowed",
		}
	}

	// Construct full target path
	targetPath := filepath.Join(baseDir, cleanPath)

	// CRITICAL: Verify the target is strictly within base directory
	relPath, err := filepath.Rel(baseDir, targetPath)
	if err != nil || strings.HasPrefix(relPath, "..") || relPath == "." {
		return "", &PathError{
			StatusCode: 400,
			Message:    "invalid path: escapes base directory",
		}
	}

	// Use Lstat to check if path exists WITHOUT following symlinks
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &PathError{
				StatusCode: 404,
				Message:    "path does not exist",
			}
		}
		return "", &PathError{
			StatusCode: 500,
			Message:    "failed to stat path",
		}
	}

	// SECURITY: Reject symlinks entirely to prevent escape attacks
	if info.Mode()&os.ModeSymlink != 0 {
		return "", &PathError{
			StatusCode: 400,
			Message:    "cannot delete symlinks",
		}
	}

	return targetPath, nil
}

// ResolveMkdirPath validates and resolves a path for directory creation.
// Returns the resolved filesystem path and the virtual path (for response).
// SECURITY CRITICAL: This function prevents path traversal and symlink escape.
func ResolveMkdirPath(baseDir, urlPath string) (resolvedPath, virtualPath string, err error) {
	// Reject empty path (would create base directory itself)
	if urlPath == "" || urlPath == "." {
		return "", "", &PathError{
			StatusCode: 403,
			Message:    "cannot create base directory",
		}
	}

	// Clean the path to normalize . and .. components
	cleanPath := filepath.Clean(urlPath)

	// Reject paths containing .. after cleaning
	if strings.Contains(cleanPath, "..") {
		return "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid path: contains parent directory reference",
		}
	}

	// Reject absolute paths
	if filepath.IsAbs(cleanPath) {
		return "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid path: absolute paths not allowed",
		}
	}

	// Split into parent directory and new directory name
	parentPath := filepath.Dir(cleanPath)
	dirName := filepath.Base(cleanPath)

	// SECURITY: Validate directory name independently
	if strings.ContainsAny(dirName, "/\\") {
		return "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid directory name: contains path separator",
		}
	}

	// Reject names containing null bytes
	if strings.ContainsRune(dirName, '\x00') {
		return "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid directory name: contains null byte",
		}
	}

	// Reject empty or special names
	if dirName == "" || dirName == "." || dirName == ".." {
		return "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid directory name",
		}
	}

	// Construct full target path
	targetPath := filepath.Join(baseDir, cleanPath)

	// CRITICAL: Verify the target is strictly within base directory
	relPath, err := filepath.Rel(baseDir, targetPath)
	if err != nil || strings.HasPrefix(relPath, "..") || relPath == "." {
		return "", "", &PathError{
			StatusCode: 403,
			Message:    "invalid path: escapes base directory",
		}
	}

	// Construct parent path in filesystem
	parentFullPath := filepath.Join(baseDir, parentPath)

	// Verify parent directory exists and is a directory (not a symlink)
	parentInfo, err := os.Lstat(parentFullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", &PathError{
				StatusCode: 404,
				Message:    "parent directory does not exist",
			}
		}
		return "", "", &PathError{
			StatusCode: 500,
			Message:    "failed to stat parent",
		}
	}

	// SECURITY: Reject if parent is a symlink
	if parentInfo.Mode()&os.ModeSymlink != 0 {
		return "", "", &PathError{
			StatusCode: 403,
			Message:    "cannot create directory under symlink",
		}
	}

	if !parentInfo.IsDir() {
		return "", "", &PathError{
			StatusCode: 400,
			Message:    "parent path is not a directory",
		}
	}

	// CRITICAL: Verify parent resolved path is still within base directory
	realParent, err := filepath.EvalSymlinks(parentFullPath)
	if err != nil {
		return "", "", &PathError{
			StatusCode: 404,
			Message:    "parent directory not accessible",
		}
	}

	// Also resolve base directory for comparison
	realBase, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return "", "", &PathError{
			StatusCode: 500,
			Message:    "failed to resolve base directory",
		}
	}

	relParent, err := filepath.Rel(realBase, realParent)
	if err != nil || strings.HasPrefix(relParent, "..") {
		return "", "", &PathError{
			StatusCode: 403,
			Message:    "parent directory escapes base directory",
		}
	}

	// Return the resolved path (using the real parent path)
	resolvedTarget := filepath.Join(realParent, dirName)

	return resolvedTarget, cleanPath, nil
}

// ValidateFilename validates an uploaded filename.
// Returns the sanitized filename (base name only) or an error.
func ValidateFilename(filename string) (string, error) {
	// Get base name only
	baseName := filepath.Base(filename)

	// Reject empty filenames
	if baseName == "" || baseName == "." || baseName == ".." {
		return "", &PathError{
			StatusCode: 400,
			Message:    "invalid filename",
		}
	}

	// Reject hidden files (starting with .)
	if strings.HasPrefix(baseName, ".") {
		return "", &PathError{
			StatusCode: 400,
			Message:    "hidden files not allowed",
		}
	}

	return baseName, nil
}

// ValidateDestination ensures a destination path is within the base directory.
func ValidateDestination(baseDir, destPath string) error {
	realBase, _ := filepath.EvalSymlinks(baseDir)
	if realBase == "" {
		realBase = baseDir
	}

	relPath, err := filepath.Rel(realBase, destPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return &PathError{
			StatusCode: 400,
			Message:    "invalid destination path",
		}
	}

	return nil
}
