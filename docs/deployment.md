# Deployment

This document covers deployment options for the upload service.

## systemd Service

Create `/etc/systemd/system/upload-server.service`:

```ini
[Unit]
Description=File Upload Service
After=network.target

[Service]
Type=simple
User=upload-svc
Group=www-data
WorkingDirectory=/opt/upload-server
ExecStart=/opt/upload-server/upload-server \
    -listen 127.0.0.1:8080 \
    -base-dir /srv/files \
    -max-size 2147483648 \
    -prefix /upload
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
SyslogIdentifier=upload-server

[Install]
WantedBy=multi-user.target
```

### Setup Commands

```bash
# Create service user
sudo useradd -r -s /bin/false upload-svc

# Create directories
sudo mkdir -p /opt/upload-server /srv/files

# Copy binary
sudo cp upload-server /opt/upload-server/
sudo chown -R root:root /opt/upload-server
sudo chmod 755 /opt/upload-server/upload-server

# Set up files directory
sudo chown upload-svc:www-data /srv/files
sudo chmod 775 /srv/files

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable upload-server
sudo systemctl start upload-server

# Check status
sudo systemctl status upload-server
sudo journalctl -u upload-server -f
```

## Docker

### Dockerfile

```dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o upload-server .

FROM alpine:3.19

RUN adduser -D -u 1000 uploader
WORKDIR /app
COPY --from=builder /build/upload-server .

USER uploader
EXPOSE 8080

ENTRYPOINT ["/app/upload-server"]
CMD ["-listen", ":8080", "-base-dir", "/data", "-max-size", "2147483648"]
```

### Build and Run

```bash
# Build image
docker build -t upload-server .

# Run container
docker run -d \
    --name upload-server \
    -p 8080:8080 \
    -v /srv/files:/data \
    upload-server

# With custom settings
docker run -d \
    --name upload-server \
    -p 9000:9000 \
    -v /srv/files:/data \
    upload-server \
    -listen :9000 \
    -base-dir /data \
    -max-size 1073741824
```

### Docker Compose

```yaml
version: '3.8'

services:
  upload-server:
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
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
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
      - upload-server

volumes:
  files-data:
```

## Environment Variables (Alternative)

If you prefer environment variables over flags, wrap the binary:

```bash
#!/bin/bash
# /opt/upload-server/run.sh
exec /opt/upload-server/upload-server \
    -listen "${UPLOAD_LISTEN:-:8080}" \
    -base-dir "${UPLOAD_BASE_DIR:-/srv/files}" \
    -max-size "${UPLOAD_MAX_SIZE:-2147483648}" \
    -prefix "${UPLOAD_PREFIX:-/upload}"
```

## Health Checks

The service exposes `/health` for monitoring:

```bash
# Simple check
curl -f http://localhost:8080/health

# For Kubernetes liveness probe
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

# For Kubernetes readiness probe
readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 2
  periodSeconds: 5
```

## Logging

Logs go to stdout/stderr. In production:

```bash
# View systemd logs
journalctl -u upload-server -f

# View Docker logs
docker logs -f upload-server

# Log format
# 2026/01/12 10:30:45 OK: uploaded photo.jpg to /srv/files/photos/2026 (field: file)
# 2026/01/12 10:30:46 SKIP: file document.pdf already exists (field: file)
# 2026/01/12 10:30:47 ERROR: file bad.txt (field: file): hidden files not allowed
```
