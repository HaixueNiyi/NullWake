package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExampleConfig(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "config.example.toml"))
	if err != nil {
		t.Fatalf("load config.example.toml failed: %v", err)
	}
	if cfg.Tailscale.Hostname != "homelab-node" {
		t.Fatalf("unexpected hostname: %s", cfg.Tailscale.Hostname)
	}
	if cfg.Tailscale.ListenPort != 9100 {
		t.Fatalf("unexpected listen_port: %d", cfg.Tailscale.ListenPort)
	}
	if os.FileMode(cfg.Socket.FileMode) != 0o666 {
		t.Fatalf("expected 0666 mode, got %o", cfg.Socket.FileMode)
	}
	if cfg.Socket.RingBufferSize != 200 {
		t.Fatalf("expected ring_buffer_size 200, got %d", cfg.Socket.RingBufferSize)
	}
	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}
}
