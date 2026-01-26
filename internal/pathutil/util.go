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

// Common error constructors for consistent error messages.
func errBadRequest(msg string) *PathError {
	return &PathError{StatusCode: 400, Message: msg}
}

func errNotFound(msg string) *PathError {
	return &PathError{StatusCode: 404, Message: msg}
}

func errForbidden(msg string) *PathError {
	return &PathError{StatusCode: 403, Message: msg}
}

func errConflict(msg string) *PathError {
	return &PathError{StatusCode: 409, Message: msg}
}

func errInternal(msg string) *PathError {
	return &PathError{StatusCode: 500, Message: msg}
}

// cleanPath normalizes and validates a path for traversal attempts.
// Returns the cleaned path or an error if validation fails.
func cleanPath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "..") {
		return "", errBadRequest("invalid path: contains parent directory reference")
	}
	if filepath.IsAbs(cleaned) {
		return "", errBadRequest("invalid path: absolute paths not allowed")
	}
	return cleaned, nil
}

// validateNotEmpty checks that a path is not empty or just ".".
func validateNotEmpty(path, errMsg string) error {
	if path == "" || path == "." {
		return errBadRequest(errMsg)
	}
	return nil
}

// validateNoNullBytes checks for null bytes in a string.
func validateNoNullBytes(s, context string) error {
	if strings.ContainsRune(s, '\x00') {
		return errBadRequest(fmt.Sprintf("invalid %s: contains null byte", context))
	}
	return nil
}

// validateName validates a simple filename or directory name.
// It rejects empty, special names (., ..), path separators, and null bytes.
func validateName(name, context string) error {
	cleaned := filepath.Clean(name)
	if strings.ContainsAny(cleaned, "/\\") || strings.Contains(cleaned, "..") {
		return errBadRequest(fmt.Sprintf("invalid %s: must be a simple name without path separators", context))
	}
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return errBadRequest(fmt.Sprintf("invalid %s", context))
	}
	return validateNoNullBytes(cleaned, context)
}

// isWithinBase checks if targetPath is within baseDir using relative path calculation.
// Returns the relative path if valid, or an error if the path escapes the base.
// If allowBase is true, "." (the base directory itself) is allowed.
func isWithinBase(baseDir, targetPath string, allowBase bool) (string, error) {
	relPath, err := filepath.Rel(baseDir, targetPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", errBadRequest("invalid path: escapes base directory")
	}
	if relPath == "." && !allowBase {
		return "", errBadRequest("invalid path: escapes base directory")
	}
	return relPath, nil
}

// lstatPath performs Lstat on a path and returns file info with proper error mapping.
func lstatPath(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errNotFound("path does not exist")
		}
		return nil, errInternal("failed to stat path")
	}
	return info, nil
}

// rejectSymlink returns an error if the file info indicates a symlink.
func rejectSymlink(info os.FileInfo, action string) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return errBadRequest(fmt.Sprintf("cannot %s symlinks", action))
	}
	return nil
}

// ensureNotExists returns an error if a path already exists.
func ensureNotExists(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return errConflict("destination already exists")
	}
	if !os.IsNotExist(err) {
		return errInternal("failed to check destination")
	}
	return nil
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
	realBaseDir, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return "", errInternal("base directory resolution failed")
	}

	cleanedPath, err := cleanPath(urlPath)
	if err != nil {
		return "", err
	}

	targetDir := filepath.Join(realBaseDir, cleanedPath)
	realTarget, err := filepath.EvalSymlinks(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Path doesn't exist yet; verify it would be within base if created.
			if _, err := isWithinBase(realBaseDir, targetDir, true); err != nil {
				return "", err
			}
			return targetDir, nil
		}
		return "", errNotFound("invalid target path")
	}

	if _, err := isWithinBase(realBaseDir, realTarget, true); err != nil {
		return "", err
	}
	return realTarget, nil
}

// ResolveDeletePath validates and resolves a path for deletion.
// SECURITY CRITICAL: Prevents path traversal and symlink escape using Lstat.
func ResolveDeletePath(baseDir, urlPath string) (string, error) {
	if err := validateNotEmpty(urlPath, "cannot delete base directory"); err != nil {
		return "", errForbidden("cannot delete base directory")
	}

	cleanedPath, err := cleanPath(urlPath)
	if err != nil {
		return "", err
	}

	targetPath := filepath.Join(baseDir, cleanedPath)
	if _, err := isWithinBase(baseDir, targetPath, false); err != nil {
		return "", err
	}

	info, err := lstatPath(targetPath)
	if err != nil {
		return "", err
	}
	if err := rejectSymlink(info, "delete"); err != nil {
		return "", err
	}

	return targetPath, nil
}

