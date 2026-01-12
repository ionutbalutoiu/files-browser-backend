# API Documentation

This document describes the HTTP API endpoints provided by the files-svc service.

## Overview

The file service provides endpoints for:
- Uploading files
- Deleting files and directories
- Creating directories
- Health checks

All endpoints return JSON responses on errors. Content-Type is `application/json` for JSON responses.

---

## Endpoints

### Upload Files

```
POST /upload/<path>/
```

Upload one or more files to the specified directory path.

#### Request

- **Method:** `POST`
- **Content-Type:** `multipart/form-data`
- **URL Parameters:**
  - `<path>` - Target directory path relative to the base directory (optional, defaults to root)

#### Example Requests

```bash
# Upload single file to root
curl -X POST -F "file=@photo.jpg" http://localhost:8080/upload/

# Upload single file to subdirectory
curl -X POST -F "file=@photo.jpg" http://localhost:8080/upload/photos/2026/

# Upload multiple files
curl -X POST \
  -F "files=@doc1.pdf" \
  -F "files=@doc2.pdf" \
  -F "files=@doc3.pdf" \
  http://localhost:8080/upload/documents/
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

**All Files Skipped (409 Conflict):**
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

#### Notes

- Target directories are created automatically if they don't exist
- Files starting with `.` (hidden files) are rejected
- Existing files are never overwritten (returned in `skipped` array)
- The form field name can be anything (commonly `file` or `files`)

---

### Delete File or Directory

```
DELETE /delete/<path>
```

Delete a file or empty directory at the specified path.

#### Request

- **Method:** `DELETE`
- **URL Parameters:**
  - `<path>` - Path to the file or directory to delete (required)

#### Example Requests

```bash
# Delete a file
curl -X DELETE http://localhost:8080/delete/photos/2026/image.jpg

# Delete an empty directory
curl -X DELETE http://localhost:8080/delete/photos/2026/

# With verbose output
curl -v -X DELETE http://localhost:8080/delete/docs/old-file.pdf
```

#### Response

**Success:** Returns `204 No Content` with empty body.

**Error:**
```json
{
  "error": "description of the error"
}
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

#### Notes

- Directories can only be deleted if they are empty
- Symlinks cannot be deleted (rejected for security)
- The base directory itself cannot be deleted

---

### Create Directory

```
POST /mkdir/<path>/
```

Create a new directory at the specified path.

#### Request

- **Method:** `POST`
- **URL Parameters:**
  - `<path>` - Path for the new directory (required)
- **Body:** None required

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
|------|---------|
| 201 | Directory created successfully |
| 400 | Invalid path or directory name |
| 403 | Forbidden (e.g., trying to create base directory, symlink escape) |
| 404 | Parent directory does not exist |
| 405 | Method not allowed (only POST is accepted) |
| 409 | Directory or file already exists at path |
| 500 | Internal server error |

#### Notes

- Parent directories must already exist (no recursive creation)
- Directory names cannot contain path separators or null bytes
- Symlinks in the parent path are rejected for security

---

### Health Check

```
GET /health
```

Returns the health status of the service.

#### Request

- **Method:** `GET`
- **Body:** None

#### Example Request

```bash
curl http://localhost:8080/health
```

#### Response

**Success (200 OK):**
```
OK
```

#### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Service is healthy |

---

## Security Considerations

### Path Safety

All endpoints implement the following security measures:

1. **Path Traversal Prevention** - `../` sequences and absolute paths are rejected
2. **Symlink Escape Prevention** - Symlinks that escape the base directory are rejected
3. **Hidden File Protection** - Files starting with `.` are rejected on upload
4. **Overwrite Prevention** - Existing files cannot be overwritten

### What's NOT Protected (Out of Scope)

- **Authentication** - No auth mechanism; use Nginx or a reverse proxy
- **Authorization** - No per-path permissions
- **Virus Scanning** - Files are stored as-is
- **Rate Limiting** - Implement at the reverse proxy layer
- **File Type Validation** - Any file type is accepted

---

## Error Response Format

All error responses follow this format:

```json
{
  "error": "human-readable error message"
}
```

Common error messages include:
- `"only POST method is allowed"` / `"only DELETE method is allowed"`
- `"Content-Type must be multipart/form-data"`
- `"invalid path: contains parent directory reference"`
- `"invalid path: absolute paths not allowed"`
- `"invalid path: escapes base directory"`
- `"path does not exist"`
- `"directory is not empty"`
- `"cannot delete symlinks"`
- `"file already exists"`
- `"hidden files not allowed"`
- `"parent directory does not exist"`
- `"directory already exists"`
