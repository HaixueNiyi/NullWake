package netstack

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/nullwake/nullwake/agent/internal/collector"
	"github.com/nullwake/nullwake/agent/internal/protocol"
)

type fakeCollector struct {
	collectCalls atomic.Int32
	resetCalls   atomic.Int32
}

func (f *fakeCollector) Collect(_ context.Context) (collector.Snapshot, error) {
	f.collectCalls.Add(1)
	return collector.Snapshot{
		CPU: protocol.CPUMetrics{UsagePercent: 12.5, FreqMHz: 3200, TempC: 45.0},
		RAM: protocol.RAMMetrics{UsedMB: 1024, TotalMB: 8192},
		Net: protocol.NetMetrics{RxBytesSec: 100, TxBytesSec: 50},
		GPU: nil,
	}, nil
}

func (f *fakeCollector) Reset() {
	f.resetCalls.Add(1)
}

type fakeMonitor struct {
	collectCalls atomic.Int32
	resetCalls   atomic.Int32
}

func (f *fakeMonitor) CollectServices(_ context.Context) []protocol.ServiceStatus {
	f.collectCalls.Add(1)
	return []protocol.ServiceStatus{
		{
			ID:      "mc",
			Name:    "Minecraft",
			Status:  protocol.ServiceRunning,
			Summary: "2 players",
			Metrics: map[string]any{"players": 2},
			History: []protocol.HistoryEvent{{TS: 1730000000, Event: "PlayerAlice joined"}},
		},
	}
}

func (f *fakeMonitor) Reset() {
	f.resetCalls.Add(1)
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http") + "/ws"
}

func TestHubSubscribePingUnsubscribeAndWatchdog(t *testing.T) {
	fc := &fakeCollector{}
	fm := &fakeMonitor{}
	hub := NewHub(fc, fm, nil)
	hub.SetWatchdogTimeout(250 * time.Millisecond)

	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL(srv.URL), nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.CloseNow()

	// Connecting alone must NOT wake collectors.
	time.Sleep(50 * time.Millisecond)
	if fc.collectCalls.Load() != 0 || hub.ActiveSubscribers() != 0 {
		t.Fatalf("expected zero collection before SUBSCRIBE, got calls=%d subs=%d", fc.collectCalls.Load(), hub.ActiveSubscribers())
	}

	// Send SUBSCRIBE.
	subMsg, _ := json.Marshal(protocol.ClientMessage{Type: protocol.MsgSubscribe, IntervalMs: 200})
	if err := conn.Write(ctx, websocket.MessageText, subMsg); err != nil {
		t.Fatalf("write SUBSCRIBE: %v", err)
	}

	// Expect immediate initial TELEMETRY frame.
	_, raw, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read initial TELEMETRY: %v", err)
	}
	var tele protocol.TelemetryMessage
	if err := json.Unmarshal(raw, &tele); err != nil {
		t.Fatalf("decode TELEMETRY: %v", err)
	}
	if tele.Type != protocol.TelemetryMessageType || tele.Data.GPU != nil || len(tele.Data.Services) != 1 {
		t.Fatalf("unexpected telemetry payload: %+v", tele)
	}

	// Keep alive across 300ms (> watchdog 250ms) by sending PINGs.
	pingMsg, _ := json.Marshal(protocol.ClientMessage{Type: protocol.MsgPing})
	for i := 0; i < 3; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := conn.Write(ctx, websocket.MessageText, pingMsg); err != nil {
			t.Fatalf("write PING: %v", err)
		}
	}
	if hub.ActiveSubscribers() != 1 {
		t.Fatalf("expected 1 active subscriber after PING keepalive, got %d", hub.ActiveSubscribers())
	}

	// Unsubscribe must halt collectors and reset baselines.
	unsubMsg, _ := json.Marshal(protocol.ClientMessage{Type: protocol.MsgUnsubscribe})
	if err := conn.Write(ctx, websocket.MessageText, unsubMsg); err != nil {
		t.Fatalf("write UNSUBSCRIBE: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && hub.ActiveSubscribers() != 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.ActiveSubscribers() != 0 {
		t.Fatalf("expected 0 active subscribers after UNSUBSCRIBE, got %d", hub.ActiveSubscribers())
	}
	if fc.resetCalls.Load() == 0 || fm.resetCalls.Load() == 0 {
		t.Fatalf("expected collector and monitor Reset() after UNSUBSCRIBE")
	}

	// Re-subscribe and omit PING -> watchdog should terminate session.
	if err := conn.Write(ctx, websocket.MessageText, subMsg); err != nil {
		t.Fatalf("re-subscribe: %v", err)
	}
	// Drain frames until server closes connection due to watchdog.
	closedByWatchdog := false
	for !closedByWatchdog {
		_, _, err := conn.Read(ctx)
		if err != nil {
			closedByWatchdog = true
		}
	}

	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) && hub.ActiveSubscribers() != 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.ActiveSubscribers() != 0 {
		t.Fatalf("expected 0 subscribers after watchdog disconnect, got %d", hub.ActiveSubscribers())
	}
}
