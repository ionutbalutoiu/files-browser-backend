package fs

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListSharePublicFiles returns a sorted list of all publicly shared files
// under publicBaseDir. It includes symlinks pointing to regular files and
// regular files directly present. Directories and broken/invalid symlinks
// are skipped.
func ListSharePublicFiles(publicBaseDir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(publicBaseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip entries we can't access
			return nil
		}

		// Skip the root directory itself
		if path == publicBaseDir {
			return nil
		}

		// Skip directories (but continue walking into them)
		if d.IsDir() {
			return nil
		}

		// Use Lstat to get info without following symlinks
		info, err := os.Lstat(path)
		if err != nil {
			return nil // Skip on error
		}

		// Check if it's a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			// Follow the symlink to check target
			targetInfo, err := os.Stat(path)
			if err != nil {
				// Broken symlink or inaccessible target - skip
				return nil
			}
			if !targetInfo.Mode().IsRegular() {
				// Symlink points to non-regular file (e.g., directory) - skip
				return nil
			}
			// Valid symlink to regular file - include it
		} else if info.Mode().IsRegular() {
			// Regular file - include it
		} else {
			// Something else (device, socket, etc.) - skip
			return nil
		}

		// Compute relative path
		relPath, err := filepath.Rel(publicBaseDir, path)
		if err != nil {
			return nil // Skip on error
		}

		// Defensive check: reject paths that escape (should never happen if rooted)
		if strings.HasPrefix(relPath, "..") {
			return nil
		}

		// Convert to forward slashes for consistent API output
		relPath = filepath.ToSlash(relPath)

		files = append(files, relPath)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort lexicographically for deterministic output
	sort.Strings(files)

	return files, nil
}
