package files

import (
	"context"
	"errors"
	"fmt"
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
	// Uploaded contains filenames that were successfully uploaded.
	Uploaded []string `json:"uploaded"`
	// Skipped contains filenames that were skipped (e.g., file already exists, no overwrite).
	Skipped []string `json:"skipped"`
	// Errors contains validation or processing error messages, omitted if empty.
	Errors []string `json:"errors,omitempty"`
}

// UploadHandler handles file upload requests.
type UploadHandler struct {
	Config config.Config
}

// NewUploadHandler creates a new files upload handler.
func NewUploadHandler(cfg config.Config) *UploadHandler {
	return &UploadHandler{Config: cfg}
}

// validateContentType checks if the request has the correct Content-Type header.
func validateContentType(r *http.Request) error {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return errors.New("content-type must be multipart/form-data")
	}
	return nil
}

// determineResponseStatus calculates the appropriate HTTP status code based on response.
func determineResponseStatus(resp Response) int {
	if len(resp.Uploaded) > 0 {
		return http.StatusCreated
	}
	if len(resp.Skipped) > 0 {
		return http.StatusConflict
	}
	if len(resp.Errors) > 0 {
		return http.StatusBadRequest
	}
	return http.StatusCreated
}

// ServeHTTP handles PUT /api/files?path=<path> requests.
func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := validateContentType(r); err != nil {
		httputil.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	targetPath := r.URL.Query().Get("path")
	targetDir, err := pathutil.ResolveTargetDir(h.Config.BaseDir, targetPath)
	if err != nil {
		httputil.HandlePathError(w, err, "upload path resolution")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.Config.MaxUploadSize)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			httputil.ErrorResponse(w, http.StatusRequestEntityTooLarge, "upload size exceeds limit")
			return
		}
		httputil.ErrorResponse(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	response := h.processUploads(r.Context(), r.MultipartForm, targetDir)
	httputil.JSONResponse(w, determineResponseStatus(response), response)
}

// processUploads handles all files in the multipart form.
func (h *UploadHandler) processUploads(ctx context.Context, form *multipart.Form, targetDir string) Response {
	response := Response{
		Uploaded: []string{},
		Skipped:  []string{},
		Errors:   []string{},
	}

	if err := service.EnsureDir(ctx, targetDir); err != nil {
		response.Errors = append(response.Errors, "failed to create target directory")
		return response
	}

	for _, files := range form.File {
		for _, fileHeader := range files {
			h.processFile(ctx, fileHeader, targetDir, &response)
		}
	}

	return response
}

// processFile handles a single file upload and updates the response accordingly.
func (h *UploadHandler) processFile(ctx context.Context, fh *multipart.FileHeader, targetDir string, resp *Response) {
	err := service.SaveFile(ctx, fh, targetDir, h.Config.BaseDir)
	if err == nil {
		resp.Uploaded = append(resp.Uploaded, fh.Filename)
		return
	}

	var fileErr *service.FileError
	if errors.As(err, &fileErr) {
		if fileErr.IsConflict {
			resp.Skipped = append(resp.Skipped, fh.Filename)
		} else {
			resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %s", fh.Filename, fileErr.Message))
		}
		return
	}

	resp.Errors = append(resp.Errors, fmt.Sprintf("%s: internal error", fh.Filename))
}
