# Nginx Integration

This document explains how to integrate the file service (upload & delete) with Nginx.

## Prerequisites

- Nginx installed and configured
- File service running (default: `localhost:8080`)
- Files directory accessible by both Nginx and the file service

## Configuration

Add this to your Nginx server block:

```nginx
# =============================================================================
# FILE SERVICE - Nginx Configuration (Upload & Delete)
# =============================================================================

# Maximum upload size (must match or exceed Go service's -max-size)
client_max_body_size 2G;

# Increase timeouts for large uploads
proxy_connect_timeout 300;
proxy_send_timeout 300;
proxy_read_timeout 300;
send_timeout 300;

# Serve static files directly
location /files/ {
    alias /srv/files/;
    autoindex on;
    autoindex_format json;
    autoindex_exact_size on;
    autoindex_localtime off;
    
    # Optional: enable download headers for all files
    add_header Content-Disposition 'attachment';
}

# SPA at /ui/
location /ui/ {
    alias /var/www/spa/;
    try_files $uri $uri/ /ui/index.html;
}

# Upload endpoint - proxy to Go service
location /upload/ {
    # Proxy to file service
    proxy_pass http://127.0.0.1:8080;
    
    # Required headers
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    
    # Disable buffering for streaming uploads
    proxy_request_buffering off;
    
    # Timeouts for large uploads
    proxy_connect_timeout 300;
    proxy_send_timeout 600;
    proxy_read_timeout 600;
    
    # Don't limit body size here (use client_max_body_size above)
    # The Go service enforces its own limit
}

# Delete endpoint - proxy to Go service
location /delete/ {
    # Proxy to file service
    proxy_pass http://127.0.0.1:8080;
    
    # Required headers
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

# Health check endpoint (optional, useful for load balancers)
location = /health {
    proxy_pass http://127.0.0.1:8080/health;
    proxy_set_header Host $host;
}
```

## Complete Example Server Block

```nginx
server {
    listen 80;
    server_name files.example.com;
    
    # Global settings
    client_max_body_size 2G;
    
    # Files - static serving with JSON autoindex
    location /files/ {
        alias /srv/files/;
        autoindex on;
        autoindex_format json;
    }
    
    # SPA
    location /ui/ {
        alias /var/www/spa/;
        try_files $uri $uri/ /ui/index.html;
    }
    
    # Upload API
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
    
    # Delete API
    location /delete/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
    
    # Root redirect
    location = / {
        return 302 /ui/;
    }
}
```

## Important Notes

### 1. Directory Paths Must Match

The Go service's `-base-dir` must point to the same directory that Nginx serves at `/files/`:

| Setting | Value |
| ------- | ----- |
| Nginx `alias` | `/srv/files/` |
| Go `-base-dir` | `/srv/files` |

### 2. File Permissions

The user running the Go service must have write permissions to the files directory:

```bash
# If nginx runs as www-data and upload service runs as upload-svc:
sudo chown -R upload-svc:www-data /srv/files
sudo chmod -R 775 /srv/files
sudo chmod g+s /srv/files  # New files inherit group
```

### 3. Size Limits

Configure `client_max_body_size` in Nginx to match or exceed the Go service's `-max-size`:

| Nginx | Go Service |
| ----- | ---------- |
| `client_max_body_size 2G` | `-max-size 2147483648` |

If Nginx's limit is lower, users will get a `413` error from Nginx before reaching the Go service.

### 4. Request Buffering

`proxy_request_buffering off` is critical for large uploads. Without it:
- Nginx buffers the entire upload to disk first
- Doubles disk I/O
- Delays the start of streaming to the Go service

### 5. Trailing Slashes

The upload endpoint expects a trailing slash. Configure your SPA to always include it:
- ✅ `POST /upload/photos/2026/`
- ❌ `POST /upload/photos/2026`

## Testing the Integration

```bash
# Test Nginx is proxying correctly
curl -X POST -F "file=@test.txt" http://localhost/upload/test/

# Check the file was created
curl http://localhost/files/test/test.txt

# Test health endpoint
curl http://localhost/upload/health
```

## Troubleshooting

### 413 Request Entity Too Large

Increase `client_max_body_size` in Nginx configuration.

### 502 Bad Gateway

The Go upload service is not running. Start it:

```bash
systemctl start upload-server
# or
./upload-server -base-dir /srv/files
```

### 504 Gateway Timeout

Increase proxy timeouts for very large uploads:

```nginx
proxy_send_timeout 3600;
proxy_read_timeout 3600;
```

### Permission Denied Errors

Check that the upload service user can write to the files directory:

```bash
sudo -u upload-svc touch /srv/files/test-write
```
