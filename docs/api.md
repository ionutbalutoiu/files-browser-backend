# API Specification

Base URL: `/api`

## Endpoints

### Health Check

```http
GET /healthz
```

**Response:** `200 OK` with empty body

---

### Upload Files

```http
PUT /api/files?path=<path>
```

Upload files to a directory. Creates parent directories automatically.

**Request:**
- Content-Type: `multipart/form-data`
- Query: `path` - target directory (optional, defaults to root)
- Body: multipart form with files (field name can be anything)

**Response:**
```typescript
// 201 Created (at least one file uploaded)
// 409 Conflict (all files already exist)
{
  uploaded: string[]  // successfully uploaded filenames
  skipped: string[]   // skipped due to existing files
  errors?: string[]   // error messages (if any)
}
```

**Status Codes:**

| Code | Condition |
| ---- | --------- |
| 201 | At least one file uploaded |
| 400 | Invalid path or content type |
| 409 | All files skipped (already exist) |
| 413 | Upload size exceeds limit |

**Notes:**
- Files starting with `.` are rejected
- Existing files are never overwritten

---

### Create Folder

```http
POST /api/folders
```

**Request:**
```typescript
{
  path: string  // e.g. "photos/2026/vacation"
}
```

**Response:**
```typescript
// 201 Created
{
  created: string  // the created path
}
```

**Status Codes:**

| Code | Condition |
| ---- | --------- |
| 201 | Directory created |
| 400 | Invalid path or missing path field |
| 409 | Directory already exists |

---

### Delete Item

```http
DELETE /api/files?path=<path>
```

Delete a file or empty directory.

**Request:**
- Query: `path` - path to delete (required)

**Response:** `204 No Content`

**Status Codes:**

| Code | Condition |
| ---- | --------- |
| 204 | Deleted successfully |
| 400 | Invalid path |
| 403 | Cannot delete base directory |
| 404 | Path does not exist |
| 409 | Directory is not empty |

---

### Move Item

```http
POST /api/files/move
```

Move a file/directory to a new location.

**Request:**
```typescript
{
  from: string  // source path, e.g. "docs/old.txt"
  to: string    // destination path, e.g. "archive/new.txt"
}
```

**Response:**
```typescript
// 200 OK
{
  from: string
  to: string
  success: boolean
}
```

**Status Codes:**

| Code | Condition |
| ---- | --------- |
| 200 | Moved successfully |
| 400 | Invalid paths or missing fields |
| 404 | Source does not exist |
| 409 | Destination already exists |

---

### Rename Item

```http
POST /api/files/rename
```

Rename a file/directory in place.

**Request:**
```typescript
{
  path: string  // current path, e.g. "docs/old.txt"
  name: string  // new filename, e.g. "new.txt"
}
```

**Response:**
```typescript
// 200 OK
{
  from: string  // original path
  to: string    // new path
  success: boolean
}
```

**Status Codes:**

| Code | Condition |
| ---- | --------- |
| 200 | Renamed successfully |
| 400 | Invalid path/name or name contains path separators |
| 404 | Source does not exist |
| 409 | Destination already exists |

---

### List Public Shares

```http
GET /api/public-shares
```

List all publicly shared files.

**Response:**
```typescript
// 200 OK
string[]  // array of relative paths to shared files, sorted alphabetically
```

**Status Codes:**

| Code | Condition |
| ---- | --------- |
| 200 | Success |
| 501 | Public sharing not enabled |

**Notes:**

- Returns empty array `[]` if no files are shared
- Only includes valid symlinks pointing to existing files
- Excludes directories and broken symlinks
- Results are sorted alphabetically

---

### Create Public Share

```http
POST /api/public-shares
```

Create a public share for a file.

**Request:**
```typescript
{
  path: string  // file path to share, e.g. "docs/report.pdf"
}
```

**Response:**
```typescript
// 201 Created
{
  shareId: string  // base64-encoded path, URL-safe
  path: string     // the shared file path
}
```

**Status Codes:**

| Code | Condition |
| ---- | --------- |
| 201 | Share created |
| 400 | Invalid path or not a regular file |
| 404 | File does not exist |
| 409 | Share already exists |
| 501 | Public sharing not enabled |

**Notes:**

- Only regular files can be shared (not directories)
- Share is a symlink in `PUBLIC_BASE_DIR`

---

### Delete Public Share

```http
DELETE /api/public-shares?path=<path>
```

Delete a public share by its path.

**Request:**

- Query: `path` - the path of the shared file (required)

**Response:** `204 No Content`

**Status Codes:**

| Code | Condition |
| ---- | --------- |
| 204 | Deleted successfully |
| 400 | Invalid or missing path |
| 404 | Share does not exist |
| 501 | Public sharing not enabled |

---

## Error Response Format

All error responses return:

```typescript
{
  error: string  // human-readable error message
}
```

## Path Conventions

- Paths are relative to the base directory
- No leading slash required
- Use `/` as separator
- `..`, absolute paths, and null bytes are rejected
- Hidden files (starting with `.`) are rejected
