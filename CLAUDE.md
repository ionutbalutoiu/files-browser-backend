# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**Important:** Before making any code changes, read `.claude/rules/backend.md` for Go backend development conventions and standards.

## Project Overview

Go HTTP microservice for secure file management. Provides REST API for file uploads, deletions, directory creation, renaming, and public sharing. Designed to run behind Nginx. Zero external dependencies - pure Go standard library only.

## Build and Run Commands

```bash
# Build
go build -o files-svc ./cmd/files-svc
make build                    # Creates build/files-svc

# Run locally (requires /tmp/files directory)
make run                      # Runs on :8080 with /tmp/files as base

# Run with custom config
./files-svc -base-dir /srv/files -listen :9000 -max-upload-size 104857600

# Environment variables
FILES_SVC_UPLOAD_BASE_DIR=/var/www/files ./files-svc
FILES_SVC_PUBLIC_BASE_DIR=/srv/public ./files-svc
FILES_SVC_MAX_UPLOAD_SIZE=104857600 ./files-svc
```

## Testing Commands

```bash
go test ./...                           # Run all tests
go test -v ./...                        # Verbose output
go test -v ./internal/handlers/...      # Single package
go test -v ./internal/handlers/ -run TestUpload  # Single test
make coverage                           # Generate coverage.html
```

## Linting and Formatting

```bash
make fmt                      # Format code
make lint                     # Run golangci-lint
```

## Docker

```bash
make docker                   # Build image
make docker-run               # Run container on port 8080
```

## Architecture

### Layered Structure
- **cmd/files-svc/main.go** - Entry point, CLI flags, server startup
- **internal/config/** - Configuration loading and validation
- **internal/server/** - HTTP server setup, route registration
- **internal/handlers/** - HTTP request handlers (one file per endpoint)
- **internal/fs/** - Filesystem operations (SaveFile, Delete, Mkdir, SharePublic)
- **internal/pathutil/** - Security-critical path validation

### Handler Pattern
Each handler is a struct implementing `http.Handler`:
```go
type UploadHandler struct {
    Config config.Config
}
func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

### Path Validation Functions (internal/pathutil/util.go)
- `ResolveTargetDir()` - For uploads
- `ResolveDeletePath()` - For deletions (uses Lstat, no symlink follow)
- `ResolveMkdirPath()` - For directory creation
- `ResolveRenamePaths()` - For renames with overwrite prevention
- `ResolveSharePublicPath()` - For public sharing (files only)

### Error Types
- `PathError` - Path-related errors with HTTP status codes
- `FileError` - File operation errors with conflict flag
- Use `errors.As(err, &pathErr)` for type assertion

### Response Helpers
- `JSONResponse(w, status, data)` - Success responses
- `ErrorResponse(w, status, message)` - Error responses

## Security Properties

1. **Path Traversal Prevention** - Rejects `../`, absolute paths, null bytes
2. **Symlink Rejection** - Uses `os.Lstat()` (doesn't follow symlinks)
3. **No Overwrites** - Uses `os.O_EXCL`, returns HTTP 409 if file exists
4. **Hidden Files Rejected** - Files starting with `.` are rejected
5. **Type Enforcement** - Public sharing only allows regular files, not directories

## API Endpoints

All endpoints under `/api/`:
- `POST /api/upload/<path>/` - Upload files (multipart)
- `DELETE /api/delete/<path>` - Delete file or empty directory
- `POST /api/mkdir/<path>/` - Create directory
- `POST|PATCH /api/rename/<path>?newName=<name>` - Rename
- `POST /api/share-public/<path>` - Create public share symlink
- `GET /api/share-public-files/<path>` - List public shares
- `DELETE /api/share-public-delete` - Delete public share
- `GET /api/health` - Health check

## Key Files by Task

| Task | Files |
|------|-------|
| Add new endpoint | `internal/handlers/*.go`, `internal/server/server.go` |
| Modify path validation | `internal/pathutil/util.go` |
| Change filesystem operations | `internal/fs/ops.go` |
| Update configuration | `internal/config/config.go` |
