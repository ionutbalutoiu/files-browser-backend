# CLAUDE.md

**Read `.claude/rules/backend.md` before making code changes.**

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
internal/service/       → Filesystem ops (SaveFile, Delete, Mkdir, SharePublic)
internal/pathutil/      → Path validation (security-critical)
internal/httputil/      → HTTP response helpers (ErrorResponse, JSONResponse)
configs/                → Sample configuration files
```

**Handler pattern:** Structs with `Config` field implementing `http.Handler`.

**Path validators** in `pathutil/util.go`: `ResolveTargetDir`, `ResolveDeletePath`, `ResolveMkdirPath`, `ResolveMovePaths`, `ResolveSharePublicPath`, `ValidateRelativePath`.

**Errors:** `PathError` (with HTTP status), `FileError` (with conflict flag). Use `errors.As()`.

## Security

- Path traversal blocked (`../`, absolute paths, null bytes)
- Symlinks rejected (uses `Lstat`)
- No overwrites (`O_EXCL`, returns 409)
- Hidden files (`.` prefix) rejected
- Public sharing: regular files only

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
