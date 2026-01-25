# files-svc

Minimal Go HTTP service for file management. Runs behind Nginx.

## Features

- Streaming uploads (not buffered in memory)
- File/directory deletion, creation, move/rename
- Public file sharing via symlinks
- Path traversal protection, no overwrites, safe writes
- Graceful shutdown

## Build & Run

```bash
make build                    # Build to build/files-svc
make run                      # Run on :8080 with /tmp/files

# Custom configuration
./files-svc -listen :9000 -base-dir /srv/files -max-upload-size 2147483648
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `FILES_SVC_BASE_DIR` | `/srv/files` | Base directory for files |
| `FILES_SVC_PUBLIC_BASE_DIR` | (none) | Directory for public shares |
| `FILES_SVC_MAX_UPLOAD_SIZE` | `2147483648` | Max upload size (bytes) |

## API

See [docs/api.md](docs/api.md) for complete API documentation.

## Testing

```bash
go test ./...                 # All tests
make coverage                 # Generate coverage.html
```

## License

MIT
