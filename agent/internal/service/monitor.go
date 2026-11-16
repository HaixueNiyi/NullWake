package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/nullwake/nullwake/agent/internal/config"
	"github.com/nullwake/nullwake/agent/internal/protocol"
)

// Monitor evaluates configured services (systemd DBus, script plugins, push-only)
// strictly on demand and merges buffered push events from the RingBuffer.
type Monitor struct {
	configs []config.ServiceConfig
	rb      *RingBuffer
	systemd SystemdQuerier

	mu         sync.Mutex
	lastStates map[string]string
}

// NewMonitor creates a service Monitor.
func NewMonitor(configs []config.ServiceConfig, rb *RingBuffer, systemd SystemdQuerier) *Monitor {
	if systemd == nil {
		systemd = NewDBusSystemdQuerier()
	}
	if rb == nil {
		rb = NewRingBuffer(200)
	}
	return &Monitor{
		configs:    configs,
		rb:         rb,
		systemd:    systemd,
		lastStates: make(map[string]string),
	}
}

// CollectServices queries all configured services and merges any dynamic push-only service IDs.
func (m *Monitor) CollectServices(ctx context.Context) []protocol.ServiceStatus {
	seen := make(map[string]bool, len(m.configs))
	statuses := make([]protocol.ServiceStatus, 0, len(m.configs))

	for _, sc := range m.configs {
		seen[sc.ID] = true
		var st protocol.ServiceStatus
		switch sc.Type {
		case "systemd":
			st = m.collectSystemd(ctx, sc)
		case "script":
			st = m.collectScript(ctx, sc)
		default:
			st = m.collectPushOnly(sc.ID, sc.Name)
		}
		m.recordTransition(sc.ID, st.Status)
		st.History = m.rb.History(sc.ID)
		if st.Metrics == nil {
			st.Metrics = map[string]any{}
		}
		statuses = append(statuses, st)
	}

	// Include unconfigured services that pushed events via /run/nullwake.sock.
	for _, id := range m.rb.ServiceIDs() {
		if seen[id] {
			continue
		}
		st := m.collectPushOnly(id, id)
		st.History = m.rb.History(id)
		statuses = append(statuses, st)
	}

	return statuses
}

// Reset closes any cached DBus connections when returning to idle.
func (m *Monitor) Reset() {
	if m.systemd != nil {
		_ = m.systemd.Close()
	}
}

func (m *Monitor) collectSystemd(ctx context.Context, sc config.ServiceConfig) protocol.ServiceStatus {
	qCtx, cancel := context.WithTimeout(ctx, time.Duration(sc.TimeoutMs)*time.Millisecond)
	defer cancel()

	state, err := m.systemd.QueryUnit(qCtx, sc.Unit)
	if err != nil {
		return protocol.ServiceStatus{
			ID:      sc.ID,
			Name:    sc.Name,
			Status:  protocol.ServiceDegraded,
			Summary: fmt.Sprintf("dbus error: %v", err),
			Metrics: map[string]any{"unit": sc.Unit},
		}
	}

	summary := state.ActiveState
	if state.SubState != "" {
		summary = fmt.Sprintf("%s (%s)", state.ActiveState, state.SubState)
	}
	return protocol.ServiceStatus{
		ID:      sc.ID,
		Name:    sc.Name,
		Status:  state.Status,
		Summary: summary,
		Metrics: map[string]any{
			"unit":         sc.Unit,
			"active_state": state.ActiveState,
			"sub_state":    state.SubState,
		},
	}
}

type scriptOutput struct {
	Status  string         `json:"status"`
	Summary string         `json:"summary"`
	Metrics map[string]any `json:"metrics"`
}

func (m *Monitor) collectScript(ctx context.Context, sc config.ServiceConfig) protocol.ServiceStatus {
	timeout := time.Duration(sc.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, sc.Command[0], sc.Command[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return protocol.ServiceStatus{
			ID:      sc.ID,
			Name:    sc.Name,
			Status:  protocol.ServiceDegraded,
			Summary: fmt.Sprintf("plugin error: %v", err),
			Metrics: map[string]any{},
		}
	}

	var parsed scriptOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return protocol.ServiceStatus{
			ID:      sc.ID,
			Name:    sc.Name,
			Status:  protocol.ServiceDegraded,
			Summary: fmt.Sprintf("invalid plugin JSON: %v", err),
			Metrics: map[string]any{},
		}
	}

	status := normalizeStatus(parsed.Status)
	if parsed.Metrics == nil {
		parsed.Metrics = map[string]any{}
	}
	return protocol.ServiceStatus{
		ID:      sc.ID,
		Name:    sc.Name,
		Status:  status,
		Summary: parsed.Summary,
		Metrics: parsed.Metrics,
	}
}

func (m *Monitor) collectPushOnly(id, name string) protocol.ServiceStatus {
	hist := m.rb.History(id)
	summary := "Listening on push socket"
	if len(hist) > 0 {
		summary = hist[len(hist)-1].Event
	}
	return protocol.ServiceStatus{
		ID:      id,
		Name:    name,
		Status:  protocol.ServiceRunning,
		Summary: summary,
		Metrics: map[string]any{"buffered_events": len(hist)},
		History: hist,
	}
}

func (m *Monitor) recordTransition(id, newStatus string) {
	m.mu.Lock()
	prev, ok := m.lastStates[id]
	m.lastStates[id] = newStatus
	m.mu.Unlock()

	if ok && prev != newStatus {
		m.rb.Push(id, protocol.HistoryEvent{
			TS:    time.Now().Unix(),
			Event: fmt.Sprintf("Status changed: %s -> %s", prev, newStatus),
		})
	}
}

func normalizeStatus(raw string) string {
	switch raw {
	case protocol.ServiceRunning, protocol.ServiceStopped, protocol.ServiceDegraded:
		return raw
	default:
		return protocol.ServiceDegraded
	}
}
