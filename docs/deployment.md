# Deployment

This document covers deployment options for the file service.

## systemd Service

Create `/etc/systemd/system/files-svc.service`:

```ini
[Unit]
Description=File Service
After=network.target

[Service]
Type=simple
User=files-svc
Group=www-data
WorkingDirectory=/opt/files-svc
ExecStart=/opt/files-svc/files-svc \
    -listen 127.0.0.1:8080 \
    -base-dir /srv/files \
    -max-size 2147483648
Restart=always
RestartSec=5

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/srv/files
PrivateTmp=true

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=files-svc

[Install]
WantedBy=multi-user.target
```

### Setup Commands

```bash
# Create service user
sudo useradd -r -s /bin/false files-svc

# Create directories
sudo mkdir -p /opt/files-svc /srv/files

# Copy binary
sudo cp files-svc /opt/files-svc/
sudo chown -R root:root /opt/files-svc
sudo chmod 755 /opt/files-svc/files-svc

# Set up files directory
sudo chown files-svc:www-data /srv/files
sudo chmod 775 /srv/files

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable files-svc
sudo systemctl start files-svc

# Check status
sudo systemctl status files-svc
sudo journalctl -u files-svc -f
```

## Docker

### Dockerfile

```dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o files-svc ./cmd/files-svc

FROM alpine:3.23

RUN adduser -D -u 1000 uploader
WORKDIR /app
COPY --from=builder /build/files-svc .

USER uploader
EXPOSE 8080

ENTRYPOINT ["/app/files-svc"]
CMD ["-listen", ":8080", "-base-dir", "/data", "-max-size", "2147483648"]
```

### Build and Run

```bash
# Build image
docker build -t files-svc .

# Run container
docker run -d \
    --name files-svc \
    -p 8080:8080 \
    -v /srv/files:/data \
    files-svc

# With custom settings
docker run -d \
    --name files-svc \
    -p 9000:9000 \
    -v /srv/files:/data \
    files-svc \
    -listen :9000 \
    -base-dir /data \
    -max-size 1073741824
```

### Docker Compose

```yaml
version: '3.8'

services:
  files-svc:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - files-data:/data
    command:
      - "-listen"
      - ":8080"
      - "-base-dir"
      - "/data"
      - "-max-size"
      - "2147483648"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - files-data:/srv/files:ro
      - ./nginx.conf:/etc/nginx/conf.d/default.conf:ro
      - ./spa:/var/www/spa:ro
    depends_on:
      - files-svc

volumes:
  files-data:
```

## Environment Variables (Alternative)

If you prefer environment variables over flags, wrap the binary:

```bash
#!/bin/bash
# /opt/files-svc/run.sh
exec /opt/files-svc/files-svc \
    -listen ":8080" \
    -base-dir "${FILES_SVC_UPLOAD_BASE_DIR:-/srv/files}" \
    -max-size "${FILES_SVC_UPLOAD_MAX_SIZE:-2147483648}"
```

## Health Checks

The service exposes `/api/health` for monitoring:

```bash
# Simple check
curl -f http://localhost:8080/api/health

# For Kubernetes liveness probe
livenessProbe:
  httpGet:
    path: /api/health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

# For Kubernetes readiness probe
readinessProbe:
  httpGet:
    path: /api/health
    port: 8080
  initialDelaySeconds: 2
  periodSeconds: 5
```

## Logging

Logs go to stdout/stderr. In production:

```bash
# View systemd logs
journalctl -u files-svc -f

# View Docker logs
docker logs -f files-svc

# Log format
# 2026/01/12 10:30:45 OK: uploaded photo.jpg to /srv/files/photos/2026 (field: file)
# 2026/01/12 10:30:46 SKIP: file document.pdf already exists (field: file)
# 2026/01/12 10:30:47 ERROR: file bad.txt (field: file): hidden files not allowed
```
