package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
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
	scriptBody := "#!/bin/sh\necho '{\"status\":\"running\",\"summary\":\"4/20 players\",\"metrics\":[{\"label\":\"Players\",\"value\":\"4/20\"},{\"label\":\"TPS\",\"value\":\"19.9\"}]}'\n"
	if err := os.WriteFile(scriptPath, []byte(scriptBody), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	legacyScriptPath := filepath.Join(dir, "legacy.sh")
	legacyBody := "#!/bin/sh\necho '{\"status\":\"running\",\"summary\":\"ok\",\"metrics\":{\"alpha\":1,\"beta\":\"two\"}}'\n"
	if err := os.WriteFile(legacyScriptPath, []byte(legacyBody), 0o755); err != nil {
		t.Fatalf("write legacy script: %v", err)
	}

	rb := NewRingBuffer(10)
	rb.Push("dynamic-job", protocol.HistoryEvent{TS: 1730000010, Event: "Backup finished"})

	mockSys := &mockSystemdQuerier{
		state: UnitState{
			Status:      protocol.ServiceRunning,
			ActiveState: "active",
			SubState:    "running",
			RSSBytes:    64 * 1024 * 1024,
			HasRSS:      true,
		},
	}

	mon := NewMonitor([]config.ServiceConfig{
		{ID: "docker", Name: "Docker", Type: "systemd", Unit: "docker.service", TimeoutMs: 1000},
		{ID: "mc", Name: "Minecraft", Type: "script", Command: []string{scriptPath}, TimeoutMs: 1000},
		{ID: "legacy", Name: "Legacy", Type: "script", Command: []string{legacyScriptPath}, TimeoutMs: 1000},
	}, rb, mockSys)

	statuses := mon.CollectServices(context.Background())
	if len(statuses) != 4 {
		t.Fatalf("expected 4 services (3 configured + 1 dynamic push), got %d", len(statuses))
	}
	if statuses[0].Status != protocol.ServiceRunning || statuses[0].Summary != "active (running)" {
		t.Fatalf("unexpected systemd status: %+v", statuses[0])
	}
	foundMemory := false
	for _, metric := range statuses[0].Metrics {
		if metric.Label == "Memory" && metric.Value == "64.0 MB" {
			foundMemory = true
		}
	}
	if !foundMemory {
		t.Fatalf("expected systemd RSS Memory metric, got: %+v", statuses[0].Metrics)
	}

	if statuses[1].Summary != "4/20 players" || len(statuses[1].Metrics) != 2 ||
		statuses[1].Metrics[0].Label != "Players" || statuses[1].Metrics[0].Value != "4/20" ||
		statuses[1].Metrics[1].Label != "TPS" || statuses[1].Metrics[1].Value != "19.9" {
		t.Fatalf("unexpected ordered script status: %+v", statuses[1])
	}
	if len(statuses[2].Metrics) != 2 || statuses[2].Metrics[0].Label != "alpha" || statuses[2].Metrics[1].Label != "beta" {
		t.Fatalf("unexpected legacy script metrics: %+v", statuses[2].Metrics)
	}
	if statuses[3].ID != "dynamic-job" || len(statuses[3].History) != 1 {
		t.Fatalf("unexpected dynamic push service: %+v", statuses[3])
	}

	// Simulate systemd unit failure transition -> should log transition to ring buffer.
	mockSys.state = UnitState{Status: protocol.ServiceDegraded, ActiveState: "failed", SubState: "failed"}
	statuses = mon.CollectServices(context.Background())
	if statuses[0].Status != protocol.ServiceDegraded || len(statuses[0].History) != 1 {
		t.Fatalf("expected transition event recorded in history, got: %+v", statuses[0])
	}
}

func TestMonitorTCPProbeAndHistoryMerge(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	openAddr := ln.Addr().String()
	defer ln.Close()

	// Allocate and immediately close a listener to get a guaranteed closed port.
	closedLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen closed tcp: %v", err)
	}
	closedAddr := closedLn.Addr().String()
	_ = closedLn.Close()

	rb := NewRingBuffer(10)
	rb.Push("pg", protocol.HistoryEvent{TS: 1730000100, Event: "Checkpoint complete"})

	mon := NewMonitor([]config.ServiceConfig{
		{ID: "pg", Name: "PostgreSQL", Type: "tcp", Addr: openAddr, TimeoutMs: 500},
		{ID: "dead", Name: "Dead Port", Type: "tcp", Addr: closedAddr, TimeoutMs: 300},
	}, rb, nil)

	statuses := mon.CollectServices(context.Background())
	if len(statuses) != 2 {
		t.Fatalf("expected 2 tcp services, got %d", len(statuses))
	}
	if statuses[0].Status != protocol.ServiceRunning {
		t.Fatalf("expected open tcp service running, got %+v", statuses[0])
	}
	if len(statuses[0].Metrics) != 2 || statuses[0].Metrics[0].Value != openAddr || statuses[0].Metrics[1].Label != "Latency" {
		t.Fatalf("unexpected open tcp metrics: %+v", statuses[0].Metrics)
	}
	if len(statuses[0].History) != 1 || statuses[0].History[0].Event != "Checkpoint complete" {
		t.Fatalf("expected unified ring-buffer history merged into tcp service, got %+v", statuses[0].History)
	}

	if statuses[1].Status != protocol.ServiceStopped {
		t.Fatalf("expected closed tcp service stopped, got %+v", statuses[1])
	}
}

func TestMonitorHTTPJSONProbe(t *testing.T) {
	healthySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"players":{"online":7,"max":20},"tps":19.85,"nested":{"ok":true}}`))
	}))
	defer healthySrv.Close()

	failingSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"overloaded"}`))
	}))
	defer failingSrv.Close()

	mon := NewMonitor([]config.ServiceConfig{
		{
			ID:              "mc-http",
			Name:            "Minecraft HTTP",
			Type:            "http_json",
			URL:             healthySrv.URL,
			TimeoutMs:       1000,
			SummaryTemplate: "{players.online}/{players.max} online",
			Fields: []config.FieldMapping{
				{Path: "players.online", Label: "Players"},
				{Path: "tps", Label: "TPS"},
				{Path: "missing.key", Label: "Missing"},
			},
		},
		{
			ID:        "bad-http",
			Name:      "Unhealthy API",
			Type:      "http_json",
			URL:       failingSrv.URL,
			TimeoutMs: 1000,
		},
	}, NewRingBuffer(10), nil)

	statuses := mon.CollectServices(context.Background())
	if len(statuses) != 2 {
		t.Fatalf("expected 2 http_json services, got %d", len(statuses))
	}
	if statuses[0].Status != protocol.ServiceRunning || statuses[0].Summary != "7/20 online" {
		t.Fatalf("unexpected healthy http_json status: %+v", statuses[0])
	}
	if len(statuses[0].Metrics) != 3 ||
		statuses[0].Metrics[0].Label != "Players" || statuses[0].Metrics[0].Value != "7" ||
		statuses[0].Metrics[1].Label != "TPS" || statuses[0].Metrics[1].Value != "19.85" ||
		statuses[0].Metrics[2].Label != "Missing" || statuses[0].Metrics[2].Value != "--" {
		t.Fatalf("unexpected ordered http_json metrics: %+v", statuses[0].Metrics)
	}
	if statuses[1].Status != protocol.ServiceDegraded {
		t.Fatalf("expected HTTP 503 service degraded, got %+v", statuses[1])
	}
}
