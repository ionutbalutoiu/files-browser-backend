// Package pathutil provides path validation and normalization utilities.
package pathutil

import (
	"fmt"
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

// ValidateRelativePath validates that a path is safe (no traversal, not absolute).
func ValidateRelativePath(path string) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if strings.HasPrefix(path, "/") {
		return fmt.Errorf("absolute paths not allowed")
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal not allowed")
	}
	return nil
}

// ResolveTargetDir validates and resolves a target directory path for uploads.
// It ensures the path is safe and within the base directory.
func ResolveTargetDir(baseDir, urlPath string) (string, error) {
	// Resolve symlinks in baseDir first (handles macOS /var -> /private/var)
	realBaseDir, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return "", &PathError{
			StatusCode: 500,
			Message:    "base directory resolution failed",
		}
	}

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
	targetDir := filepath.Join(realBaseDir, cleanPath)

	// Resolve any symlinks to get the real path
	realTarget, err := filepath.EvalSymlinks(targetDir)
	if err != nil {
		// If path doesn't exist, check parent directory
		if os.IsNotExist(err) {
			// Verify the path would still be under base if created
			relPath, relErr := filepath.Rel(realBaseDir, targetDir)
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
	relPath, err := filepath.Rel(realBaseDir, realTarget)
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

// ResolveRenamePaths validates and resolves paths for rename operation.
// Returns resolved filesystem paths and virtual paths (for response).
// SECURITY CRITICAL: Prevents path traversal, symlink escape, and overwriting.
func ResolveRenamePaths(baseDir, oldPath, newName string) (resolvedOld, resolvedNew, virtualOld, virtualNew string, err error) {
	// Reject empty old path
	if oldPath == "" || oldPath == "." {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "source path is required",
		}
	}

	// Reject empty new name
	if newName == "" {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "new name is required",
		}
	}

	// Clean the old path
	cleanOldPath := filepath.Clean(oldPath)

	// Reject paths containing .. after cleaning
	if strings.Contains(cleanOldPath, "..") {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid path: contains parent directory reference",
		}
	}

	// Reject absolute paths
	if filepath.IsAbs(cleanOldPath) {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid path: absolute paths not allowed",
		}
	}

	// Validate new name - must be a simple name, not a path
	cleanNewName := filepath.Clean(newName)
	if strings.ContainsAny(cleanNewName, "/\\") || strings.Contains(cleanNewName, "..") {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid new name: must be a simple name without path separators",
		}
	}

	// Reject special names
	if cleanNewName == "" || cleanNewName == "." || cleanNewName == ".." {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid new name",
		}
	}

	// Reject names containing null bytes
	if strings.ContainsRune(cleanNewName, '\x00') {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid new name: contains null byte",
		}
	}

	// Construct full old path
	oldFullPath := filepath.Join(baseDir, cleanOldPath)

	// CRITICAL: Verify the old path is strictly within base directory
	relOldPath, err := filepath.Rel(baseDir, oldFullPath)
	if err != nil || strings.HasPrefix(relOldPath, "..") || relOldPath == "." {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid path: escapes base directory",
		}
	}

	// Use Lstat to check if old path exists WITHOUT following symlinks
	oldInfo, err := os.Lstat(oldFullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", "", "", &PathError{
				StatusCode: 404,
				Message:    "source path does not exist",
			}
		}
		return "", "", "", "", &PathError{
			StatusCode: 500,
			Message:    "failed to stat source path",
		}
	}

	// SECURITY: Reject symlinks
	if oldInfo.Mode()&os.ModeSymlink != 0 {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "cannot rename symlinks",
		}
	}

	// Construct new path (same parent directory, new name)
	parentDir := filepath.Dir(oldFullPath)
	newFullPath := filepath.Join(parentDir, cleanNewName)

	// CRITICAL: Verify new path is also within base directory
	relNewPath, err := filepath.Rel(baseDir, newFullPath)
	if err != nil || strings.HasPrefix(relNewPath, "..") {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid new name: would escape base directory",
		}
	}

	// Check if new path already exists
	if _, err := os.Lstat(newFullPath); err == nil {
		return "", "", "", "", &PathError{
			StatusCode: 409,
			Message:    "destination already exists",
		}
	} else if !os.IsNotExist(err) {
		return "", "", "", "", &PathError{
			StatusCode: 500,
			Message:    "failed to check destination",
		}
	}

	// Virtual paths for response (relative to base)
	virtualOld = cleanOldPath
	virtualNew = relNewPath

	return oldFullPath, newFullPath, virtualOld, virtualNew, nil
}

