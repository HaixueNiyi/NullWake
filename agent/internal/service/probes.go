package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nullwake/nullwake/agent/internal/config"
	"github.com/nullwake/nullwake/agent/internal/protocol"
)

var templateTokenRe = regexp.MustCompile(`\{([a-zA-Z0-9_.-]+)\}`)

func (m *Monitor) collectTCP(ctx context.Context, sc config.ServiceConfig) protocol.ServiceStatus {
	timeout := time.Duration(sc.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(dialCtx, "tcp", sc.Addr)
	if err != nil {
		return protocol.ServiceStatus{
			ID:      sc.ID,
			Name:    sc.Name,
			Status:  protocol.ServiceStopped,
			Summary: fmt.Sprintf("Unreachable (%s)", sc.Addr),
			Metrics: []protocol.ServiceMetric{
				{Label: "Address", Value: sc.Addr},
				{Label: "State", Value: "unreachable"},
			},
		}
	}
	_ = conn.Close()

	latency := time.Since(start)
	latencyText := fmt.Sprintf("%d ms", latency.Milliseconds())
	if latency < time.Millisecond {
		latencyText = "<1 ms"
	}

	return protocol.ServiceStatus{
		ID:      sc.ID,
		Name:    sc.Name,
		Status:  protocol.ServiceRunning,
		Summary: fmt.Sprintf("Listening on %s (%s)", sc.Addr, latencyText),
		Metrics: []protocol.ServiceMetric{
			{Label: "Address", Value: sc.Addr},
			{Label: "Latency", Value: latencyText},
		},
	}
}

func (m *Monitor) collectHTTPJSON(ctx context.Context, sc config.ServiceConfig) protocol.ServiceStatus {
	timeout := time.Duration(sc.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, sc.URL, nil)
	if err != nil {
		return protocol.ServiceStatus{
			ID:      sc.ID,
			Name:    sc.Name,
			Status:  protocol.ServiceDegraded,
			Summary: fmt.Sprintf("Invalid URL: %v", err),
			Metrics: []protocol.ServiceMetric{},
		}
	}
	req.Header.Set("Accept", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return protocol.ServiceStatus{
			ID:      sc.ID,
			Name:    sc.Name,
			Status:  protocol.ServiceStopped,
			Summary: fmt.Sprintf("Endpoint unreachable: %v", err),
			Metrics: []protocol.ServiceMetric{
				{Label: "Endpoint", Value: sc.URL},
			},
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return protocol.ServiceStatus{
			ID:      sc.ID,
			Name:    sc.Name,
			Status:  protocol.ServiceDegraded,
			Summary: fmt.Sprintf("Read error: %v", err),
			Metrics: []protocol.ServiceMetric{},
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return protocol.ServiceStatus{
			ID:      sc.ID,
			Name:    sc.Name,
			Status:  protocol.ServiceDegraded,
			Summary: fmt.Sprintf("HTTP %d", resp.StatusCode),
			Metrics: []protocol.ServiceMetric{
				{Label: "HTTP Status", Value: strconv.Itoa(resp.StatusCode)},
			},
		}
	}

	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return protocol.ServiceStatus{
			ID:      sc.ID,
			Name:    sc.Name,
			Status:  protocol.ServiceDegraded,
			Summary: fmt.Sprintf("Invalid JSON: %v", err),
			Metrics: []protocol.ServiceMetric{},
		}
	}

	metrics := make([]protocol.ServiceMetric, 0, len(sc.Fields))
	for _, f := range sc.Fields {
		val, ok := lookupDotPath(decoded, f.Path)
		valStr := "--"
		if ok {
			valStr = formatJSONScalar(val)
		}
		metrics = append(metrics, protocol.ServiceMetric{
			Label: f.Label,
			Value: valStr,
		})
	}

	summary := renderSummaryTemplate(sc.SummaryTemplate, decoded)
	if summary == "" {
		if len(metrics) > 0 {
			summary = fmt.Sprintf("%s: %s", metrics[0].Label, metrics[0].Value)
		} else {
			summary = fmt.Sprintf("HTTP %d OK", resp.StatusCode)
		}
	}

	return protocol.ServiceStatus{
		ID:      sc.ID,
		Name:    sc.Name,
		Status:  protocol.ServiceRunning,
		Summary: summary,
		Metrics: metrics,
	}
}

// lookupDotPath resolves a dot-separated path (supporting object keys and slice indices) in decoded JSON.
func lookupDotPath(root any, dotPath string) (any, bool) {
	if root == nil || dotPath == "" {
		return nil, false
	}
	cur := root
	for _, seg := range strings.Split(dotPath, ".") {
		switch node := cur.(type) {
		case map[string]any:
			next, ok := node[seg]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

func renderSummaryTemplate(tmpl string, root any) string {
	if strings.TrimSpace(tmpl) == "" {
		return ""
	}
	return templateTokenRe.ReplaceAllStringFunc(tmpl, func(match string) string {
		key := match[1 : len(match)-1]
		if val, ok := lookupDotPath(root, key); ok {
			return formatJSONScalar(val)
		}
		return "--"
	})
}

func formatJSONScalar(v any) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case string:
		return val
	case bool:
		return strconv.FormatBool(val)
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", val)
	}
}