// ResolveMkdirPath validates and resolves a path for directory creation.
// Returns the resolved filesystem path and the virtual path (for response).
// SECURITY CRITICAL: Prevents path traversal and symlink escape.
func ResolveMkdirPath(baseDir, urlPath string) (resolvedPath, virtualPath string, err error) {
	if err := validateNotEmpty(urlPath, "cannot create base directory"); err != nil {
		return "", "", errForbidden("cannot create base directory")
	}

	cleanedPath, err := cleanPath(urlPath)
	if err != nil {
		return "", "", err
	}

	dirName := filepath.Base(cleanedPath)
	if err := validateName(dirName, "directory name"); err != nil {
		return "", "", err
	}

	targetPath := filepath.Join(baseDir, cleanedPath)
	if _, err := isWithinBase(baseDir, targetPath, false); err != nil {
		return "", "", errForbidden("invalid path: escapes base directory")
	}

	parentPath := filepath.Dir(cleanedPath)
	parentFullPath := filepath.Join(baseDir, parentPath)
	if err := validateParentDir(baseDir, parentFullPath); err != nil {
		return "", "", err
	}

	realParent, err := filepath.EvalSymlinks(parentFullPath)
	if err != nil {
		return "", "", errNotFound("parent directory not accessible")
	}

	resolvedTarget := filepath.Join(realParent, dirName)
	return resolvedTarget, cleanedPath, nil
}

// validateParentDir checks that a parent directory exists, is a directory, not a symlink,
// and is within the base directory.
func validateParentDir(baseDir, parentFullPath string) error {
	parentInfo, err := os.Lstat(parentFullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errNotFound("parent directory does not exist")
		}
		return errInternal("failed to stat parent")
	}
	if parentInfo.Mode()&os.ModeSymlink != 0 {
		return errForbidden("cannot create directory under symlink")
	}
	if !parentInfo.IsDir() {
		return errBadRequest("parent path is not a directory")
	}

	realParent, err := filepath.EvalSymlinks(parentFullPath)
	if err != nil {
		return errNotFound("parent directory not accessible")
	}
	realBase, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return errInternal("failed to resolve base directory")
	}

	relParent, err := filepath.Rel(realBase, realParent)
	if err != nil || strings.HasPrefix(relParent, "..") {
		return errForbidden("parent directory escapes base directory")
	}
	return nil
}

// ResolveRenamePaths validates and resolves paths for rename operation.
// Returns resolved filesystem paths and virtual paths (for response).
// SECURITY CRITICAL: Prevents path traversal, symlink escape, and overwriting.
func ResolveRenamePaths(baseDir, oldPath, newName string) (resolvedOld, resolvedNew, virtualOld, virtualNew string, err error) {
	if err := validateNotEmpty(oldPath, "source path is required"); err != nil {
		return "", "", "", "", err
	}
	if newName == "" {
		return "", "", "", "", errBadRequest("new name is required")
	}

	cleanOldPath, err := cleanPath(oldPath)
	if err != nil {
		return "", "", "", "", err
	}
	if err := validateName(newName, "new name"); err != nil {
		return "", "", "", "", err
	}
	cleanNewName := filepath.Clean(newName)

	oldFullPath := filepath.Join(baseDir, cleanOldPath)
	if _, err := isWithinBase(baseDir, oldFullPath, false); err != nil {
		return "", "", "", "", err
	}

	oldInfo, err := lstatPath(oldFullPath)
	if err != nil {
		if pathErr, ok := err.(*PathError); ok && pathErr.StatusCode == 404 {
			return "", "", "", "", errNotFound("source path does not exist")
		}
		return "", "", "", "", errInternal("failed to stat source path")
	}
	if err := rejectSymlink(oldInfo, "rename"); err != nil {
		return "", "", "", "", err
	}

	parentDir := filepath.Dir(oldFullPath)
	newFullPath := filepath.Join(parentDir, cleanNewName)
	relNewPath, err := isWithinBase(baseDir, newFullPath, false)
	if err != nil {
		return "", "", "", "", errBadRequest("invalid new name: would escape base directory")
	}

	if err := ensureNotExists(newFullPath); err != nil {
		return "", "", "", "", err
	}

	return oldFullPath, newFullPath, cleanOldPath, relNewPath, nil
}

