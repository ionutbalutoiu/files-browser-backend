# SPA Integration

This document shows how to integrate file uploads and deletions into your Svelte SPA.

## Basic Upload Function

```javascript
/**
 * Upload files to the server
 * @param {FileList|File[]} files - Files to upload
 * @param {string} targetPath - Target directory path (e.g., "photos/2026")
 * @returns {Promise<{uploaded: string[], skipped: string[], errors?: string[]}>}
 */
async function uploadFiles(files, targetPath) {
  const formData = new FormData();
  
  // Add all files to form data
  for (const file of files) {
    formData.append('files', file);
  }
  
  // Ensure path has trailing slash
  const path = targetPath.replace(/\/$/, '') + '/';
  
  const response = await fetch(`/upload/${path}`, {
    method: 'POST',
    body: formData,
    // Don't set Content-Type header - browser sets it with boundary
  });
  
  const result = await response.json();
  
  if (!response.ok && !result.uploaded?.length) {
    throw new Error(result.error || `Upload failed: ${response.status}`);
  }
  
  return result;
}
```

## Basic Delete Function

```javascript
/**
 * Delete a file or empty directory
 * @param {string} path - Path to file or directory (e.g., "photos/2026/image.jpg")
 * @returns {Promise<void>}
 */
async function deleteFile(path) {
  // Normalize path (remove leading/trailing slashes for files)
  const normalizedPath = path.replace(/^\/+|\/+$/g, '');
  
  const response = await fetch(`/delete/${normalizedPath}`, {
    method: 'DELETE',
  });
  
  if (!response.ok) {
    const result = await response.json().catch(() => ({}));
    const message = result.error || `Delete failed: ${response.status}`;
    
    // Map status codes to user-friendly messages
    switch (response.status) {
      case 404:
        throw new Error('File not found');
      case 409:
        throw new Error('Directory is not empty');
      case 403:
        throw new Error('Cannot delete this path');
      default:
        throw new Error(message);
    }
  }
}
```

## Svelte Component Example

```svelte
<script>
  let files = [];
  let uploading = false;
  let result = null;
  let error = null;
  
  // Current directory from URL hash (e.g., #/photos/2026 -> photos/2026)
  $: currentPath = window.location.hash.replace(/^#\/?/, '');
  
  async function handleUpload() {
    if (!files.length) return;
    
    uploading = true;
    error = null;
    result = null;
    
    try {
      result = await uploadFiles(files, currentPath);
      files = []; // Clear selection
      
      // Optionally refresh file list
      // dispatch('refresh');
    } catch (err) {
      error = err.message;
    } finally {
      uploading = false;
    }
  }
  
  function handleFileSelect(event) {
    files = Array.from(event.target.files);
  }
  
  function handleDrop(event) {
    event.preventDefault();
    files = Array.from(event.dataTransfer.files);
  }
</script>

<div 
  class="upload-zone"
  on:drop={handleDrop}
  on:dragover|preventDefault
>
  <input 
    type="file" 
    multiple 
    on:change={handleFileSelect}
    disabled={uploading}
  />
  
  <button 
    on:click={handleUpload} 
    disabled={!files.length || uploading}
  >
    {uploading ? 'Uploading...' : `Upload ${files.length} file(s)`}
  </button>
  
  {#if error}
    <p class="error">{error}</p>
  {/if}
  
  {#if result}
    <div class="result">
      {#if result.uploaded.length}
        <p class="success">Uploaded: {result.uploaded.join(', ')}</p>
      {/if}
      {#if result.skipped.length}
        <p class="warning">Skipped (already exist): {result.skipped.join(', ')}</p>
      {/if}
      {#if result.errors?.length}
        <p class="error">Errors: {result.errors.join(', ')}</p>
      {/if}
    </div>
  {/if}
</div>

<style>
  .upload-zone {
    border: 2px dashed #ccc;
    padding: 20px;
    text-align: center;
  }
  .error { color: red; }
  .warning { color: orange; }
  .success { color: green; }
</style>
```

## Upload with Progress

For large files, you may want to show upload progress:

