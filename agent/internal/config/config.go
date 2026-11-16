package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config defines the full runtime configuration for nullwake-agent.
type Config struct {
	Tailscale TailscaleConfig `toml:"tailscale"`
	Socket    SocketConfig    `toml:"socket"`
	Services  []ServiceConfig `toml:"services"`
}

// TailscaleConfig configures the embedded tsnet server.
type TailscaleConfig struct {
	Hostname   string `toml:"hostname"`
	StateDir   string `toml:"state_dir"`
	AuthKey    string `toml:"auth_key"`
	ListenPort int    `toml:"listen_port"`
}

// SocketConfig configures the push event Unix Domain Socket and ring buffer.
type SocketConfig struct {
	Path           string   `toml:"path"`
	FileMode       FileMode `toml:"file_mode"`
	RingBufferSize int      `toml:"ring_buffer_size"`
}

// ServiceConfig defines a monitored service plugin or push-only service metadata.
type ServiceConfig struct {
	ID        string   `toml:"id"`
	Name      string   `toml:"name"`
	Type      string   `toml:"type"` // "systemd" | "script" | "push_only"
	Unit      string   `toml:"unit"`
	Command   []string `toml:"command"`
	TimeoutMs int      `toml:"timeout_ms"`
}

// FileMode wraps os.FileMode to support both octal strings ("0666") and TOML integers (0o666).
type FileMode os.FileMode

// UnmarshalTOML parses a file mode from a string (e.g. "0666") or integer (e.g. 0o666).
func (m *FileMode) UnmarshalTOML(v any) error {
	switch val := v.(type) {
	case int64:
		if val < 0 || val > 0o7777 {
			return fmt.Errorf("file_mode out of range: %d", val)
		}
		*m = FileMode(val)
		return nil
	case string:
		parsed, err := strconv.ParseUint(strings.TrimSpace(val), 8, 32)
		if err != nil {
			return fmt.Errorf("invalid octal file_mode %q: %w", val, err)
		}
		*m = FileMode(parsed)
		return nil
	default:
		return fmt.Errorf("unexpected type %T for file_mode", v)
	}
}

// Default returns a Config populated with safe defaults.
func Default() Config {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "nullwake-agent"
	}
	return Config{
		Tailscale: TailscaleConfig{
			Hostname:   hostname,
			StateDir:   "/var/lib/nullwake/tsnet-state",
			ListenPort: 9100,
		},
		Socket: SocketConfig{
			Path:           "/run/nullwake.sock",
			FileMode:       FileMode(0o666),
			RingBufferSize: 200,
		},
	}
}

// Load reads and validates a TOML config file. If path is empty, Default() is returned.
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			return Config{}, fmt.Errorf("decode config %q: %w", path, err)
		}
	}

	if cfg.Tailscale.AuthKey == "" {
		cfg.Tailscale.AuthKey = os.Getenv("TS_AUTHKEY")
	}
	if cfg.Tailscale.Hostname == "" {
		cfg.Tailscale.Hostname = "nullwake-agent"
	}
	if cfg.Tailscale.StateDir == "" {
		cfg.Tailscale.StateDir = "/var/lib/nullwake/tsnet-state"
	}
	if cfg.Tailscale.ListenPort <= 0 || cfg.Tailscale.ListenPort > 65535 {
		cfg.Tailscale.ListenPort = 9100
	}
	if cfg.Socket.Path == "" {
		cfg.Socket.Path = "/run/nullwake.sock"
	}
	if cfg.Socket.FileMode == 0 {
		cfg.Socket.FileMode = FileMode(0o666)
	}
	if cfg.Socket.RingBufferSize <= 0 {
		cfg.Socket.RingBufferSize = 200
	}

	for i := range cfg.Services {
		if cfg.Services[i].ID == "" {
			return Config{}, fmt.Errorf("services[%d]: id is required", i)
		}
		if cfg.Services[i].Name == "" {
			cfg.Services[i].Name = cfg.Services[i].ID
		}
		if cfg.Services[i].TimeoutMs <= 0 {
			cfg.Services[i].TimeoutMs = 1500
		}
		switch cfg.Services[i].Type {
		case "systemd":
			if cfg.Services[i].Unit == "" {
				return Config{}, fmt.Errorf("services[%d] (%s): unit is required for systemd type", i, cfg.Services[i].ID)
			}
		case "script":
			if len(cfg.Services[i].Command) == 0 {
				return Config{}, fmt.Errorf("services[%d] (%s): command is required for script type", i, cfg.Services[i].ID)
			}
		case "push_only", "":
			cfg.Services[i].Type = "push_only"
		default:
			return Config{}, fmt.Errorf("services[%d] (%s): unsupported type %q", i, cfg.Services[i].ID, cfg.Services[i].Type)
		}
	}

	return cfg, nil
}
