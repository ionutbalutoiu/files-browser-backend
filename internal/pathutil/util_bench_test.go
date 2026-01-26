package pathutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"files-browser-backend/internal/pathutil"
)

// BenchmarkResolveTargetDir measures path resolution performance for uploads.
func BenchmarkResolveTargetDir(b *testing.B) {
	baseDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pathutil.ResolveTargetDir(baseDir, "photos/2026/vacation")
	}
}

// BenchmarkResolveTargetDir_Deep measures resolution with deeply nested paths.
func BenchmarkResolveTargetDir_Deep(b *testing.B) {
	baseDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pathutil.ResolveTargetDir(baseDir, "a/b/c/d/e/f/g/h/i/j")
	}
}

// BenchmarkResolveDeletePath measures path resolution for deletion.
func BenchmarkResolveDeletePath(b *testing.B) {
	baseDir := b.TempDir()
	// Create a file to delete
	testFile := filepath.Join(baseDir, "testfile.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pathutil.ResolveDeletePath(baseDir, "testfile.txt")
	}
}

// BenchmarkResolveDeletePath_Nested measures deletion resolution with nested paths.
func BenchmarkResolveDeletePath_Nested(b *testing.B) {
	baseDir := b.TempDir()
	// Create nested structure
	nestedDir := filepath.Join(baseDir, "a", "b", "c")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		b.Fatal(err)
	}
	testFile := filepath.Join(nestedDir, "file.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pathutil.ResolveDeletePath(baseDir, "a/b/c/file.txt")
	}
}

// BenchmarkResolveMkdirPath measures path resolution for directory creation.
func BenchmarkResolveMkdirPath(b *testing.B) {
	baseDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = pathutil.ResolveMkdirPath(baseDir, "newdir")
	}
}

// BenchmarkResolveMkdirPath_Nested measures mkdir resolution with nested paths.
func BenchmarkResolveMkdirPath_Nested(b *testing.B) {
	baseDir := b.TempDir()
	// Create parent structure
	parentDir := filepath.Join(baseDir, "parent", "child")
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = pathutil.ResolveMkdirPath(baseDir, "parent/child/newdir")
	}
}

// BenchmarkResolveMovePaths measures path resolution for move operations.
func BenchmarkResolveMovePaths(b *testing.B) {
	baseDir := b.TempDir()
	// Create source file
	srcFile := filepath.Join(baseDir, "source.txt")
	if err := os.WriteFile(srcFile, []byte("content"), 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _ = pathutil.ResolveMovePaths(baseDir, "source.txt", "dest.txt")
	}
}

// BenchmarkResolveMovePaths_CrossDir measures move resolution across directories.
func BenchmarkResolveMovePaths_CrossDir(b *testing.B) {
	baseDir := b.TempDir()
	// Create structure
	srcDir := filepath.Join(baseDir, "srcdir")
	dstDir := filepath.Join(baseDir, "dstdir")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		b.Fatal(err)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		b.Fatal(err)
	}
	srcFile := filepath.Join(srcDir, "file.txt")
	if err := os.WriteFile(srcFile, []byte("content"), 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _ = pathutil.ResolveMovePaths(baseDir, "srcdir/file.txt", "dstdir/file.txt")
	}
}

// BenchmarkValidateFilename measures filename validation performance.
func BenchmarkValidateFilename(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pathutil.ValidateFilename("my-document-2026.pdf")
	}
}

// BenchmarkValidateFilename_Path measures validation with path in filename.
func BenchmarkValidateFilename_Path(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pathutil.ValidateFilename("path/to/my-document-2026.pdf")
	}
}

// BenchmarkResolveSharePublicPath measures public share path resolution.
func BenchmarkResolveSharePublicPath(b *testing.B) {
	baseDir := b.TempDir()
	// Create a file to share
	testFile := filepath.Join(baseDir, "shareable.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = pathutil.ResolveSharePublicPath(baseDir, "shareable.txt")
	}
}
