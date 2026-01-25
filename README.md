# files-svc

A secure, minimal Go service for handling file uploads, deletions, and directory creation via HTTP. Designed to run behind Nginx or another reverse proxy.

## Features

- **Streaming uploads** - files are streamed to disk, not buffered in memory
- **Multiple file support** - upload several files in a single request
- **File deletion** - delete files or empty directories
- **Directory creation** - create new directories via HTTP
- **Rename support** - rename files or directories within the base directory
- **Path safety** - prevents path traversal attacks (`../`, symlinks, absolute paths)
- **No overwrites** - rejects uploads that would overwrite existing files (HTTP 409)
- **Auto-create directories** - target directories for uploads are created automatically
- **Safe writes** - files are synced to disk before success response
- **Graceful shutdown** - in-progress operations complete before shutdown

## Project Structure

```
.
├── cmd/
│   └── files-svc/
│       └── main.go            # Entry point (flags, config, startup)
├── internal/
│   ├── config/                # Configuration loading & defaults
│   │   └── config.go
│   ├── server/                # HTTP server wiring
│   │   └── server.go
│   ├── handlers/              # HTTP handlers
│   │   ├── upload.go
│   │   ├── delete.go
│   │   ├── mkdir.go
│   │   ├── rename.go
│   │   └── health.go
│   ├── fs/                    # Filesystem operations
│   │   └── ops.go
│   └── pathutil/              # Path validation & normalization
│       └── util.go
├── docs/
│   ├── api.md                 # HTTP API documentation
│   ├── deployment.md          # Deployment guide
│   ├── nginx.md               # Nginx integration
│   └── spa-integration.md     # SPA integration guide
├── go.mod
├── go.sum
├── Dockerfile
├── Makefile
└── README.md
```

## Build

```bash
# Build the binary
go build -o files-svc ./cmd/files-svc

# Or with optimizations
go build -ldflags="-s -w" -o files-svc ./cmd/files-svc
```

## Usage

```bash
# Run with defaults (listens on :8080, base dir /srv/files, max 2GB)
./files-svc

# Specify base directory
./files-svc --base-dir /srv/files

# Custom configuration
./files-svc \
  -listen :9000 \
  -base-dir /var/www/files \
  -max-upload-size 104857600

# Using environment variables
FILES_SVC_BASE_DIR=/var/www/files ./files-svc
FILES_SVC_MAX_UPLOAD_SIZE=104857600 ./files-svc

# Show help
./files-svc -help
```

### Command-Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `:8080` | Address and port to listen on |
| `-base-dir` | `/srv/files` | Base directory for file storage (env: `FILES_SVC_BASE_DIR`) |
| `-max-upload-size` | `2147483648` (2GB) | Maximum upload size in bytes (env: `FILES_SVC_MAX_UPLOAD_SIZE`) |

## API Endpoints

See [docs/api.md](docs/api.md) for complete API documentation.

### Quick Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/upload/<path>/` | POST | Upload files to directory |
| `/api/delete/<path>` | DELETE | Delete file or empty directory |
| `/api/mkdir/<path>/` | POST | Create new directory |
| `/api/rename/<path>?newName=<name>` | POST/PATCH | Rename file or directory |
| `/api/health` | GET | Health check |

### Examples

```bash
# Upload a file
curl -X POST -F "file=@photo.jpg" http://localhost:8080/api/upload/photos/2026/

# Delete a file
curl -X DELETE http://localhost:8080/api/delete/photos/2026/photo.jpg

# Create a directory
curl -X POST http://localhost:8080/api/mkdir/photos/2026/

# Rename a file
curl -X POST "http://localhost:8080/api/rename/old.txt?newName=new.txt"

# Health check
curl http://localhost:8080/api/health
```

## Nginx Integration

The service is designed to run behind Nginx. Add this to your Nginx server block:

```nginx
# Maximum upload size (must match or exceed Go service's -max-size)
client_max_body_size 2G;

# Serve static files directly
location /files/ {
    alias /srv/files/;
    autoindex on;
    autoindex_format json;
}

# Upload endpoint - proxy to Go service
location /upload/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_request_buffering off;
    proxy_connect_timeout 300;
    proxy_send_timeout 600;
    proxy_read_timeout 600;
}

# Delete endpoint - proxy to Go service
location /delete/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}

# Mkdir endpoint - proxy to Go service
location /mkdir/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}

# Health check endpoint
location = /health {
    proxy_pass http://127.0.0.1:8080/health;
    proxy_set_header Host $host;
}
```

See [docs/nginx.md](docs/nginx.md) for complete Nginx configuration.

## Security Considerations

### What's Protected

1. **Path Traversal** - `../` sequences and absolute paths are rejected
2. **Symlink Escape** - symlinks that escape the base directory are rejected
3. **Hidden Files** - files starting with `.` are rejected
4. **Empty Filenames** - rejected
5. **Overwrite Prevention** - existing files cannot be overwritten
6. **Size Limits** - enforced at the HTTP layer

### What's NOT Protected (Out of Scope)

- **Authentication** - no auth mechanism; use Nginx or a reverse proxy
- **Authorization** - no per-path permissions
- **Virus Scanning** - files are stored as-is
- **Rate Limiting** - implement at the Nginx layer
- **File Type Validation** - any file type is accepted

## Testing

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for specific package
go test -v ./internal/handlers/...
```

## Deployment

See [docs/deployment.md](docs/deployment.md) for deployment options including:

- systemd service configuration
- Docker deployment
- Security hardening

## License

MIT License - see [LICENSE](LICENSE) for details.
