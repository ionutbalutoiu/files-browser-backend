package pathutil_test

import (
	"errors"
	"os"
	"testing"

	"files-browser-backend/internal/pathutil"
)

func TestMkdirNullByteRejection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pathutil-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Test the ResolveMkdirPath function directly since HTTP layer
	// rejects null bytes before reaching our handler
	_, _, err = pathutil.ResolveMkdirPath(tmpDir, "test\x00evil")
	if err == nil {
		t.Error("null byte in path should be rejected")
	}

	var pathErr *pathutil.PathError
	if !errors.As(err, &pathErr) {
		t.Errorf("expected PathError, got %T", err)
	}
	if pathErr.StatusCode != 400 {
		t.Errorf("expected 400, got %d", pathErr.StatusCode)
	}
}