// ResolveMovePaths validates and resolves paths for move operation.
// Returns resolved filesystem paths and virtual paths (for response).
// SECURITY CRITICAL: Prevents path traversal, symlink escape, and overwriting.
func ResolveMovePaths(baseDir, sourcePath, destPath string) (resolvedSource, resolvedDest, virtualSource, virtualDest string, err error) {
	if err := validateNotEmpty(sourcePath, "source path is required"); err != nil {
		return "", "", "", "", err
	}
	if err := validateNotEmpty(destPath, "destination path is required"); err != nil {
		return "", "", "", "", err
	}

	cleanSourcePath, err := cleanAndValidateMovePath(sourcePath, "source")
	if err != nil {
		return "", "", "", "", err
	}
	cleanDestPath, err := cleanAndValidateMovePath(destPath, "destination")
	if err != nil {
		return "", "", "", "", err
	}

	sourceFullPath := filepath.Join(baseDir, cleanSourcePath)
	destFullPath := filepath.Join(baseDir, cleanDestPath)

	if _, err := isWithinBase(baseDir, sourceFullPath, false); err != nil {
		return "", "", "", "", errBadRequest("invalid source path: escapes base directory")
	}
	if _, err := isWithinBase(baseDir, destFullPath, false); err != nil {
		return "", "", "", "", errBadRequest("invalid destination path: escapes base directory")
	}

	sourceInfo, err := lstatPath(sourceFullPath)
	if err != nil {
		if pathErr, ok := err.(*PathError); ok && pathErr.StatusCode == 404 {
			return "", "", "", "", errNotFound("source path does not exist")
		}
		return "", "", "", "", errInternal("failed to stat source path")
	}
	if err := rejectSymlink(sourceInfo, "move"); err != nil {
		return "", "", "", "", err
	}

	if err := validateDestParent(destFullPath); err != nil {
		return "", "", "", "", err
	}
	if err := ensureNotExists(destFullPath); err != nil {
		return "", "", "", "", err
	}

	return sourceFullPath, destFullPath, cleanSourcePath, cleanDestPath, nil
}

// cleanAndValidateMovePath cleans and validates a path for move operations.
func cleanAndValidateMovePath(path, context string) (string, error) {
	cleanedPath, err := cleanPath(path)
	if err != nil {
		// Adjust error message for context.
		if strings.Contains(err.Error(), "parent directory reference") {
			return "", errBadRequest(fmt.Sprintf("invalid %s path: contains parent directory reference", context))
		}
		if strings.Contains(err.Error(), "absolute paths") {
			return "", errBadRequest(fmt.Sprintf("invalid %s path: absolute paths not allowed", context))
		}
		return "", err
	}
	if err := validateNoNullBytes(cleanedPath, context+" path"); err != nil {
		return "", err
	}
	return cleanedPath, nil
}

// validateDestParent checks that the destination parent directory exists and is valid.
func validateDestParent(destFullPath string) error {
	destParentDir := filepath.Dir(destFullPath)
	destParentInfo, err := os.Lstat(destParentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return errNotFound("destination parent directory does not exist")
		}
		return errInternal("failed to stat destination parent")
	}
	if destParentInfo.Mode()&os.ModeSymlink != 0 {
		return errBadRequest("cannot move to directory under symlink")
	}
	if !destParentInfo.IsDir() {
		return errBadRequest("destination parent is not a directory")
	}
	return nil
}

// ValidateFilename validates an uploaded filename.
// Returns the sanitized filename (base name only) or an error.
func ValidateFilename(filename string) (string, error) {
	baseName := filepath.Base(filename)
	if baseName == "" || baseName == "." || baseName == ".." {
		return "", errBadRequest("invalid filename")
	}
	if strings.HasPrefix(baseName, ".") {
		return "", errBadRequest("hidden files not allowed")
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
		return errBadRequest("invalid destination path")
	}
	return nil
}

// ResolveSharePublicPath validates and resolves a path for public sharing.
// Returns the resolved filesystem path and virtual path.
// SECURITY CRITICAL: Prevents path traversal, symlink escape, and ensures only regular files.
func ResolveSharePublicPath(baseDir, urlPath string) (resolvedPath, virtualPath string, err error) {
	if err := validateNotEmpty(urlPath, "file path is required"); err != nil {
		return "", "", err
	}

	cleanedPath, err := cleanPath(urlPath)
	if err != nil {
		return "", "", err
	}

	targetPath := filepath.Join(baseDir, cleanedPath)
	if _, err := isWithinBase(baseDir, targetPath, false); err != nil {
		return "", "", err
	}

	info, err := lstatPath(targetPath)
	if err != nil {
		return "", "", err
	}
	if err := rejectSymlink(info, "share"); err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return "", "", errBadRequest("only regular files can be shared publicly")
	}

	return targetPath, cleanedPath, nil
}
