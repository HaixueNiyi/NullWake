package protocol

import "encoding/json"

// ClientMessageType enumerates valid commands sent from the Android client to the Linux agent.
type ClientMessageType string

const (
	MsgSubscribe   ClientMessageType = "SUBSCRIBE"
	MsgPing        ClientMessageType = "PING"
	MsgUnsubscribe ClientMessageType = "UNSUBSCRIBE"
)

// ClientMessage represents a control message sent by the client over WebSocket.
type ClientMessage struct {
	Type       ClientMessageType `json:"type"`
	IntervalMs int               `json:"interval_ms,omitempty"`
}

// TelemetryMessageType is the fixed message type for agent telemetry payloads.
const TelemetryMessageType = "TELEMETRY"

// TelemetryMessage is the envelope emitted by the agent on each active tick.
type TelemetryMessage struct {
	Type      string        `json:"type"`
	Timestamp int64         `json:"timestamp"`
	Data      TelemetryData `json:"data"`
}

// NewTelemetryMessage constructs a TelemetryMessage with the TELEMETRY type set.
func NewTelemetryMessage(ts int64, data TelemetryData) TelemetryMessage {
	if data.Services == nil {
		data.Services = []ServiceStatus{}
	}
	return TelemetryMessage{
		Type:      TelemetryMessageType,
		Timestamp: ts,
		Data:      data,
	}
}

// TelemetryData holds all hardware and service metrics in a telemetry snapshot.
type TelemetryData struct {
	CPU      CPUMetrics      `json:"cpu"`
	RAM      RAMMetrics      `json:"ram"`
	Net      NetMetrics      `json:"net"`
	GPU      *GPUMetrics     `json:"gpu"`
	Services []ServiceStatus `json:"services"`
}

// CPUMetrics describes CPU utilization, frequency, and package temperature.
type CPUMetrics struct {
	UsagePercent float64 `json:"usage_percent"`
	FreqMHz      int     `json:"freq_mhz"`
	TempC        float64 `json:"temp_c"`
}

// RAMMetrics describes system RAM and swap usage in megabytes.
type RAMMetrics struct {
	UsedMB     uint64 `json:"used_mb"`
	TotalMB    uint64 `json:"total_mb"`
	SwapUsedMB uint64 `json:"swap_used_mb"`
}

// NetMetrics describes aggregate non-loopback network throughput in bytes/sec.
type NetMetrics struct {
	RxBytesSec uint64 `json:"rx_bytes_sec"`
	TxBytesSec uint64 `json:"tx_bytes_sec"`
}

// GPUMetrics describes AMD or Intel GPU metrics. Nil when the host is headless.
type GPUMetrics struct {
	Vendor       string  `json:"vendor"`
	Name         string  `json:"name"`
	UsagePercent float64 `json:"usage_percent"`
	VRAMUsedMB   uint64  `json:"vram_used_mb"`
	VRAMTotalMB  uint64  `json:"vram_total_mb"`
	TempC        float64 `json:"temp_c"`
}

// ServiceStatus constants define normalized service health states.
const (
	ServiceRunning  = "running"
	ServiceStopped  = "stopped"
	ServiceDegraded = "degraded"
)

// ServiceStatus describes a monitored service's current state, custom metrics, and recent events.
type ServiceStatus struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Status  string         `json:"status"`
	Summary string         `json:"summary"`
	Metrics map[string]any `json:"metrics"`
	History []HistoryEvent `json:"history"`
}

// HistoryEvent represents a timestamped event stored in the in-memory ring buffer.
type HistoryEvent struct {
	TS    int64  `json:"ts"`
	Event string `json:"event"`
}

// PushEvent represents a single-line JSON event pushed via the Unix domain socket.
type PushEvent struct {
	ServiceID string `json:"service_id"`
	Event     string `json:"event"`
	TS        int64  `json:"ts,omitempty"`
}

// MarshalJSON ensures Services is never serialized as null.
func (d TelemetryData) MarshalJSON() ([]byte, error) {
	type alias TelemetryData
	copyData := alias(d)
	if copyData.Services == nil {
		copyData.Services = []ServiceStatus{}
	}
	return json.Marshal(copyData)
}
