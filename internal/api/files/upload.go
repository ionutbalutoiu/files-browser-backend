package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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
	reader, err := r.MultipartReader()
	if err != nil {
		httputil.ErrorResponse(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	response, err := h.processUploads(r.Context(), reader, targetDir)
	if err != nil {
		if isUploadSizeExceeded(err) {
			httputil.ErrorResponse(w, http.StatusRequestEntityTooLarge, "upload size exceeds limit")
			return
		}
		httputil.ErrorResponse(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}
	httputil.JSONResponse(w, determineResponseStatus(response), response)
}

// processUploads handles all files in the multipart form.
func (h *UploadHandler) processUploads(ctx context.Context, reader *multipart.Reader, targetDir string) (Response, error) {
	response := Response{
		Uploaded: []string{},
		Skipped:  []string{},
		Errors:   []string{},
	}

	if err := service.EnsureDir(ctx, targetDir); err != nil {
		response.Errors = append(response.Errors, "failed to create target directory")
		return response, nil
	}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return response, err
		}

		filename := part.FileName()
		if filename == "" {
			_ = part.Close()
			continue
		}

		exists, normalizedName, err := h.fileExists(filename, targetDir)
		if err != nil {
			_ = part.Close()
			response.Errors = append(response.Errors, "failed to validate existing files")
			continue
		}
		if exists {
			_ = part.Close()
			response.Skipped = append(response.Skipped, normalizedName)
			continue
		}

		if err := h.processPart(ctx, filename, part, targetDir, &response); err != nil {
			_ = part.Close()
			return response, err
		}
		if err := part.Close(); err != nil {
			return response, err
		}
	}

	return response, nil
}

// fileExists checks whether the destination already exists for a valid upload filename.
// Invalid filenames/destinations are not treated as existence conflicts here and are
// left to SaveStream so existing validation messages stay consistent.
func (h *UploadHandler) fileExists(rawFilename, targetDir string) (bool, string, error) {
	filename, err := pathutil.ValidateFilename(rawFilename)
	if err != nil {
		return false, "", nil
	}

	destPath := filepath.Join(targetDir, filename)
	if err := pathutil.ValidateDestination(h.Config.BaseDir, destPath); err != nil {
		return false, "", nil
	}

	_, err = os.Stat(destPath)
	if err == nil {
		return true, filename, nil
	}
	if os.IsNotExist(err) {
		return false, filename, nil
	}
	return false, "", fmt.Errorf("stat destination %q: %w", filename, err)
}

func isUploadSizeExceeded(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr) || strings.Contains(err.Error(), "request body too large")
}

// processPart handles a single file part and updates the response accordingly.
func (h *UploadHandler) processPart(ctx context.Context, filename string, part *multipart.Part, targetDir string, resp *Response) error {
	err := service.SaveStream(ctx, filename, part, targetDir, h.Config.BaseDir)
	if err == nil {
		resp.Uploaded = append(resp.Uploaded, filename)
		return nil
	}

	var fileErr *service.FileError
	if errors.As(err, &fileErr) {
		if fileErr.IsConflict {
			resp.Skipped = append(resp.Skipped, filename)
		} else {
			resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %s", filename, fileErr.Message))
		}
		return nil
	}

	return err
}
