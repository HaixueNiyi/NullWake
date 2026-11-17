package discover

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nullwake/nullwake/agent/internal/config"
)

func TestIsNoisySystemdUnit(t *testing.T) {
	if !IsNoisySystemdUnit("systemd-journald.service") {
		t.Fatalf("expected systemd-journald.service to be filtered")
	}
	if !IsNoisySystemdUnit("getty@tty1.service") {
		t.Fatalf("expected getty@tty1.service to be filtered")
	}
	if !IsNoisySystemdUnit("dbus-broker.service") {
		t.Fatalf("expected dbus-broker.service to be filtered")
	}
	if IsNoisySystemdUnit("docker.service") {
		t.Fatalf("expected docker.service to be kept")
	}
	if IsNoisySystemdUnit("postgresql.service") {
		t.Fatalf("expected postgresql.service to be kept")
	}
}

func TestParseProcNetTCPAndTCP6(t *testing.T) {
	inodes := map[string]string{
		"12345": "postgres",
		"99999": "ollama",
	}

	v4Sample := strings.Join([]string{
		"  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode",
		"   0: 0100007F:1538 00000000:0000 0A 00000000:00000000 00:00000000 00000000   100        0 12345 1 00000000 100 0 0 10 0",
		"   1: 0100007F:9999 00000000:0000 01 00000000:00000000 00:00000000 00000000   100        0 55555 1 00000000 100 0 0 10 0",
	}, "\n")

	v4 := ParseProcNetTCP(v4Sample, false, inodes)
	if len(v4) != 1 {
		t.Fatalf("expected 1 TCP_LISTEN IPv4 entry, got %d (%+v)", len(v4), v4)
	}
	if v4[0].Addr != "127.0.0.1:5432" || v4[0].Port != 5432 || v4[0].ProcessName != "postgres" {
		t.Fatalf("unexpected IPv4 candidate: %+v", v4[0])
	}

	v6Sample := strings.Join([]string{
		"  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode",
		"   0: 00000000000000000000000000000000:2CCC 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 99999 1 00000000 100 0 0 10 0",
		"   1: 00000000000000000000000001000000:1F90 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 88888 1 00000000 100 0 0 10 0",
	}, "\n")

	v6 := ParseProcNetTCP(v6Sample, true, inodes)
	if len(v6) != 2 {
		t.Fatalf("expected 2 TCP_LISTEN IPv6 entries, got %d", len(v6))
	}
	if v6[0].Addr != "127.0.0.1:11468" || v6[0].ProcessName != "ollama" {
		t.Fatalf("unexpected wildcard IPv6 candidate: %+v", v6[0])
	}
	if v6[1].Addr != "[::1]:8080" || v6[1].ID != "tcp-8080" {
		t.Fatalf("unexpected ::1 IPv6 candidate: %+v", v6[1])
	}
}

func TestRunDiscoverInteractiveAppendAndLoad(t *testing.T) {
	tmp := t.TempDir()
	procRoot := filepath.Join(tmp, "proc")
	if err := os.MkdirAll(filepath.Join(procRoot, "net"), 0o755); err != nil {
		t.Fatalf("mkdir proc/net: %v", err)
	}
	tcpBody := strings.Join([]string{
		"  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode",
		"   0: 00000000:18EB 00000000:0000 0A 00000000:00000000 00:00000000 00000000   100        0 7001 1 00000000 100 0 0 10 0",
	}, "\n")
	if err := os.WriteFile(filepath.Join(procRoot, "net", "tcp"), []byte(tcpBody), 0o644); err != nil {
		t.Fatalf("write tcp: %v", err)
	}

	cfgPath := filepath.Join(tmp, "config.toml")
	initialCfg := "[tailscale]\nhostname = \"test-node\"\nstate_dir = \"/tmp/tsnet\"\nlisten_port = 9100\n"
	if err := os.WriteFile(cfgPath, []byte(initialCfg), 0o644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	var out bytes.Buffer
	in := strings.NewReader("1,2\ny\n")
	err := Run(context.Background(), Options{
		ConfigPath: cfgPath,
		ProcRoot:   procRoot,
		In:         in,
		Out:        &out,
		ScanUnits: func(context.Context) ([]Candidate, error) {
			return []Candidate{
				{
					Kind:        "systemd",
					ID:          "docker",
					Name:        "Docker",
					Unit:        "docker.service",
					Description: "Docker Application Container Engine",
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("discover Run failed: %v", err)
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("reload appended config: %v", err)
	}
	if len(loaded.Services) != 2 {
		t.Fatalf("expected 2 appended services, got %d (%+v)", len(loaded.Services), loaded.Services)
	}
	if loaded.Services[0].Type != "systemd" || loaded.Services[0].Unit != "docker.service" {
		t.Fatalf("unexpected first appended service: %+v", loaded.Services[0])
	}
	if loaded.Services[1].Type != "tcp" || loaded.Services[1].Addr != "127.0.0.1:6379" || loaded.Services[1].TimeoutMs != 500 {
		t.Fatalf("unexpected second appended service: %+v", loaded.Services[1])
	}
}
