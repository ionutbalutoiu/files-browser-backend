# AGENTS.md

Primary contributor guide for `files-browser-backend`.

## 1. Project Overview

Go HTTP microservice for file management:
- Upload files.
- Delete files and empty directories.
- Create directories.
- Move and rename files/directories.
- Manage public shares through symlinks.

Design goals:
- Security-first path handling.
- Predictable API contracts.
- Minimal dependencies (stdlib-first).
- Maintainable, testable code.

## 2. Key Commands

```bash
make build
make run
make fmt
go vet ./...
go test ./...
go test -race ./...
```

## 3. Architecture

```text
cmd/files-svc/          Entry point, CLI flags
internal/config/        Configuration and validation
internal/server/        HTTP server lifecycle and graceful shutdown
internal/api/           HTTP handlers
  files/                Upload and delete
  files/actions/        Move and rename
  folders/              Create folder
  publicshares/         Public share endpoints
  health/               Health endpoint
internal/service/       Filesystem operations
internal/pathutil/      Security-critical path validation/resolution
internal/httputil/      Shared HTTP JSON/error helpers
docs/                   API documentation
```

Handler pattern:
- Handler structs include `Config`.
- `NewXHandler(cfg)` constructors.
- `ServeHTTP` parses input, calls service/pathutil, maps errors, writes JSON.

## 4. Non-Negotiable Runtime and API Invariants

### Upload behavior
- Uploads are processed as a multipart stream (`MultipartReader`), not via full form buffering.
- Single pass over multipart file parts.
- Existing file check happens before save attempt.
- Existing-file conflicts are reported in `skipped` only, not in `errors`.
- Partial success is allowed:
  - `201` when at least one file is uploaded.
  - `409` when nothing uploaded and at least one file is skipped.
  - `400` for validation/processing errors.
  - `413` when max upload size is exceeded.
- Non-file multipart parts are ignored.

### Filesystem safety
- No overwrites: destination creation uses exclusive semantics (`O_EXCL`).
- Path traversal blocked (`..`, absolute paths, null bytes).
- Symlink-sensitive operations use `Lstat` where required.
- Hidden files (`.` prefix) are rejected on upload.

### Server hardening
- `ReadHeaderTimeout` is set.
- `MaxHeaderBytes` is set.
- Graceful shutdown on `SIGINT`/`SIGTERM` using context-driven signal handling.
- Keep upload-friendly semantics: do not introduce restrictive read/write timeouts without explicit decision.

### Config validation
- `ListenAddr` must be non-empty.
- `MaxUploadSize` must be `> 0`.
- Base/public directories must resolve correctly with existing behavior preserved.

## 5. Go Coding Standards

### Core principles (priority order)
1. Clarity
2. Simplicity
3. Concision
4. Maintainability
5. Consistency
6. Avoid duplication

### Control flow
- Keep happy path straight-line.
- Use guard clauses and early returns.
- Avoid deep nesting.

### Error handling
- No panics in production flow.
- Wrap errors with context using `%w` when callers may inspect cause.
- Keep error messages lowercase, no trailing punctuation.
- Handle errors once (log or return, not both).

### Naming and style
- MixedCaps; no underscores.
- Preserve initialism casing (`ID`, `URL`, etc.).
- Avoid shadowing predeclared identifiers.
- Comments are full sentences ending with a period.
- Keep lines reasonably short (target <= 120 chars).

### Imports
- Group in order:
  1) Standard library
  2) External
  3) Internal

### Resource management
- Use `defer` for cleanup.
- `context.Context` is first parameter (`ctx`).
- Do not store context in structs.

### Concurrency
- Default synchronous.
- Keep goroutine lifecycles explicit and bounded.
- Prefer simple channel usage and context cancellation.

## 6. Testing Standards

- Use table-driven tests when it improves coverage/readability.
- Use `httptest` for handler tests.
- Use `t.TempDir()` for filesystem tests.
- Test success and failure paths.
- Include expected and actual values in failures.
- Run `go test ./...` and `go test -race ./...` for significant changes.

Required coverage for upload/config/server changes:
- Upload conflict and skipped behavior.
- Partial success scenarios.
- Upload size limit (`413`) behavior.
- Filename/path validation behavior.
- Config validation (`ListenAddr`, `MaxUploadSize`).
- Server defaults (`ReadHeaderTimeout`, `MaxHeaderBytes`).

## 7. API and Documentation Policy

- Preserve external API compatibility unless explicitly requested.
- When behavior or contract changes:
  - Update `docs/api.md`.
  - Update `README.md` if user-visible runtime behavior changes.
  - Add/adjust tests in the same change.

## 8. Change Checklist (Before Merge)

1. Code formatted (`make fmt` or `gofmt`).
2. Static checks pass (`go vet ./...`).
3. Tests pass (`go test ./...`).
4. Race tests pass for non-trivial changes (`go test -race ./...`).
5. Security-critical path logic reviewed for traversal/symlink regressions.
6. Docs updated for any behavior/contract changes.

## 9. Dependency Policy

- Prefer stdlib-first solutions.
- Add external dependencies only when benefit is clear and maintenance cost is justified.
- Keep dependency surface minimal.
