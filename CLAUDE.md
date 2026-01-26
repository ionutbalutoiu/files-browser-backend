# CLAUDE.md

**Read `.claude/rules/golang-style.md` before making code changes.**

## Overview

Go HTTP microservice for file management (upload, delete, mkdir, rename, public sharing). Pure stdlib, runs behind Nginx.

## Commands

```bash
make build          # Build to build/files-svc
make run            # Run on :8080 with /tmp/files
make fmt && make lint
go test ./...       # All tests
go test -v ./internal/api/ -run TestUpload  # Single test
```

## Go Conventions

- `gofmt`, `go vet` always. Run `make fmt` before commits.
- Early returns, guard clauses. No deep nesting.
- Wrap errors with context: `fmt.Errorf("save file: %w", err)`
- No panics except init-time programmer errors.
- Small, single-purpose functions.

## Architecture

```text
cmd/files-svc/          → Entry point, CLI flags
internal/config/        → Configuration
internal/server/        → HTTP server, routes
internal/api/           → Request handlers
  files/                → Upload, delete handlers
  files/actions/        → Move, rename handlers
  folders/              → Create folder handler
  publicshares/         → Public share handlers
  health/               → Health check handler
internal/service/       → Filesystem ops (file/dir CRUD, public sharing)
internal/pathutil/      → Path validation (security-critical)
internal/httputil/      → HTTP response helpers (ErrorResponse, JSONResponse)
configs/                → Sample configuration files
```

**Handler pattern:** Structs with `Config` field implementing `http.Handler`. Flow: Parse/validate input → call service → map errors → encode JSON.

```go
type FooHandler struct { Config config.Config }
func NewFooHandler(cfg config.Config) *FooHandler { return &FooHandler{Config: cfg} }
func (h *FooHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { ... }
```

**Path validation:** `pathutil/` contains `Resolve*` and `Validate*` functions for secure path handling.

**Errors:** `PathError` (with HTTP status), `FileError` (with conflict flag). Use `errors.As()`.

## HTTP Status Codes

| Code | Use |
|------|-----|
| 200 | Success (GET) |
| 201 | Created (POST upload/mkdir/share) |
| 204 | Deleted |
| 400 | Invalid path, validation error |
| 404 | Not found |
| 409 | Conflict (file exists) |

## Security

- Path traversal blocked (`../`, absolute paths, null bytes)
- Symlinks rejected (uses `Lstat`)
- No overwrites (`O_EXCL`, returns 409)
- Hidden files (`.` prefix) rejected
- Public sharing: regular files only

## Testing

- Use `httptest.NewRecorder()` for handler tests.
- Use `t.TempDir()` for filesystem tests.
- Test both success and error paths.

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/healthz` | Health check |
| PUT | `/api/files?path=` | Upload files |
| POST | `/api/folders` | Create directory |
| DELETE | `/api/files?path=` | Delete file/folder |
| POST | `/api/files/move` | Move item |
| POST | `/api/files/rename` | Rename item |
| GET | `/api/public-shares` | List public shares |
| POST | `/api/public-shares` | Create public share |
| DELETE | `/api/public-shares?path=` | Delete public share |

## Adding New Endpoints

1. Create handler in `internal/api/<resource>/handler.go`
2. Add service function in `internal/service/` if needed
3. Add path validator in `internal/pathutil/` if needed
4. Register route in `internal/server/server.go`
5. Use `httputil.ErrorResponse`/`JSONResponse` for responses
6. Update API docs in `docs/api.md`

## Breaking Changes

When making changes that break backwards compatibility (API changes, removed endpoints, changed request/response formats):

1. Update `docs/api.md` with new API specification
2. Update `CLAUDE.md` API section
3. Update `README.md` if affected
