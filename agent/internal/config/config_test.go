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

func TestLoadServicesTemplatesExample(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "..", "examples", "services.example.toml"))
	if err != nil {
		t.Fatalf("load examples/services.example.toml failed: %v", err)
	}
	if len(cfg.Services) != 6 {
		t.Fatalf("expected 6 template services, got %d", len(cfg.Services))
	}
	if cfg.Services[1].Type != "tcp" || cfg.Services[1].Addr != "127.0.0.1:5432" || cfg.Services[1].TimeoutMs != 500 {
		t.Fatalf("unexpected tcp service: %+v", cfg.Services[1])
	}
	if cfg.Services[3].Type != "http_json" || len(cfg.Services[3].Fields) != 3 ||
		cfg.Services[3].Fields[0].Path != "players.online" || cfg.Services[3].Fields[0].Label != "Players Online" {
		t.Fatalf("unexpected http_json service fields: %+v", cfg.Services[3])
	}
}

func TestTCPAndHTTPJSONValidationDefaults(t *testing.T) {
	dir := t.TempDir()

	validPath := filepath.Join(dir, "valid.toml")
	validToml := `
[[services]]
id = "redis"
type = "tcp"
addr = "127.0.0.1:6379"

[[services]]
id = "api"
type = "http_json"
url = "http://127.0.0.1:8080/health"
`
	if err := os.WriteFile(validPath, []byte(validToml), 0o644); err != nil {
		t.Fatalf("write valid toml: %v", err)
	}
	cfg, err := Load(validPath)
	if err != nil {
		t.Fatalf("load valid toml: %v", err)
	}
	if cfg.Services[0].TimeoutMs != 500 {
		t.Fatalf("expected tcp default timeout 500ms, got %d", cfg.Services[0].TimeoutMs)
	}
	if cfg.Services[1].TimeoutMs != 1500 {
		t.Fatalf("expected http_json default timeout 1500ms, got %d", cfg.Services[1].TimeoutMs)
	}

	missingAddr := filepath.Join(dir, "bad_tcp.toml")
	if err := os.WriteFile(missingAddr, []byte("[[services]]\nid = \"bad\"\ntype = \"tcp\"\n"), 0o644); err != nil {
		t.Fatalf("write bad tcp: %v", err)
	}
	if _, err := Load(missingAddr); err == nil {
		t.Fatalf("expected validation error for tcp service without addr")
	}

	missingURL := filepath.Join(dir, "bad_http.toml")
	if err := os.WriteFile(missingURL, []byte("[[services]]\nid = \"bad\"\ntype = \"http_json\"\n"), 0o644); err != nil {
		t.Fatalf("write bad http: %v", err)
	}
	if _, err := Load(missingURL); err == nil {
		t.Fatalf("expected validation error for http_json service without url")
	}
}
