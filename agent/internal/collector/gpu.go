package collector

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nullwake/nullwake/agent/internal/protocol"
)

type gpuKind int

const (
	gpuUnknown gpuKind = iota
	gpuNone
	gpuAMD
	gpuIntel
)

type gpuCollector struct {
	sysRoot  string
	kind     gpuKind
	cardPath string
	name     string

	// Intel PMU tracking (opened only when subscribed, closed on Reset).
	pmuFD         int
	pmuFailed     bool
	prevBusyNs    uint64
	prevSampleAt  time.Time
	hasPrevSample bool

	// Intel RC6 residency tracking (unprivileged sysfs fallback).
	prevRC6Ms    uint64
	prevRC6At    time.Time
	hasPrevRC6   bool
	rc6NowFn     func() time.Time
}

func newGPUCollector(sysRoot string) *gpuCollector {
	return &gpuCollector{
		sysRoot:  sysRoot,
		pmuFD:    -1,
		rc6NowFn: time.Now,
	}
}

func (g *gpuCollector) detect() {
	if g.kind != gpuUnknown {
		return
	}

	cards, err := filepath.Glob(filepath.Join(g.sysRoot, "class/drm/card*"))
	if err != nil || len(cards) == 0 {
		g.kind = gpuNone
		return
	}
	sort.Strings(cards)

	for _, card := range cards {
		base := filepath.Base(card)
		// Skip connector sub-entries such as card0-HDMI-A-1
		if strings.Contains(base, "-") {
			continue
		}

		vendor, _ := readTrimmed(filepath.Join(card, "device/vendor"))
		vendor = strings.ToLower(vendor)

		if vendor == "0x1002" || fileExists(filepath.Join(card, "device/gpu_busy_percent")) {
			g.kind = gpuAMD
			g.cardPath = card
			g.name = firstNonEmptyFile(
				filepath.Join(card, "device/product_name"),
				filepath.Join(card, "device/marketing_name"),
			)
			if g.name == "" {
				g.name = "AMD Radeon GPU"
			}
			return
		}

		if vendor == "0x8086" ||
			fileExists(filepath.Join(card, "gt_cur_freq_mhz")) ||
			fileExists(filepath.Join(card, "gt/gt0/rps_cur_freq_mhz")) {
			g.kind = gpuIntel
			g.cardPath = card
			g.name = firstNonEmptyFile(
				filepath.Join(card, "device/label"),
				filepath.Join(card, "device/product_name"),
			)
			if g.name == "" {
				g.name = "Intel Integrated GPU"
			}
			return
		}
	}

	g.kind = gpuNone
}

func (g *gpuCollector) collect(ctx context.Context) *protocol.GPUMetrics {
	g.detect()
	switch g.kind {
	case gpuAMD:
		return g.collectAMD()
	case gpuIntel:
		return g.collectIntel(ctx)
	default:
		return nil
	}
}

func (g *gpuCollector) collectAMD() *protocol.GPUMetrics {
	usage, _ := readFloat64File(filepath.Join(g.cardPath, "device/gpu_busy_percent"))
	vramUsedBytes, _ := readUint64File(filepath.Join(g.cardPath, "device/mem_info_vram_used"))
	vramTotalBytes, _ := readUint64File(filepath.Join(g.cardPath, "device/mem_info_vram_total"))
	tempC := g.readDeviceHwmonTemp(g.cardPath)

	return &protocol.GPUMetrics{
		Vendor:       "amd",
		Name:         g.name,
		UsagePercent: round2(clampPercent(usage)),
		VRAMUsedMB:   vramUsedBytes / (1024 * 1024),
		VRAMTotalMB:  vramTotalBytes / (1024 * 1024),
		TempC:        round1(tempC),
	}
}

func (g *gpuCollector) collectIntel(ctx context.Context) *protocol.GPUMetrics {
	usage, ok := g.readIntelPMUUsage()
	if !ok {
		if rc6Usage, rc6OK := g.readIntelRC6Usage(); rc6OK {
			usage = rc6Usage
			ok = true
		}
	}
	if !ok && g.sysRoot == "/sys" {
		if topUsage, topOK := g.readIntelGPUTop(ctx); topOK {
			usage = topUsage
			ok = true
		}
	}
	if !ok {
		usage = g.estimateIntelUsageFromFreq()
	}

	tempC := g.readDeviceHwmonTemp(g.cardPath)
	if tempC == 0 {
		// iGPU shares die thermal zone with CPU package when dedicated card hwmon is absent.
		tempC = (&cpuCollector{sysRoot: g.sysRoot}).readTempC()
	}

	return &protocol.GPUMetrics{
		Vendor:       "intel",
		Name:         g.name,
		UsagePercent: round2(clampPercent(usage)),
		VRAMUsedMB:   0,
		VRAMTotalMB:  0,
		TempC:        round1(tempC),
	}
}

