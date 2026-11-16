package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nullwake/nullwake/agent/internal/config"
	"github.com/nullwake/nullwake/agent/internal/protocol"
)

func TestRingBufferWrapAround(t *testing.T) {
	rb := NewRingBuffer(3)
	for i := 1; i <= 5; i++ {
		rb.Push("mc", protocol.HistoryEvent{
			TS:    int64(100 + i),
			Event: fmt.Sprintf("event-%d", i),
		})
	}

	got := rb.History("mc")
	if len(got) != 3 {
		t.Fatalf("expected 3 buffered events, got %d", len(got))
	}
	want := []string{"event-3", "event-4", "event-5"}
	for i, w := range want {
		if got[i].Event != w {
			t.Fatalf("slot %d: got %q, want %q", i, got[i].Event, w)
		}
	}
}

func TestUnixSocketIngestionAndPermissions(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "nullwake.sock")
	rb := NewRingBuffer(200)

	srv, err := NewSocketServer(sockPath, 0o666, rb)
	if err != nil {
		t.Fatalf("start socket server: %v", err)
	}
	defer srv.Close()

	info, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	// Check permissions (note umask doesn't affect explicit os.Chmod).
	if perm := info.Mode().Perm(); perm != 0o666 {
		t.Fatalf("expected socket mode 0666, got %04o", perm)
	}

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial unix socket: %v", err)
	}
	_, err = conn.Write([]byte(`{"service_id":"mc","event":"PlayerAlice joined","ts":1730000000}` + "\n" +
		`{"service_id":"mc","event":"PlayerBob joined"}` + "\n"))
	if err != nil {
		t.Fatalf("write events: %v", err)
	}
	_ = conn.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hist := rb.History("mc")
		if len(hist) == 2 {
			if hist[0].TS != 1730000000 || hist[0].Event != "PlayerAlice joined" {
				t.Fatalf("unexpected first event: %+v", hist[0])
			}
			if hist[1].TS == 0 || hist[1].Event != "PlayerBob joined" {
				t.Fatalf("unexpected second event: %+v", hist[1])
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for socket events in ring buffer")
}

type mockSystemdQuerier struct {
	state UnitState
	err   error
}

func (m *mockSystemdQuerier) QueryUnit(_ context.Context, _ string) (UnitState, error) {
	return m.state, m.err
}

func (m *mockSystemdQuerier) Close() error { return nil }

func TestMonitorScriptSystemdAndDynamicPush(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "plugin.sh")
	scriptBody := "#!/bin/sh\necho '{\"status\":\"running\",\"summary\":\"4/20 players\",\"metrics\":{\"players\":4,\"tps\":19.9}}'\n"
	if err := os.WriteFile(scriptPath, []byte(scriptBody), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	rb := NewRingBuffer(10)
	rb.Push("dynamic-job", protocol.HistoryEvent{TS: 1730000010, Event: "Backup finished"})

	mockSys := &mockSystemdQuerier{
		state: UnitState{
			Status:      protocol.ServiceRunning,
			ActiveState: "active",
			SubState:    "running",
		},
	}

	mon := NewMonitor([]config.ServiceConfig{
		{ID: "docker", Name: "Docker", Type: "systemd", Unit: "docker.service", TimeoutMs: 1000},
		{ID: "mc", Name: "Minecraft", Type: "script", Command: []string{scriptPath}, TimeoutMs: 1000},
	}, rb, mockSys)

	statuses := mon.CollectServices(context.Background())
	if len(statuses) != 3 {
		t.Fatalf("expected 3 services (2 configured + 1 dynamic push), got %d", len(statuses))
	}
	if statuses[0].Status != protocol.ServiceRunning || statuses[0].Summary != "active (running)" {
		t.Fatalf("unexpected systemd status: %+v", statuses[0])
	}
	if statuses[1].Summary != "4/20 players" || statuses[1].Metrics["players"] != float64(4) {
		t.Fatalf("unexpected script status: %+v", statuses[1])
	}
	if statuses[2].ID != "dynamic-job" || len(statuses[2].History) != 1 {
		t.Fatalf("unexpected dynamic push service: %+v", statuses[2])
	}

	// Simulate systemd unit failure transition -> should log transition to ring buffer.
	mockSys.state = UnitState{Status: protocol.ServiceDegraded, ActiveState: "failed", SubState: "failed"}
	statuses = mon.CollectServices(context.Background())
	if statuses[0].Status != protocol.ServiceDegraded || len(statuses[0].History) != 1 {
		t.Fatalf("expected transition event recorded in history, got: %+v", statuses[0])
	}
}
