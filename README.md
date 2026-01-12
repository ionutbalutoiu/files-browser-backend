# File Service

A secure, minimal Go service for handling file uploads, deletions, and directory creation via HTTP, designed to run behind Nginx.

## Features

- **Streaming uploads** - files are streamed to disk, not buffered in memory
- **Multiple file support** - upload several files in a single request
- **File deletion** - delete files or empty directories
- **Directory creation** - create new directories via HTTP
- **Path safety** - prevents path traversal attacks (`../`, symlinks, absolute paths)
- **No overwrites** - rejects uploads that would overwrite existing files (HTTP 409)
- **Auto-create directories** - target directories for uploads are created automatically
- **Safe writes** - files are synced to disk before success response
- **Graceful shutdown** - in-progress operations complete before shutdown

## Build

```bash
# Build the binary
go build -o upload-server .

# Or with optimizations
go build -ldflags="-s -w" -o upload-server .
```

## Usage

```bash
# Run with defaults (listens on :8080, base dir /srv/files, max 2GB)
./upload-server

# Custom configuration
./upload-server \
  -listen :9000 \
  -base-dir /var/www/files \
  -max-size 104857600 \
  -upload-prefix /upload \
  -delete-prefix /delete \
  -mkdir-prefix /mkdir

# Using environment variable for base dir
UPLOAD_BASE_DIR=/var/www/files ./upload-server

# Show help
./upload-server -help
```

### Command-Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `:8080` | Address and port to listen on |
| `-base-dir` | `/srv/files` | Base directory for file storage (env: `UPLOAD_BASE_DIR`) |
| `-max-size` | `2147483648` (2GB) | Maximum upload size in bytes |
| `-upload-prefix` | `/upload` | URL prefix for upload endpoint |
| `-delete-prefix` | `/delete` | URL prefix for delete endpoint |
| `-mkdir-prefix` | `/mkdir` | URL prefix for mkdir endpoint |

## API

### Upload Files

```http
POST /upload/<path>/
Content-Type: multipart/form-data
```

- `<path>` maps to a subdirectory under the base files root
- Multiple files can be sent in a single request
- Form field name can be anything (commonly `file` or `files`)

#### Example Request

```bash
# Upload single file
curl -X POST -F "file=@photo.jpg" http://localhost:8080/upload/photos/2026/

# Upload multiple files
curl -X POST \
  -F "files=@doc1.pdf" \
  -F "files=@doc2.pdf" \
  -F "files=@doc3.pdf" \
  http://localhost:8080/upload/documents/

# Upload to root directory
curl -X POST -F "file=@readme.txt" http://localhost:8080/upload/
```

#### Response

**Success (201 Created):**
```json
{
  "uploaded": ["photo.jpg"],
  "skipped": []
}
```

**Partial Success (201 Created):**
```json
{
  "uploaded": ["new-file.txt"],
  "skipped": ["existing-file.txt"]
}
```

**All Skipped (409 Conflict):**
```json
{
  "uploaded": [],
  "skipped": ["existing-file.txt"]
}
```

**Error:**
```json
{
  "error": "description of the error"
}
```

#### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 201 | At least one file uploaded successfully |
| 400 | Malformed request (bad path, bad content type, invalid filename) |
| 404 | Invalid target path |
| 405 | Method not allowed (only POST is accepted) |
| 409 | All files skipped due to conflicts (files already exist) |
| 413 | Upload size exceeds limit |
| 500 | Internal server error |

### Delete File or Directory

```http
DELETE /delete/<path>
```

- `<path>` maps to a file or directory under the base files root
- Directories can only be deleted if empty
- Symlinks are rejected (cannot be deleted)

#### Example Requests

```bash
# Delete a file
curl -X DELETE http://localhost:8080/delete/photos/2026/image.jpg

# Delete an empty directory
curl -X DELETE http://localhost:8080/delete/photos/2026/

# Verbose output to see status
curl -v -X DELETE http://localhost:8080/delete/docs/old-file.pdf
```

#### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 204 | Successfully deleted |
| 400 | Invalid path (traversal attempt, symlink, etc.) |
| 403 | Forbidden (e.g., trying to delete base directory) |
| 404 | Path does not exist |
| 405 | Method not allowed (only DELETE is accepted) |
| 409 | Directory is not empty |
| 500 | Internal server error |
### Create Directory

```http
POST /mkdir/<path>/
```

- `<path>` maps to the new directory path under the base files root
- The final path component is the directory to be created
- Parent directories must already exist (no recursive creation)
- Symlinks in the path are rejected

#### Example Requests

```bash
# Create a directory in root
curl -X POST http://localhost:8080/mkdir/photos/

# Create a nested directory (parent must exist)
curl -X POST http://localhost:8080/mkdir/photos/2026/

# Create another level
curl -X POST http://localhost:8080/mkdir/photos/2026/vacation/
```

#### Response

**Success (201 Created):**
```json
{
  "created": "photos/2026/"
}
```

**Error:**
```json
{
  "error": "description of the error"
}
```

#### HTTP Status Codes

| Code | Meaning |
|------|---------||
| 201 | Directory created successfully |
| 400 | Invalid path or directory name |
| 403 | Forbidden (e.g., trying to create base directory, symlink escape) |
| 404 | Parent directory does not exist |
| 405 | Method not allowed (only POST is accepted) |
| 409 | Directory or file already exists at path |
| 500 | Internal server error |
### Health Check

```http
GET /health
```

Returns `200 OK` with body `OK`.

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

### Filename Handling

- Original filenames are preserved (base name only)
- Path components in filenames are stripped (`foo/bar/file.txt` → `file.txt`)
- Files starting with `.` are rejected (prevents `.htaccess`, `.env`, etc.)

## Testing

```bash
go test -v ./...
```

## Assumptions & Design Decisions

1. **Filename Preservation** - original filenames are kept; no sanitization beyond safety checks
2. **Directory Creation** - target directories are created with mode `0755`
3. **File Permissions** - uploaded files are created with mode `0644`
4. **No Timeout** - read/write timeouts are disabled to support large uploads over slow connections
5. **Memory Buffer** - 32MB memory buffer per request; larger files stream through temp files
6. **Atomic Writes** - uses `O_EXCL` flag to prevent race conditions on file creation
7. **Sync to Disk** - `fsync` is called before returning success to ensure durability
