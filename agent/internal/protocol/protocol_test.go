package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClientMessageJSON(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want ClientMessage
	}{
		{
			name: "subscribe",
			raw:  `{"type":"SUBSCRIBE","interval_ms":1000}`,
			want: ClientMessage{Type: MsgSubscribe, IntervalMs: 1000},
		},
		{
			name: "ping",
			raw:  `{"type":"PING"}`,
			want: ClientMessage{Type: MsgPing},
		},
		{
			name: "unsubscribe",
			raw:  `{"type":"UNSUBSCRIBE"}`,
			want: ClientMessage{Type: MsgUnsubscribe},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got ClientMessage
			if err := json.Unmarshal([]byte(tc.raw), &got); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			if string(encoded) != tc.raw {
				t.Fatalf("encoded %s, want %s", string(encoded), tc.raw)
			}
		})
	}
}

func TestTelemetryMessageHeadlessGPUNull(t *testing.T) {
	msg := NewTelemetryMessage(1730000000, TelemetryData{
		CPU: CPUMetrics{UsagePercent: 12.5, FreqMHz: 3600, TempC: 48.2},
		RAM: RAMMetrics{UsedMB: 2048, TotalMB: 16384, SwapUsedMB: 0},
		Net: NetMetrics{RxBytesSec: 1024, TxBytesSec: 512},
		GPU: nil,
	})

	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	raw := string(b)
	if !strings.Contains(raw, `"gpu":null`) {
		t.Fatalf("expected gpu to serialize as null, got: %s", raw)
	}
	if !strings.Contains(raw, `"services":[]`) {
		t.Fatalf("expected services to serialize as empty array, got: %s", raw)
	}
	if !strings.Contains(raw, `"type":"TELEMETRY"`) {
		t.Fatalf("expected type TELEMETRY, got: %s", raw)
	}
}

func TestTelemetryMessageWithGPUAndServices(t *testing.T) {
	msg := NewTelemetryMessage(1730000001, TelemetryData{
		CPU: CPUMetrics{UsagePercent: 45.0, FreqMHz: 4200, TempC: 61.0},
		RAM: RAMMetrics{UsedMB: 8192, TotalMB: 32768, SwapUsedMB: 128},
		Net: NetMetrics{RxBytesSec: 40960, TxBytesSec: 20480},
		GPU: &GPUMetrics{
			Vendor:       "amd",
			Name:         "AMD Radeon RX 6700 XT",
			UsagePercent: 72.3,
			VRAMUsedMB:   4096,
			VRAMTotalMB:  12288,
			TempC:        64.5,
		},
		Services: []ServiceStatus{
			{
				ID:      "mc",
				Name:    "Minecraft Server",
				Status:  ServiceRunning,
				Summary: "3/20 players online",
				Metrics: []ServiceMetric{
					{Label: "Online Players", Value: "3"},
					{Label: "TPS", Value: "20.0"},
				},
				History: []HistoryEvent{{TS: 1730000000, Event: "PlayerAlice joined"}},
			},
		},
	})

	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	raw := string(b)
	playersIdx := strings.Index(raw, `"Online Players"`)
	tpsIdx := strings.Index(raw, `"TPS"`)
	if playersIdx < 0 || tpsIdx < 0 || playersIdx > tpsIdx {
		t.Fatalf("expected deterministic metric order in JSON: %s", raw)
	}

	var decoded TelemetryMessage
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.Data.GPU == nil {
		t.Fatalf("expected non-nil GPU after roundtrip")
	}
	if decoded.Data.GPU.Vendor != "amd" || decoded.Data.GPU.VRAMTotalMB != 12288 {
		t.Fatalf("unexpected GPU metrics: %+v", decoded.Data.GPU)
	}
	if len(decoded.Data.Services) != 1 || decoded.Data.Services[0].ID != "mc" {
		t.Fatalf("unexpected services: %+v", decoded.Data.Services)
	}
	if len(decoded.Data.Services[0].Metrics) != 2 || decoded.Data.Services[0].Metrics[0].Label != "Online Players" || decoded.Data.Services[0].Metrics[0].Value != "3" {
		t.Fatalf("unexpected ordered metrics: %+v", decoded.Data.Services[0].Metrics)
	}
	if len(decoded.Data.Services[0].History) != 1 || decoded.Data.Services[0].History[0].Event != "PlayerAlice joined" {
		t.Fatalf("unexpected history: %+v", decoded.Data.Services[0].History)
	}
}