```javascript
/**
 * Upload files with progress tracking
 * @param {FileList|File[]} files - Files to upload
 * @param {string} targetPath - Target directory path
 * @param {function} onProgress - Progress callback (0-100)
 */
async function uploadFilesWithProgress(files, targetPath, onProgress) {
  return new Promise((resolve, reject) => {
    const formData = new FormData();
    for (const file of files) {
      formData.append('files', file);
    }
    
    const path = targetPath.replace(/\/$/, '') + '/';
    
    const xhr = new XMLHttpRequest();
    
    xhr.upload.addEventListener('progress', (e) => {
      if (e.lengthComputable) {
        const percent = Math.round((e.loaded / e.total) * 100);
        onProgress(percent);
      }
    });
    
    xhr.addEventListener('load', () => {
      try {
        const result = JSON.parse(xhr.responseText);
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve(result);
        } else if (result.uploaded?.length) {
          // Partial success
          resolve(result);
        } else {
          reject(new Error(result.error || `Upload failed: ${xhr.status}`));
        }
      } catch {
        reject(new Error('Invalid server response'));
      }
    });
    
    xhr.addEventListener('error', () => {
      reject(new Error('Network error'));
    });
    
    xhr.open('POST', `/upload/${path}`);
    xhr.send(formData);
  });
}
```

## Handling Responses

### Success Response

```json
{
  "uploaded": ["file1.txt", "file2.txt"],
  "skipped": []
}
```

### Partial Success

```json
{
  "uploaded": ["new-file.txt"],
  "skipped": ["existing-file.txt"],
  "errors": ["bad-file.txt: hidden files not allowed"]
}
```

### Error Response

```json
{
  "error": "invalid path: contains parent directory reference"
}
```

## URL Mapping

The SPA uses hash routing, and the upload endpoint uses path routing. The mapping is straightforward:

| SPA URL | Upload Endpoint | Files Location |
| ------- | --------------- | -------------- |
| `/ui/#/` | `POST /upload/` | `/srv/files/` |
| `/ui/#/photos` | `POST /upload/photos/` | `/srv/files/photos/` |
| `/ui/#/photos/2026` | `POST /upload/photos/2026/` | `/srv/files/photos/2026/` |

## Error Handling Best Practices

```javascript
async function safeUpload(files, path) {
  try {
    const result = await uploadFiles(files, path);
    
    // Handle different outcomes
    if (result.uploaded.length === 0) {
      if (result.skipped.length > 0) {
        return { 
          status: 'skipped', 
          message: `Files already exist: ${result.skipped.join(', ')}` 
        };
      }
      if (result.errors?.length > 0) {
        return { 
          status: 'error', 
          message: result.errors.join('; ') 
        };
      }
    }
    
    return { 
      status: 'success', 
      message: `Uploaded: ${result.uploaded.join(', ')}`,
      result 
    };
    
  } catch (err) {
    // Network errors, 4xx/5xx responses without uploaded files
    return { 
      status: 'error', 
      message: err.message 
    };
  }
}
```

## Security Notes

1. **Don't trust client filenames** - The server validates and sanitizes them
2. **Path is from URL hash** - Ensure your routing doesn't allow `..` in paths
3. **Size limits** - Nginx and the Go service both enforce limits; show user-friendly messages
4. **File types** - The server accepts any file type; add client-side validation if needed
5. **Delete confirmation** - Always prompt users before deleting; there's no undo

```javascript
// Optional client-side validation
function validateFiles(files) {
  const maxSize = 2 * 1024 * 1024 * 1024; // 2GB
  const errors = [];
  
  for (const file of files) {
    if (file.size > maxSize) {
      errors.push(`${file.name}: exceeds 2GB limit`);
    }
    if (file.name.startsWith('.')) {
      errors.push(`${file.name}: hidden files not allowed`);
    }
  }
  
  return errors;
}
```

## Delete Integration Example

```svelte
<script>
  export let fileName;
  export let filePath;
  
  let deleting = false;
  let error = null;
  
  async function handleDelete() {
    // Always confirm with user
    if (!confirm(`Delete "${fileName}"? This cannot be undone.`)) {
      return;
    }
    
    deleting = true;
    error = null;
    
    try {
      await deleteFile(filePath);
      // Refresh file list or remove from UI
      dispatch('deleted', { path: filePath });
    } catch (err) {
      error = err.message;
    } finally {
      deleting = false;
    }
  }
</script>

<button on:click={handleDelete} disabled={deleting}>
  {deleting ? 'Deleting...' : 'Delete'}
</button>

{#if error}
  <span class="error">{error}</span>
{/if}
```

## Delete HTTP Status Codes

| Status | Meaning | User Message |
| ------ | ------- | ------------ |
| 204 | Success | (no message, refresh UI) |
| 400 | Invalid path | "Invalid file path" |
| 403 | Forbidden | "Cannot delete this path" |
| 404 | Not found | "File not found" |
| 405 | Wrong method | (shouldn't happen from SPA) |
| 409 | Not empty | "Directory is not empty" |
| 500 | Server error | "Server error, try again" |
