package files

import (
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"strings"

	"files-browser-backend/internal/config"
	"files-browser-backend/internal/httputil"
	"files-browser-backend/internal/pathutil"
	"files-browser-backend/internal/service"
)

// Response is the JSON response for file upload requests.
type Response struct {
	Uploaded []string `json:"uploaded"`
	Skipped  []string `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

// UploadHandler handles file upload requests.
type UploadHandler struct {
	Config config.Config
}

// NewUploadHandler creates a new files upload handler.
func NewUploadHandler(cfg config.Config) *UploadHandler {
	return &UploadHandler{Config: cfg}
}

// ServeHTTP handles PUT /api/files?path=<path> requests.
func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check Content-Type
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		httputil.ErrorResponse(w, http.StatusBadRequest, "Content-Type must be multipart/form-data")
		return
	}

	// Extract target path from query parameter (empty = root)
	targetPath := r.URL.Query().Get("path")

	// Resolve and validate target directory
	targetDir, err := pathutil.ResolveTargetDir(h.Config.BaseDir, targetPath)
	if err != nil {
		var pathErr *pathutil.PathError
		if errors.As(err, &pathErr) {
			httputil.ErrorResponse(w, pathErr.StatusCode, pathErr.Message)
			return
		}
		log.Printf("ERROR: path resolution failed: %v", err)
		httputil.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Wrap request body with size limit
	r.Body = http.MaxBytesReader(w, r.Body, h.Config.MaxUploadSize)

	// Parse multipart form with a small memory buffer (32MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			httputil.ErrorResponse(w, http.StatusRequestEntityTooLarge, "upload size exceeds limit")
			return
		}
		log.Printf("ERROR: failed to parse multipart form: %v", err)
		httputil.ErrorResponse(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}
	defer func() {
		if err := r.MultipartForm.RemoveAll(); err != nil {
			log.Printf("WARN: failed to remove multipart form temp files: %v", err)
		}
	}()

	// Process uploaded files
	response := h.processUploads(r.MultipartForm, targetDir)

	// Determine response status
	status := http.StatusCreated
	if len(response.Uploaded) == 0 {
		if len(response.Skipped) > 0 {
			status = http.StatusConflict
		} else if len(response.Errors) > 0 {
			status = http.StatusBadRequest
		}
	}

	httputil.JSONResponse(w, status, response)
}

// processUploads handles all files in the multipart form.
func (h *UploadHandler) processUploads(form *multipart.Form, targetDir string) Response {
	response := Response{
		Uploaded: []string{},
		Skipped:  []string{},
		Errors:   []string{},
	}

	// Ensure target directory exists
	if err := service.EnsureDir(targetDir); err != nil {
		log.Printf("ERROR: failed to create target directory %s: %v", targetDir, err)
		response.Errors = append(response.Errors, "failed to create target directory")
		return response
	}

	// Process all file fields
	for fieldName, files := range form.File {
		for _, fileHeader := range files {
			err := service.SaveFile(fileHeader, targetDir, h.Config.BaseDir)
			if err != nil {
				var fileErr *service.FileError
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