// ResolveMovePaths validates and resolves paths for move operation.
// Returns resolved filesystem paths and virtual paths (for response).
// SECURITY CRITICAL: Prevents path traversal, symlink escape, and overwriting.
func ResolveMovePaths(baseDir, sourcePath, destPath string) (resolvedSource, resolvedDest, virtualSource, virtualDest string, err error) {
	// Reject empty source path
	if sourcePath == "" || sourcePath == "." {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "source path is required",
		}
	}

	// Reject empty destination path
	if destPath == "" || destPath == "." {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "destination path is required",
		}
	}

	// Clean both paths
	cleanSourcePath := filepath.Clean(sourcePath)
	cleanDestPath := filepath.Clean(destPath)

	// Reject paths containing .. after cleaning
	if strings.Contains(cleanSourcePath, "..") {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid source path: contains parent directory reference",
		}
	}
	if strings.Contains(cleanDestPath, "..") {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid destination path: contains parent directory reference",
		}
	}

	// Reject absolute paths
	if filepath.IsAbs(cleanSourcePath) {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid source path: absolute paths not allowed",
		}
	}
	if filepath.IsAbs(cleanDestPath) {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid destination path: absolute paths not allowed",
		}
	}

	// Reject null bytes in paths
	if strings.ContainsRune(cleanSourcePath, '\x00') {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid source path: contains null byte",
		}
	}
	if strings.ContainsRune(cleanDestPath, '\x00') {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid destination path: contains null byte",
		}
	}

	// Construct full paths
	sourceFullPath := filepath.Join(baseDir, cleanSourcePath)
	destFullPath := filepath.Join(baseDir, cleanDestPath)

	// CRITICAL: Verify the source path is strictly within base directory
	relSourcePath, err := filepath.Rel(baseDir, sourceFullPath)
	if err != nil || strings.HasPrefix(relSourcePath, "..") || relSourcePath == "." {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid source path: escapes base directory",
		}
	}

	// CRITICAL: Verify the destination path is strictly within base directory
	relDestPath, err := filepath.Rel(baseDir, destFullPath)
	if err != nil || strings.HasPrefix(relDestPath, "..") || relDestPath == "." {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid destination path: escapes base directory",
		}
	}

	// Use Lstat to check if source path exists WITHOUT following symlinks
	sourceInfo, err := os.Lstat(sourceFullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", "", "", &PathError{
				StatusCode: 404,
				Message:    "source path does not exist",
			}
		}
		return "", "", "", "", &PathError{
			StatusCode: 500,
			Message:    "failed to stat source path",
		}
	}

	// SECURITY: Reject symlinks as source
	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "cannot move symlinks",
		}
	}

	// Verify destination parent directory exists
	destParentDir := filepath.Dir(destFullPath)
	destParentInfo, err := os.Lstat(destParentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", "", "", &PathError{
				StatusCode: 404,
				Message:    "destination parent directory does not exist",
			}
		}
		return "", "", "", "", &PathError{
			StatusCode: 500,
			Message:    "failed to stat destination parent",
		}
	}

	// SECURITY: Reject if destination parent is a symlink
	if destParentInfo.Mode()&os.ModeSymlink != 0 {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "cannot move to directory under symlink",
		}
	}

	if !destParentInfo.IsDir() {
		return "", "", "", "", &PathError{
			StatusCode: 400,
			Message:    "destination parent is not a directory",
		}
	}

	// Check if destination already exists
	if _, err := os.Lstat(destFullPath); err == nil {
		return "", "", "", "", &PathError{
			StatusCode: 409,
			Message:    "destination already exists",
		}
	} else if !os.IsNotExist(err) {
		return "", "", "", "", &PathError{
			StatusCode: 500,
			Message:    "failed to check destination",
		}
	}

	// Virtual paths for response (relative to base)
	virtualSource = cleanSourcePath
	virtualDest = cleanDestPath

	return sourceFullPath, destFullPath, virtualSource, virtualDest, nil
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

// ResolveSharePublicPath validates and resolves a path for public sharing.
// It validates the source file path and returns the resolved filesystem path.
// SECURITY CRITICAL: Prevents path traversal, symlink escape, and ensures only regular files.
func ResolveSharePublicPath(baseDir, urlPath string) (resolvedPath, virtualPath string, err error) {
	// Reject empty path
	if urlPath == "" || urlPath == "." {
		return "", "", &PathError{
			StatusCode: 400,
			Message:    "file path is required",
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

	// Construct full target path
	targetPath := filepath.Join(baseDir, cleanPath)

	// CRITICAL: Verify the target is strictly within base directory
	relPath, err := filepath.Rel(baseDir, targetPath)
	if err != nil || strings.HasPrefix(relPath, "..") || relPath == "." {
		return "", "", &PathError{
			StatusCode: 400,
			Message:    "invalid path: escapes base directory",
		}
	}

	// Use Lstat to check if path exists WITHOUT following symlinks
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", &PathError{
				StatusCode: 404,
				Message:    "path does not exist",
			}
		}
		return "", "", &PathError{
			StatusCode: 500,
			Message:    "failed to stat path",
		}
	}

	// SECURITY: Reject symlinks
	if info.Mode()&os.ModeSymlink != 0 {
		return "", "", &PathError{
			StatusCode: 400,
			Message:    "cannot share symlinks",
		}
	}

	// Only allow regular files
	if !info.Mode().IsRegular() {
		return "", "", &PathError{
			StatusCode: 400,
			Message:    "only regular files can be shared publicly",
		}
	}

	return targetPath, cleanPath, nil
}
