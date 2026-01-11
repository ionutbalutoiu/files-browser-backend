// Package main implements a secure file upload and delete service for use behind Nginx.
// It receives multipart/form-data uploads and DELETE requests, operating on files
// within the configured base directory.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Config holds the service configuration.
type Config struct {
	ListenAddr    string
	BaseDir       string
	MaxUploadSize int64
	UploadPrefix  string
	DeletePrefix  string
}

// UploadResponse is the JSON response for upload requests.
type UploadResponse struct {
	Uploaded []string `json:"uploaded"`
	Skipped  []string `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

// Server handles file upload requests.
type Server struct {
	config Config
}

// NewServer creates a new upload server with the given configuration.
func NewServer(cfg Config) (*Server, error) {
	// Resolve and validate base directory
	absBase, err := filepath.Abs(cfg.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("invalid base directory: %w", err)
	}

	// Ensure base directory exists
	info, err := os.Stat(absBase)
	if err != nil {
		return nil, fmt.Errorf("base directory error: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("base path is not a directory: %s", absBase)
	}

	cfg.BaseDir = absBase
	return &Server{config: cfg}, nil
}

// HandleUpload handles file upload requests (POST /upload/<path>/).
func (s *Server) HandleUpload(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		s.errorResponse(w, http.StatusMethodNotAllowed, "only POST method is allowed")
		return
	}

	// Check Content-Type
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		s.errorResponse(w, http.StatusBadRequest, "Content-Type must be multipart/form-data")
		return
	}

	// Extract target path from URL
	targetPath := strings.TrimPrefix(r.URL.Path, s.config.UploadPrefix)
	targetPath = strings.TrimPrefix(targetPath, "/")

	// Resolve and validate target directory
	targetDir, err := s.resolveTargetDir(targetPath)
	if err != nil {
		var pathErr *PathError
		if errors.As(err, &pathErr) {
			s.errorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: path resolution failed: %v", err)
		s.errorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Wrap request body with size limit
	r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadSize)

	// Parse multipart form with a small memory buffer (32MB)
	// Files larger than this will be streamed to temp files
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			s.errorResponse(w, http.StatusRequestEntityTooLarge, "upload size exceeds limit")
			return
		}
		log.Printf("ERROR: failed to parse multipart form: %v", err)
		s.errorResponse(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}
	defer r.MultipartForm.RemoveAll()

	// Process uploaded files
	response := s.processUploads(r.MultipartForm, targetDir)

	// Determine response status
	status := http.StatusCreated
	if len(response.Uploaded) == 0 {
		if len(response.Skipped) > 0 {
			status = http.StatusConflict
		} else if len(response.Errors) > 0 {
			status = http.StatusBadRequest
		}
	}

	s.jsonResponse(w, status, response)
}

// PathError represents a path validation error with HTTP status code.
type PathError struct {
	StatusCode int
	Message    string
}

func (e *PathError) Error() string {
	return e.Message
}

// resolveTargetDir validates and resolves the target directory path.
// It ensures the path is safe and within the base directory.
func (s *Server) resolveTargetDir(urlPath string) (string, error) {
	// Clean the path to remove any . or .. components
	cleanPath := filepath.Clean(urlPath)

	// Reject paths that try to escape using ..
	if strings.Contains(cleanPath, "..") {
		return "", &PathError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid path: contains parent directory reference",
		}
	}

	// Reject absolute paths
	if filepath.IsAbs(cleanPath) {
		return "", &PathError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid path: absolute paths not allowed",
		}
	}

	// Construct full target path
	targetDir := filepath.Join(s.config.BaseDir, cleanPath)

	// Resolve any symlinks to get the real path
	realTarget, err := filepath.EvalSymlinks(targetDir)
	if err != nil {
		// If path doesn't exist, check parent directory
		if os.IsNotExist(err) {
			// Verify the path would still be under base if created
			relPath, relErr := filepath.Rel(s.config.BaseDir, targetDir)
			if relErr != nil || strings.HasPrefix(relPath, "..") {
				return "", &PathError{
					StatusCode: http.StatusBadRequest,
					Message:    "invalid path: escapes base directory",
				}
			}
			return targetDir, nil
		}
		return "", &PathError{
			StatusCode: http.StatusNotFound,
			Message:    "invalid target path",
		}
	}

	// Verify resolved path is within base directory
	relPath, err := filepath.Rel(s.config.BaseDir, realTarget)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", &PathError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid path: escapes base directory",
		}
	}

	return realTarget, nil
}

// processUploads handles all files in the multipart form.
func (s *Server) processUploads(form *multipart.Form, targetDir string) UploadResponse {
	response := UploadResponse{
		Uploaded: []string{},
		Skipped:  []string{},
		Errors:   []string{},
	}

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		log.Printf("ERROR: failed to create target directory %s: %v", targetDir, err)
		response.Errors = append(response.Errors, "failed to create target directory")
		return response
	}

	// Process all file fields
	for fieldName, files := range form.File {
		for _, fileHeader := range files {
			err := s.saveFile(fileHeader, targetDir)
			if err != nil {
				var fileErr *FileError
				if errors.As(err, &fileErr) {
					if fileErr.IsConflict {
						response.Skipped = append(response.Skipped, fileHeader.Filename)
						log.Printf("SKIP: file %s already exists (field: %s)", fileHeader.Filename, fieldName)
					} else {
						response.Errors = append(response.Errors, fmt.Sprintf("%s: %s", fileHeader.Filename, fileErr.Message))
						log.Printf("ERROR: file %s (field: %s): %s", fileHeader.Filename, fieldName, fileErr.Message)
					}
				} else {
					response.Errors = append(response.Errors, fmt.Sprintf("%s: internal error", fileHeader.Filename))
					log.Printf("ERROR: file %s (field: %s): %v", fileHeader.Filename, fieldName, err)
				}
				continue
			}
			response.Uploaded = append(response.Uploaded, fileHeader.Filename)
			log.Printf("OK: uploaded %s to %s (field: %s)", fileHeader.Filename, targetDir, fieldName)
		}
	}

	// Clear errors array if empty for cleaner JSON
	if len(response.Errors) == 0 {
		response.Errors = nil
	}

	return response
}

// FileError represents a file processing error.
type FileError struct {
	Message    string
	IsConflict bool
}

func (e *FileError) Error() string {
	return e.Message
}

// saveFile saves a single uploaded file to the target directory.
func (s *Server) saveFile(fh *multipart.FileHeader, targetDir string) error {
	// Validate filename
	filename := filepath.Base(fh.Filename)

	// Reject empty filenames
	if filename == "" || filename == "." || filename == ".." {
		return &FileError{Message: "invalid filename"}
	}

	// Reject filenames with path separators (extra safety)
	if strings.ContainsAny(fh.Filename, "/\\") && filename != fh.Filename {
		// Client sent a path, we'll just use the base name
		log.Printf("WARN: stripped path from filename: %s -> %s", fh.Filename, filename)
	}

	// Reject hidden files (starting with .)
	if strings.HasPrefix(filename, ".") {
		return &FileError{Message: "hidden files not allowed"}
	}

	// Construct destination path
	destPath := filepath.Join(targetDir, filename)

	// Final safety check: ensure destination is within base directory
	realBase, _ := filepath.EvalSymlinks(s.config.BaseDir)
	if realBase == "" {
		realBase = s.config.BaseDir
	}

	// Check if destination would escape base (handles edge cases)
	relPath, err := filepath.Rel(realBase, destPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
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

// errorResponse sends a JSON error response.
func (s *Server) errorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// jsonResponse sends a JSON response.
func (s *Server) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// HandleDelete handles file/directory deletion requests (DELETE /delete/<path>).
// Security: Uses Lstat to avoid following symlinks, validates path is strictly
// within base directory, and refuses to delete the base directory itself.
func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	// Only allow DELETE method
	if r.Method != http.MethodDelete {
		s.errorResponse(w, http.StatusMethodNotAllowed, "only DELETE method is allowed")
		return
	}

	// Extract target path from URL
	targetPath := strings.TrimPrefix(r.URL.Path, s.config.DeletePrefix)
	targetPath = strings.TrimPrefix(targetPath, "/")
	targetPath = strings.TrimSuffix(targetPath, "/") // Normalize trailing slash

	// Resolve and validate target path for deletion
	resolvedPath, err := s.resolveDeletePath(targetPath)
	if err != nil {
		var pathErr *PathError
		if errors.As(err, &pathErr) {
			s.errorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: delete path resolution failed: %v", err)
		s.errorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Perform deletion
	if err := s.performDelete(resolvedPath); err != nil {
		var pathErr *PathError
		if errors.As(err, &pathErr) {
			s.errorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: deletion failed for %s: %v", resolvedPath, err)
		s.errorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	log.Printf("OK: deleted %s", resolvedPath)
	w.WriteHeader(http.StatusNoContent)
}

// resolveDeletePath validates and resolves a path for deletion.
// SECURITY CRITICAL: This function prevents path traversal and symlink escape.
// It uses Lstat (not Stat) to avoid following symlinks.
func (s *Server) resolveDeletePath(urlPath string) (string, error) {
	// Reject empty path (would delete base directory)
	if urlPath == "" || urlPath == "." {
		return "", &PathError{
			StatusCode: http.StatusForbidden,
			Message:    "cannot delete base directory",
		}
	}

	// Clean the path to normalize . and .. components
	cleanPath := filepath.Clean(urlPath)

	// Reject paths containing .. after cleaning
	// filepath.Clean resolves .., so if it still contains .., something is wrong
	if strings.Contains(cleanPath, "..") {
		return "", &PathError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid path: contains parent directory reference",
		}
	}

	// Reject absolute paths
	if filepath.IsAbs(cleanPath) {
		return "", &PathError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid path: absolute paths not allowed",
		}
	}

	// Construct full target path
	targetPath := filepath.Join(s.config.BaseDir, cleanPath)

	// CRITICAL: Verify the target is strictly within base directory
	// Use filepath.Rel to check containment
	relPath, err := filepath.Rel(s.config.BaseDir, targetPath)
	if err != nil || strings.HasPrefix(relPath, "..") || relPath == "." {
		return "", &PathError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid path: escapes base directory",
		}
	}

	// Use Lstat to check if path exists WITHOUT following symlinks
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &PathError{
				StatusCode: http.StatusNotFound,
				Message:    "path does not exist",
			}
		}
		return "", fmt.Errorf("failed to stat path: %w", err)
	}

	// SECURITY: Reject symlinks entirely to prevent escape attacks
	if info.Mode()&os.ModeSymlink != 0 {
		return "", &PathError{
			StatusCode: http.StatusBadRequest,
			Message:    "cannot delete symlinks",
		}
	}

	return targetPath, nil
}

// performDelete deletes a file or empty directory.
// For directories, it verifies they are empty before deletion.
func (s *Server) performDelete(targetPath string) error {
	// Get file info (already validated, but need to check type)
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &PathError{
				StatusCode: http.StatusNotFound,
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
			return &PathError{
				StatusCode: http.StatusConflict,
				Message:    "directory is not empty",
			}
		}
	}

	// Perform the deletion (works for both files and empty directories)
	if err := os.Remove(targetPath); err != nil {
		if os.IsNotExist(err) {
			// Race condition: file was deleted between check and remove
			return &PathError{
				StatusCode: http.StatusNotFound,
				Message:    "path does not exist",
			}
		}
		if os.IsPermission(err) {
			return &PathError{
				StatusCode: http.StatusForbidden,
				Message:    "permission denied",
			}
		}
		return err
	}

	return nil
}

func main() {
	// Determine default base directory from environment or fallback
	defaultBaseDir := os.Getenv("UPLOAD_BASE_DIR")
	if defaultBaseDir == "" {
		defaultBaseDir = "/srv/files"
	}

	// Parse command-line flags (flags take priority over env vars)
	listenAddr := flag.String("listen", ":8080", "Address to listen on")
	baseDir := flag.String("base-dir", defaultBaseDir, "Base directory for file storage (env: UPLOAD_BASE_DIR)")
	maxSize := flag.Int64("max-size", 2*1024*1024*1024, "Maximum upload size in bytes (default: 2GB)")
	uploadPrefix := flag.String("upload-prefix", "/upload", "URL prefix for upload endpoint")
	deletePrefix := flag.String("delete-prefix", "/delete", "URL prefix for delete endpoint")
	flag.Parse()

	// Create server
	cfg := Config{
		ListenAddr:    *listenAddr,
		BaseDir:       *baseDir,
		MaxUploadSize: *maxSize,
		UploadPrefix:  *uploadPrefix,
		DeletePrefix:  *deletePrefix,
	}

	server, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc(cfg.UploadPrefix+"/", server.HandleUpload)
	mux.HandleFunc(cfg.DeletePrefix+"/", server.HandleDelete)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	httpServer := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  0, // No timeout for large uploads
		WriteTimeout: 0, // No timeout for large uploads
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown handling
	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		httpServer.SetKeepAlivesEnabled(false)
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Fatalf("Could not gracefully shutdown the server: %v", err)
		}
		close(done)
	}()

	log.Printf("File server starting on %s", cfg.ListenAddr)
	log.Printf("Base directory: %s", cfg.BaseDir)
	log.Printf("Max upload size: %d bytes (%.2f GB)", cfg.MaxUploadSize, float64(cfg.MaxUploadSize)/(1024*1024*1024))
	log.Printf("Upload prefix: %s", cfg.UploadPrefix)
	log.Printf("Delete prefix: %s", cfg.DeletePrefix)

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	<-done
	log.Println("Server stopped")
}
