# SPA Integration

This document shows how to integrate file uploads, deletions, and directory creation into your Svelte SPA.

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

## Basic Create Directory Function

```javascript
/**
 * Create a new directory
 * @param {string} parentPath - Parent directory path (e.g., "photos/2026")
 * @param {string} dirName - Name of the new directory
 * @returns {Promise<{created: string}>}
 */
async function createDirectory(parentPath, dirName) {
  // Validate directory name client-side
  if (!dirName || dirName.trim() === '') {
    throw new Error('Directory name cannot be empty');
  }
  
  if (dirName.includes('/') || dirName.includes('\\')) {
    throw new Error('Directory name cannot contain path separators');
  }
  
  if (dirName === '.' || dirName === '..') {
    throw new Error('Invalid directory name');
  }
  
  // Build the full path
  const normalizedParent = parentPath.replace(/^\/+|\/+$/g, '');
  const fullPath = normalizedParent 
    ? `${normalizedParent}/${dirName}` 
    : dirName;
  
  const response = await fetch(`/mkdir/${fullPath}/`, {
    method: 'POST',
  });
  
  const result = await response.json().catch(() => ({}));
  
  if (!response.ok) {
    const message = result.error || `Create directory failed: ${response.status}`;
    
    // Map status codes to user-friendly messages
    switch (response.status) {
      case 404:
        throw new Error('Parent directory does not exist');
      case 409:
        throw new Error('Directory already exists');
      case 403:
        throw new Error('Cannot create directory here');
      case 400:
        throw new Error('Invalid directory name');
      default:
        throw new Error(message);
    }
  }
  
  return result;
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

The SPA uses hash routing, and the API endpoints use path routing. The mapping is straightforward:

### Upload Mapping

| SPA URL | Upload Endpoint | Files Location |
| ------- | --------------- | -------------- |
| `/ui/#/` | `POST /upload/` | `/srv/files/` |
| `/ui/#/photos` | `POST /upload/photos/` | `/srv/files/photos/` |
| `/ui/#/photos/2026` | `POST /upload/photos/2026/` | `/srv/files/photos/2026/` |

### Mkdir Mapping

| SPA URL | Action | Mkdir Endpoint | Result |
| ------- | ------ | -------------- | ------ |
| `/ui/#/` | Create "photos" | `POST /mkdir/photos/` | `/srv/files/photos/` |
| `/ui/#/photos` | Create "2026" | `POST /mkdir/photos/2026/` | `/srv/files/photos/2026/` |
| `/ui/#/photos/2026` | Create "vacation" | `POST /mkdir/photos/2026/vacation/` | `/srv/files/photos/2026/vacation/` |

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

## Create Directory Integration Example

```svelte
<script>
  import { createEventDispatcher } from 'svelte';
  
  export let currentPath = '';
  
  const dispatch = createEventDispatcher();
  
  let showInput = false;
  let dirName = '';
  let creating = false;
  let error = null;
  
  function startCreate() {
    showInput = true;
    dirName = '';
    error = null;
  }
  
  function cancelCreate() {
    showInput = false;
    dirName = '';
    error = null;
  }
  
  async function handleCreate() {
    if (!dirName.trim()) {
      error = 'Please enter a directory name';
      return;
    }
    
    creating = true;
    error = null;
    
    try {
      const result = await createDirectory(currentPath, dirName.trim());
      showInput = false;
      dirName = '';
      // Notify parent to refresh file list
      dispatch('created', { path: result.created });
    } catch (err) {
      error = err.message;
    } finally {
      creating = false;
    }
  }
  
  function handleKeydown(event) {
    if (event.key === 'Enter') {
      handleCreate();
    } else if (event.key === 'Escape') {
      cancelCreate();
    }
  }
</script>

<div class="new-folder">
  {#if showInput}
    <div class="input-row">
      <input
        type="text"
        bind:value={dirName}
        placeholder="New folder name"
        disabled={creating}
        on:keydown={handleKeydown}
        autofocus
      />
      <button on:click={handleCreate} disabled={creating || !dirName.trim()}>
        {creating ? 'Creating...' : 'Create'}
      </button>
      <button on:click={cancelCreate} disabled={creating}>
        Cancel
      </button>
    </div>
    {#if error}
      <p class="error">{error}</p>
    {/if}
  {:else}
    <button on:click={startCreate}>
      📁 New Folder
    </button>
  {/if}
</div>

<style>
  .new-folder {
    margin: 10px 0;
  }
  .input-row {
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .input-row input {
    flex: 1;
    padding: 8px;
  }
  .error {
    color: red;
    font-size: 0.9em;
    margin-top: 4px;
  }
</style>
```

## Mkdir HTTP Status Codes

| Status | Meaning | User Message |
| ------ | ------- | ------------ |
| 201 | Created | (refresh UI, optionally show success) |
| 400 | Invalid name/path | "Invalid directory name" |
| 403 | Forbidden | "Cannot create directory here" |
| 404 | Parent missing | "Parent directory does not exist" |
| 405 | Wrong method | (shouldn't happen from SPA) |
| 409 | Already exists | "Directory already exists" |
| 500 | Server error | "Server error, try again" |

## Complete File Browser Toolbar Example

```svelte
<script>
  import { createEventDispatcher } from 'svelte';
  
  export let currentPath = '';
  
  const dispatch = createEventDispatcher();
  
  let files = [];
  let newFolderName = '';
  let showNewFolder = false;
  let uploading = false;
  let creating = false;
  
  async function handleUpload() {
    if (!files.length) return;
    uploading = true;
    try {
      await uploadFiles(files, currentPath);
      files = [];
      dispatch('refresh');
    } catch (err) {
      alert(err.message);
    } finally {
      uploading = false;
    }
  }
  
  async function handleCreateFolder() {
    if (!newFolderName.trim()) return;
    creating = true;
    try {
      await createDirectory(currentPath, newFolderName.trim());
      newFolderName = '';
      showNewFolder = false;
      dispatch('refresh');
    } catch (err) {
      alert(err.message);
    } finally {
      creating = false;
    }
  }
</script>

<div class="toolbar">
  <!-- Upload -->
  <label class="upload-btn">
    📤 Upload
    <input type="file" multiple bind:files on:change={handleUpload} hidden />
  </label>
  
  <!-- New Folder -->
  {#if showNewFolder}
    <input 
      type="text" 
      bind:value={newFolderName}
      placeholder="Folder name"
      on:keydown={(e) => e.key === 'Enter' && handleCreateFolder()}
    />
    <button on:click={handleCreateFolder} disabled={creating}>
      {creating ? '...' : '✓'}
    </button>
    <button on:click={() => showNewFolder = false}>✕</button>
  {:else}
    <button on:click={() => showNewFolder = true}>📁 New Folder</button>
  {/if}
</div>
```
