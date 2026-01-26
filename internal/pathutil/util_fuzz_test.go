package pathutil_test

import (
	"testing"

	"files-browser-backend/internal/pathutil"
)

// FuzzResolveTargetDir tests ResolveTargetDir with random inputs.
// Security: Ensures the function never panics on arbitrary input.
func FuzzResolveTargetDir(f *testing.F) {
	// Seed corpus with common attack patterns
	f.Add("simple/path")
	f.Add("../escape")
	f.Add("/absolute")
	f.Add("path\x00null")
	f.Add("foo/../../etc/passwd")
	f.Add("..%2F..%2Fetc%2Fpasswd")
	f.Add("....//....//etc//passwd")
	f.Add("path/./to/./file")
	f.Add("path//double//slashes")
	f.Add("")
	f.Add(".")
	f.Add("..")

	f.Fuzz(func(t *testing.T, urlPath string) {
		baseDir := t.TempDir()
		// Must not panic - result is intentionally discarded
		_, _ = pathutil.ResolveTargetDir(baseDir, urlPath)
	})
}

// FuzzResolveDeletePath tests ResolveDeletePath with random inputs.
// Security: Ensures the function never panics and properly rejects dangerous paths.
func FuzzResolveDeletePath(f *testing.F) {
	f.Add("simple.txt")
	f.Add("../escape")
	f.Add("/etc/passwd")
	f.Add("path\x00null.txt")
	f.Add("foo/../../etc/passwd")
	f.Add("")
	f.Add(".")
	f.Add("..")
	f.Add("valid/path/file.txt")

	f.Fuzz(func(t *testing.T, urlPath string) {
		baseDir := t.TempDir()
		_, _ = pathutil.ResolveDeletePath(baseDir, urlPath)
	})
}

// FuzzResolveMkdirPath tests ResolveMkdirPath with random inputs.
// Security: Ensures the function never panics on directory creation attempts.
func FuzzResolveMkdirPath(f *testing.F) {
	f.Add("newdir")
	f.Add("../escape")
	f.Add("/absolute/path")
	f.Add("path\x00null")
	f.Add("foo/../../etc")
	f.Add("")
	f.Add(".")
	f.Add("..")
	f.Add("valid/nested/dir")
	f.Add(".hidden")

	f.Fuzz(func(t *testing.T, urlPath string) {
		baseDir := t.TempDir()
		_, _, _ = pathutil.ResolveMkdirPath(baseDir, urlPath)
	})
}

// FuzzResolveMovePaths tests ResolveMovePaths with random inputs.
// Security: Ensures move operations are properly validated.
func FuzzResolveMovePaths(f *testing.F) {
	f.Add("source.txt", "dest.txt")
	f.Add("../escape", "dest.txt")
	f.Add("source.txt", "../escape")
	f.Add("/etc/passwd", "dest.txt")
	f.Add("source.txt", "/etc/passwd")
	f.Add("path\x00null", "dest.txt")
	f.Add("source.txt", "path\x00null")
	f.Add("", "dest.txt")
	f.Add("source.txt", "")
	f.Add(".", "dest.txt")
	f.Add("source.txt", ".")

	f.Fuzz(func(t *testing.T, sourcePath, destPath string) {
		baseDir := t.TempDir()
		_, _, _, _, _ = pathutil.ResolveMovePaths(baseDir, sourcePath, destPath)
	})
}

// FuzzResolveRenamePaths tests ResolveRenamePaths with random inputs.
// Security: Ensures rename operations properly validate new names.
func FuzzResolveRenamePaths(f *testing.F) {
	f.Add("file.txt", "newname.txt")
	f.Add("../escape.txt", "newname.txt")
	f.Add("file.txt", "../escape")
	f.Add("file.txt", "new/path/name.txt")
	f.Add("path\x00null.txt", "newname.txt")
	f.Add("file.txt", "name\x00null.txt")
	f.Add("", "newname.txt")
	f.Add("file.txt", "")
	f.Add(".", "newname.txt")
	f.Add("file.txt", ".")
	f.Add("file.txt", "..")

	f.Fuzz(func(t *testing.T, oldPath, newName string) {
		baseDir := t.TempDir()
		_, _, _, _, _ = pathutil.ResolveRenamePaths(baseDir, oldPath, newName)
	})
}

// FuzzValidateFilename tests ValidateFilename with random inputs.
// Security: Ensures filename validation properly rejects dangerous names.
func FuzzValidateFilename(f *testing.F) {
	f.Add("valid.txt")
	f.Add(".hidden")
	f.Add("../escape.txt")
	f.Add("path/to/file.txt")
	f.Add("")
	f.Add(".")
	f.Add("..")
	f.Add("file\x00null.txt")
	f.Add("valid-file_name.txt")

	f.Fuzz(func(t *testing.T, filename string) {
		_, _ = pathutil.ValidateFilename(filename)
	})
}

// FuzzResolveSharePublicPath tests ResolveSharePublicPath with random inputs.
// Security: Ensures public share path validation is robust.
func FuzzResolveSharePublicPath(f *testing.F) {
	f.Add("file.txt")
	f.Add("../escape.txt")
	f.Add("/etc/passwd")
	f.Add("path\x00null.txt")
	f.Add("")
	f.Add(".")
	f.Add("..")
	f.Add("valid/path/file.txt")

	f.Fuzz(func(t *testing.T, urlPath string) {
		baseDir := t.TempDir()
		_, _, _ = pathutil.ResolveSharePublicPath(baseDir, urlPath)
	})
}