// readIntelRC6Usage computes i915 active GPU busy % from kernel RC6 idle residency ms
// without requiring CAP_PERFMON or external tools.
func (g *gpuCollector) readIntelRC6Usage() (float64, bool) {
	rc6Ms, err := readFirstUint64(
		filepath.Join(g.cardPath, "gt/gt0/rc6_residency_ms"),
		filepath.Join(g.cardPath, "power/rc6_residency_ms"),
	)
	if err != nil {
		return 0, false
	}

	now := g.rc6NowFn()
	var usage float64
	if g.hasPrevRC6 {
		elapsedMs := float64(now.Sub(g.prevRC6At).Milliseconds())
		if elapsedMs > 0 && rc6Ms >= g.prevRC6Ms {
			deltaRC6Ms := float64(rc6Ms - g.prevRC6Ms)
			// Clamp idle ratio so minor kernel timer jitter never yields negative busy %.
			if deltaRC6Ms >= elapsedMs {
				usage = 0
			} else {
				usage = (1.0 - deltaRC6Ms/elapsedMs) * 100.0
			}
		}
	}
	g.prevRC6Ms = rc6Ms
	g.prevRC6At = now
	g.hasPrevRC6 = true
	return clampPercent(usage), true
}

func (g *gpuCollector) estimateIntelUsageFromFreq() float64 {
	// Prefer rps_act_freq_mhz (actual clocked frequency, 0 when RC6-gated) over requested cur_freq.
	curFreq, err1 := readFirstUint64(
		filepath.Join(g.cardPath, "gt/gt0/rps_act_freq_mhz"),
		filepath.Join(g.cardPath, "gt_act_freq_mhz"),
		filepath.Join(g.cardPath, "gt/gt0/rps_cur_freq_mhz"),
		filepath.Join(g.cardPath, "gt_cur_freq_mhz"),
	)
	maxFreq, err2 := readFirstUint64(
		filepath.Join(g.cardPath, "gt_max_freq_mhz"),
		filepath.Join(g.cardPath, "gt/gt0/rps_max_freq_mhz"),
	)
	if err1 != nil || err2 != nil || maxFreq == 0 {
		return 0
	}
	return float64(curFreq) / float64(maxFreq) * 100.0
}

func (g *gpuCollector) readIntelGPUTop(ctx context.Context) (float64, bool) {
	bin, err := exec.LookPath("intel_gpu_top")
	if err != nil {
		return 0, false
	}
	runCtx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(runCtx, bin, "-J", "-s", "100", "-o", "-")
	out, err := cmd.Output()
	if len(out) == 0 && err != nil {
		return 0, false
	}

	return parseIntelGPUTopJSON(out)
}

func parseIntelGPUTopJSON(raw []byte) (float64, bool) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return 0, false
	}
	search := text
	for {
		idx := strings.LastIndex(search, `"engines"`)
		if idx < 0 {
			return 0, false
		}
		rest := search[idx+len(`"engines"`):]
		if braceOffset := strings.IndexByte(rest, '{'); braceOffset >= 0 {
			var engines map[string]struct {
				Busy float64 `json:"busy"`
			}
			dec := json.NewDecoder(strings.NewReader(rest[braceOffset:]))
			if err := dec.Decode(&engines); err == nil && len(engines) > 0 {
				return maxEngineBusy(engines), true
			}
		}
		search = search[:idx]
	}
}

func maxEngineBusy(engines map[string]struct {
	Busy float64 `json:"busy"`
}) float64 {
	var maxVal float64
	for _, eng := range engines {
		if eng.Busy > maxVal {
			maxVal = eng.Busy
		}
	}
	return maxVal
}

func (g *gpuCollector) readDeviceHwmonTemp(cardPath string) float64 {
	inputs, _ := filepath.Glob(filepath.Join(cardPath, "device/hwmon/hwmon*/temp1_input"))
	for _, p := range inputs {
		if milli, err := readFloat64File(p); err == nil && milli > 0 && milli <= 130000 {
			return milli / 1000.0
		}
	}
	return 0
}

func (g *gpuCollector) reset() {
	g.closePMU()
	g.pmuFailed = false
	g.prevBusyNs = 0
	g.prevSampleAt = time.Time{}
	g.hasPrevSample = false
	g.prevRC6Ms = 0
	g.prevRC6At = time.Time{}
	g.hasPrevRC6 = false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func firstNonEmptyFile(paths ...string) string {
	for _, p := range paths {
		if s, err := readTrimmed(p); err == nil && s != "" {
			return s
		}
	}
	return ""
}

func readFirstUint64(paths ...string) (uint64, error) {
	var lastErr error
	for _, p := range paths {
		v, err := readUint64File(p)
		if err == nil {
			return v, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return 0, lastErr
}

func parseEventConfig(raw string) (uint64, bool) {
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "event=") {
			valStr := strings.TrimPrefix(part, "event=")
			v, err := strconv.ParseUint(valStr, 0, 64)
			return v, err == nil
		}
		if strings.HasPrefix(part, "config=") {
			valStr := strings.TrimPrefix(part, "config=")
			v, err := strconv.ParseUint(valStr, 0, 64)
			return v, err == nil
		}
	}
	return 0, false
}

func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
