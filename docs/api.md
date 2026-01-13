# API Documentation

This document describes the HTTP API endpoints provided by the files-svc service.

## Overview

The file service provides endpoints for:
- Uploading files
- Deleting files and directories
- Creating directories
- Renaming files and directories
- Health checks

All endpoints return JSON responses on errors. Content-Type is `application/json` for JSON responses.

---

## Endpoints

### Upload Files

```
POST /api/upload/<path>/
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
curl -X POST -F "file=@photo.jpg" http://localhost:8080/api/upload/

# Upload single file to subdirectory
curl -X POST -F "file=@photo.jpg" http://localhost:8080/api/upload/photos/2026/

# Upload multiple files
curl -X POST \
  -F "files=@doc1.pdf" \
  -F "files=@doc2.pdf" \
  -F "files=@doc3.pdf" \
  http://localhost:8080/api/upload/documents/
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
DELETE /api/delete/<path>
```

Delete a file or empty directory at the specified path.

#### Request

- **Method:** `DELETE`
- **URL Parameters:**
  - `<path>` - Path to the file or directory to delete (required)

#### Example Requests

```bash
# Delete a file
curl -X DELETE http://localhost:8080/api/delete/photos/2026/image.jpg

# Delete an empty directory
curl -X DELETE http://localhost:8080/api/delete/photos/2026/

# With verbose output
curl -v -X DELETE http://localhost:8080/api/delete/docs/old-file.pdf
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
POST /api/mkdir/<path>/
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
curl -X POST http://localhost:8080/api/mkdir/photos/

# Create a nested directory (parent must exist)
curl -X POST http://localhost:8080/api/mkdir/photos/2026/

# Create another level
curl -X POST http://localhost:8080/api/mkdir/photos/2026/vacation/
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

### Rename File or Directory

```
POST /api/rename/<oldPath>?newName=<newName>
```

Rename a file or directory within the same parent directory.

#### Request

- **Method:** `POST` or `PATCH`
- **URL Parameters:**
  - `<oldPath>` - Current path to the file or directory (required)
- **Query Parameters:**
  - `newName` - New name for the file or directory (required, must be a simple name without path separators)

#### Example Requests

```bash
# Rename a file
curl -X POST "http://localhost:8080/api/rename/photos/old-name.jpg?newName=new-name.jpg"

# Rename a directory
curl -X POST "http://localhost:8080/api/rename/documents/old-folder?newName=new-folder"

# Using PATCH method
curl -X PATCH "http://localhost:8080/api/rename/file.txt?newName=renamed.txt"
```

#### Response

**Success (200 OK):**
```json
{
  "old": "photos/old-name.jpg",
  "new": "photos/new-name.jpg",
  "success": true
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
| 200 | Successfully renamed |
| 400 | Invalid path or new name (traversal attempt, path separators in name, symlink, etc.) |
| 403 | Permission denied |
| 404 | Source path does not exist |
| 405 | Method not allowed (only POST and PATCH are accepted) |
| 409 | Destination already exists |
| 500 | Internal server error |

#### Notes

- Renaming only works within the same parent directory
- The `newName` must be a simple filename without path separators
- Existing files/directories cannot be overwritten
- Symlinks cannot be renamed (rejected for security)
- Directory contents are preserved when renaming directories

---

### Share File Publicly

```
POST /api/share-public/<path>
```

Create a public share symlink for a file. The symlink is created in the public base directory with the same relative path structure.

#### Prerequisites

The service must be started with the `-public-base-dir` flag or `FILES_SVC_PUBLIC_BASE_DIR` environment variable set. If not configured, this endpoint returns 501 Not Implemented.

#### Request

- **Method:** `POST`
- **URL Parameters:**
  - `<path>` - Path to the file to share publicly (required)
- **Body:** None required

#### Example Requests

```bash
# Share a file in a subdirectory
curl -X POST http://localhost:8080/api/share-public/photos/2026/pic.jpg

# Share a file in the root
curl -X POST http://localhost:8080/api/share-public/document.pdf
```

#### Example Result

Given:
- base-dir: `/srv/files`
- public-base-dir: `/srv/public`
- request: `POST /api/share-public/photos/2026/pic.jpg`

Results in:
- Real file: `/srv/files/photos/2026/pic.jpg`
- Symlink created: `/srv/public/photos/2026/pic.jpg` → `/srv/files/photos/2026/pic.jpg`

#### Response

**Success (201 Created):**
```json
{
  "shared": "photos/2026/pic.jpg"
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
| 201 | Share symlink created successfully |
| 400 | Invalid path (traversal attempt, symlink, directory, non-regular file) |
| 403 | Permission denied |
| 404 | File does not exist |
| 405 | Method not allowed (only POST is accepted) |
| 409 | Share already exists with different target, or path already exists in public directory |
| 501 | Public sharing not enabled (public-base-dir not configured) |
| 500 | Internal server error |

#### Notes

- Only regular files can be shared (directories and symlinks are rejected)
- The operation is idempotent: sharing the same file again returns success if the symlink already points to the same target
- Parent directories in the public base directory are created automatically
- The symlink points to the absolute path of the source file

---

### Health Check

```
GET /api/health
```

Returns the health status of the service.

#### Request

- **Method:** `GET`
- **Body:** None

#### Example Request

```bash
curl http://localhost:8080/api/health
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
