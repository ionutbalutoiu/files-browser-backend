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
go test -v ./internal/handlers/ -run TestUpload  # Single test
```

## Architecture

```text
cmd/files-svc/     → Entry point, CLI flags
internal/config/   → Configuration
internal/server/   → HTTP server, routes
internal/handlers/ → Request handlers (one per endpoint)
internal/fs/       → Filesystem ops (SaveFile, Delete, Mkdir, SharePublic)
internal/pathutil/ → Path validation (security-critical)
```

**Handler pattern:** Structs with `Config` field implementing `http.Handler`.

**Path validators** in `pathutil/util.go`: `ResolveTargetDir`, `ResolveDeletePath`, `ResolveMkdirPath`, `ResolveRenamePaths`, `ResolveSharePublicPath`.

**Errors:** `PathError` (with HTTP status), `FileError` (with conflict flag). Use `errors.As()`.

## Security

- Path traversal blocked (`../`, absolute paths, null bytes)
- Symlinks rejected (uses `Lstat`)
- No overwrites (`O_EXCL`, returns 409)
- Hidden files (`.` prefix) rejected
- Public sharing: regular files only

## API

- `/api/upload/<path>` POST
- `/api/delete/<path>` DELETE
- `/api/mkdir/<path>` POST
- `/api/rename/<path>?newName=` POST|PATCH
- `/api/share-public/<path>` POST
- `/api/share-public-files/<path>` GET
- `/api/share-public-delete` DELETE
- `/api/health` GET
