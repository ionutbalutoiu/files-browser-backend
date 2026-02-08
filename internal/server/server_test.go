package server

import (
	"testing"

	"files-browser-backend/internal/config"
)

func TestNewSetsHardenedHTTPServerDefaults(t *testing.T) {
	cfg := config.Config{
		ListenAddr:    ":8080",
		BaseDir:       t.TempDir(),
		PublicBaseDir: "",
		MaxUploadSize: 1024,
	}

	srv := New(cfg)

	if srv.httpServer.ReadHeaderTimeout != readHeaderTimeout {
		t.Fatalf("expected ReadHeaderTimeout %v, got %v", readHeaderTimeout, srv.httpServer.ReadHeaderTimeout)
	}
	if srv.httpServer.MaxHeaderBytes != maxHeaderBytes {
		t.Fatalf("expected MaxHeaderBytes %d, got %d", maxHeaderBytes, srv.httpServer.MaxHeaderBytes)
	}
}
