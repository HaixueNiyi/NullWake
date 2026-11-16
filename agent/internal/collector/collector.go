package collector

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/nullwake/nullwake/agent/internal/protocol"
)

// Option configures a Collector instance.
type Option func(*Collector)

// WithRoots overrides /proc and /sys paths (primarily for deterministic unit testing).
func WithRoots(procRoot, sysRoot string) Option {
	return func(c *Collector) {
		if procRoot != "" {
			c.procRoot = procRoot
		}
		if sysRoot != "" {
			c.sysRoot = sysRoot
		}
	}
}

// Collector gathers Linux hardware telemetry strictly on demand without idle polling.
type Collector struct {
	mu       sync.Mutex
	procRoot string
	sysRoot  string
	cpu      *cpuCollector
	ram      *ramCollector
	net      *netCollector
	gpu      *gpuCollector
}

// New creates a Collector. No background goroutines or file descriptors are opened until Collect is called.
func New(opts ...Option) *Collector {
	c := &Collector{
		procRoot: "/proc",
		sysRoot:  "/sys",
	}
	for _, opt := range opts {
		opt(c)
	}
	c.cpu = newCPUCollector(c.procRoot, c.sysRoot)
	c.ram = newRAMCollector(c.procRoot)
	c.net = newNetCollector(c.procRoot)
	c.gpu = newGPUCollector(c.sysRoot)
	return c
}

// Snapshot holds hardware metrics for a single telemetry sample.
type Snapshot struct {
	CPU protocol.CPUMetrics
	RAM protocol.RAMMetrics
	Net protocol.NetMetrics
	GPU *protocol.GPUMetrics
}

// Collect reads current CPU, RAM, Network, and GPU metrics on demand.
func (c *Collector) Collect(ctx context.Context) (Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cpuMetrics, err := c.cpu.collect()
	if err != nil {
		return Snapshot{}, err
	}
	ramMetrics, err := c.ram.collect()
	if err != nil {
		return Snapshot{}, err
	}
	netMetrics, err := c.net.collect()
	if err != nil {
		return Snapshot{}, err
	}
	gpuMetrics := c.gpu.collect(ctx)

	return Snapshot{
		CPU: cpuMetrics,
		RAM: ramMetrics,
		Net: netMetrics,
		GPU: gpuMetrics,
	}, nil
}

// Reset clears delta baselines and releases any open PMU descriptors so the agent returns to idle.
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cpu.reset()
	c.net.reset()
	c.gpu.reset()
}

// Close releases collector resources.
func (c *Collector) Close() error {
	c.Reset()
	return nil
}

func readTrimmed(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func readUint64File(path string) (uint64, error) {
	s, err := readTrimmed(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(s, 10, 64)
}

func readFloat64File(path string) (float64, error) {
	s, err := readTrimmed(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(s, 64)
}
