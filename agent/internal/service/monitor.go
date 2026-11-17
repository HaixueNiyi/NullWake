package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/nullwake/nullwake/agent/internal/config"
	"github.com/nullwake/nullwake/agent/internal/protocol"
)

// Monitor evaluates configured services (systemd DBus, tcp, http_json, script, push_only)
// strictly on demand and merges buffered push events from the RingBuffer.
type Monitor struct {
	configs    []config.ServiceConfig
	rb         *RingBuffer
	systemd    SystemdQuerier
	httpClient *http.Client

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
		configs: configs,
		rb:      rb,
		systemd: systemd,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
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
		case "tcp":
			st = m.collectTCP(ctx, sc)
		case "http_json":
			st = m.collectHTTPJSON(ctx, sc)
		case "script":
			st = m.collectScript(ctx, sc)
		default:
			st = m.collectPushOnly(sc.ID, sc.Name)
		}
		m.recordTransition(sc.ID, st.Status)
		st.History = m.rb.History(sc.ID)
		if st.Metrics == nil {
			st.Metrics = []protocol.ServiceMetric{}
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

// Reset closes cached DBus and idle HTTP connections when returning to idle.
func (m *Monitor) Reset() {
	if m.systemd != nil {
		_ = m.systemd.Close()
	}
	if m.httpClient != nil {
		m.httpClient.CloseIdleConnections()
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
			Metrics: []protocol.ServiceMetric{
				{Label: "Unit", Value: sc.Unit},
			},
		}
	}

	summary := state.ActiveState
	if state.SubState != "" {
		summary = fmt.Sprintf("%s (%s)", state.ActiveState, state.SubState)
	}
	metrics := []protocol.ServiceMetric{
		{Label: "Unit", Value: sc.Unit},
		{Label: "State", Value: summary},
	}
	if state.HasRSS {
		metrics = append(metrics, protocol.ServiceMetric{
			Label: "Memory",
			Value: formatBytesHuman(state.RSSBytes),
		})
	}

	return protocol.ServiceStatus{
		ID:      sc.ID,
		Name:    sc.Name,
		Status:  state.Status,
		Summary: summary,
		Metrics: metrics,
	}
}

type scriptOutput struct {
	Status  string          `json:"status"`
	Summary string          `json:"summary"`
	Metrics json.RawMessage `json:"metrics"`
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
			Metrics: []protocol.ServiceMetric{},
		}
	}

	var parsed scriptOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return protocol.ServiceStatus{
			ID:      sc.ID,
			Name:    sc.Name,
			Status:  protocol.ServiceDegraded,
			Summary: fmt.Sprintf("invalid plugin JSON: %v", err),
			Metrics: []protocol.ServiceMetric{},
		}
	}

	return protocol.ServiceStatus{
		ID:      sc.ID,
		Name:    sc.Name,
		Status:  normalizeStatus(parsed.Status),
		Summary: parsed.Summary,
		Metrics: parseScriptMetrics(parsed.Metrics),
	}
}

// parseScriptMetrics supports ordered [{"label":..,"value":..}] and legacy {"key": val} objects.
func parseScriptMetrics(raw json.RawMessage) []protocol.ServiceMetric {
	if len(raw) == 0 || string(raw) == "null" {
		return []protocol.ServiceMetric{}
	}

	var ordered []protocol.ServiceMetric
	if err := json.Unmarshal(raw, &ordered); err == nil {
		return ordered
	}

	var legacy map[string]any
	if err := json.Unmarshal(raw, &legacy); err == nil && len(legacy) > 0 {
		keys := make([]string, 0, len(legacy))
		for k := range legacy {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]protocol.ServiceMetric, 0, len(keys))
		for _, k := range keys {
			out = append(out, protocol.ServiceMetric{
				Label: k,
				Value: formatJSONScalar(legacy[k]),
			})
		}
		return out
	}

	return []protocol.ServiceMetric{}
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
		Metrics: []protocol.ServiceMetric{
			{Label: "Buffered Events", Value: strconv.Itoa(len(hist))},
		},
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

func formatBytesHuman(b uint64) string {
	const mb = 1024 * 1024
	const gb = 1024 * mb
	if b >= gb {
		return fmt.Sprintf("%.2f GB", float64(b)/float64(gb))
	}
	return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
}
