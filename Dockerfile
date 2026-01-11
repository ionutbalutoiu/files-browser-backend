FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o upload-server .

FROM alpine:3.23

# Create non-root user
RUN adduser -D -u 101 uploader

WORKDIR /app
COPY --from=builder /build/upload-server .

# Create data directory
RUN mkdir -p /data && chown uploader:uploader /data

# Environment variable for base directory
ENV UPLOAD_BASE_DIR=/data

USER uploader
EXPOSE 8080

ENTRYPOINT ["/app/upload-server"]
CMD ["-listen", ":8080", "-base-dir", "${UPLOAD_BASE_DIR}", "-max-size", "2147483648"]
