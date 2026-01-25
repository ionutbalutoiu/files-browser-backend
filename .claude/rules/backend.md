---
name: backend
description: Go backend rules for files-browser-backend
---

## Project Structure

```
cmd/files-svc/     → main(), CLI flags
internal/
  api/             → HTTP handlers (one per endpoint)
  service/         → Filesystem operations
  pathutil/        → Path validation (security-critical)
  config/          → Configuration from env
  server/          → HTTP server, routing
configs/           → Sample .env files
```

## Go Conventions

- `gofmt`, `go vet` always. Run `make fmt` before commits.
- Early returns, guard clauses. No deep nesting.
- Wrap errors with context: `fmt.Errorf("save file: %w", err)`
- No panics except init-time programmer errors.
- Small, single-purpose functions.

## Handler Pattern

Handlers in `internal/api/`:
- Struct with `Config` field implementing `http.Handler`
- Parse/validate input → call service → map errors → encode JSON

```go
type FooHandler struct { Config config.Config }
func NewFooHandler(cfg config.Config) *FooHandler { return &FooHandler{Config: cfg} }
func (h *FooHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { ... }
```

## Error Types

- `pathutil.PathError` - path validation errors with HTTP status
- `service.FileError` - filesystem errors with conflict flag
- Use `errors.As()` for type checking

## HTTP Status Codes

| Code | Use |
|------|-----|
| 200 | Success (GET) |
| 201 | Created (POST upload/mkdir/share) |
| 204 | Deleted |
| 400 | Invalid path, validation error |
| 404 | Not found |
| 409 | Conflict (file exists) |

## Security Rules (Non-Negotiable)

- Block path traversal: `../`, absolute paths, null bytes
- Reject symlinks (use `os.Lstat`)
- No overwrites (`os.O_EXCL`, return 409)
- Reject hidden files (`.` prefix)
- Public sharing: symlinks to regular files only

## Testing

- Use `httptest.NewRecorder()` for handler tests
- Use `t.TempDir()` for filesystem tests
- Test both success and error paths
- Run: `go test ./...`

## Adding New Endpoints

1. Create handler in `internal/api/foo.go`
2. Add service function in `internal/service/` if needed
3. Add path validator in `internal/pathutil/` if needed
4. Register route in `internal/server/server.go`
5. Update API docs in `CLAUDE.md`
