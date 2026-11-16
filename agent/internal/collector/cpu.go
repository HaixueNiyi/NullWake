package collector

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nullwake/nullwake/agent/internal/protocol"
)

type cpuStatSample struct {
	total uint64
	idle  uint64
}

type cpuCollector struct {
	procRoot  string
	sysRoot   string
	prev      cpuStatSample
	prevValid bool
}

func newCPUCollector(procRoot, sysRoot string) *cpuCollector {
	return &cpuCollector{
		procRoot: procRoot,
		sysRoot:  sysRoot,
	}
}

func (c *cpuCollector) reset() {
	c.prev = cpuStatSample{}
	c.prevValid = false
}

func (c *cpuCollector) collect() (protocol.CPUMetrics, error) {
	sample, err := c.readStat()
	if err != nil {
		return protocol.CPUMetrics{}, err
	}

	var usage float64
	if c.prevValid && sample.total > c.prev.total {
		totalDelta := sample.total - c.prev.total
		var idleDelta uint64
		if sample.idle >= c.prev.idle {
			idleDelta = sample.idle - c.prev.idle
		}
		if totalDelta > 0 && idleDelta <= totalDelta {
			usage = float64(totalDelta-idleDelta) / float64(totalDelta) * 100.0
		}
	}
	c.prev = sample
	c.prevValid = true

	return protocol.CPUMetrics{
		UsagePercent: round2(usage),
		FreqMHz:      c.readFreqMHz(),
		TempC:        round1(c.readTempC()),
	}, nil
}

func (c *cpuCollector) readStat() (cpuStatSample, error) {
	f, err := os.Open(filepath.Join(c.procRoot, "stat"))
	if err != nil {
		return cpuStatSample{}, fmt.Errorf("open proc stat: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return cpuStatSample{}, fmt.Errorf("unexpected cpu line in /proc/stat: %q", line)
		}
		var total, idle uint64
		for i, raw := range fields[1:] {
			val, err := strconv.ParseUint(raw, 10, 64)
			if err != nil {
				return cpuStatSample{}, fmt.Errorf("parse cpu stat field %d: %w", i, err)
			}
			total += val
			// fields[1:] index 3 is idle, index 4 is iowait.
			if i == 3 || i == 4 {
				idle += val
			}
		}
		return cpuStatSample{total: total, idle: idle}, nil
	}
	if err := scanner.Err(); err != nil {
		return cpuStatSample{}, err
	}
	return cpuStatSample{}, fmt.Errorf("aggregate cpu line not found in /proc/stat")
}

func (c *cpuCollector) readFreqMHz() int {
	matches, err := filepath.Glob(filepath.Join(c.sysRoot, "devices/system/cpu/cpu[0-9]*/cpufreq/scaling_cur_freq"))
	if err == nil && len(matches) > 0 {
		var sumKHz uint64
		var count uint64
		for _, p := range matches {
			if khz, err := readUint64File(p); err == nil && khz > 0 {
				sumKHz += khz
				count++
			}
		}
		if count > 0 {
			return int(sumKHz / count / 1000)
		}
	}

	// Fallback to /proc/cpuinfo
	f, err := os.Open(filepath.Join(c.procRoot, "cpuinfo"))
	if err != nil {
		return 0
	}
	defer f.Close()

	var sumMHz float64
	var count float64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu MHz") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if mhz, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil && mhz > 0 {
			sumMHz += mhz
			count++
		}
	}
	if count > 0 {
		return int(sumMHz / count)
	}
	return 0
}

func (c *cpuCollector) readTempC() float64 {
	// Prefer named CPU hwmon sensors first.
	preferredHwmon := map[string]bool{
		"coretemp":    true,
		"k10temp":     true,
		"zenpower":    true,
		"cpu_thermal": true,
		"acpitz":      true,
	}
	hwmonDirs, _ := filepath.Glob(filepath.Join(c.sysRoot, "class/hwmon/hwmon*"))
	for _, dir := range hwmonDirs {
		name, err := readTrimmed(filepath.Join(dir, "name"))
		if err != nil || !preferredHwmon[name] {
			continue
		}
		inputs, _ := filepath.Glob(filepath.Join(dir, "temp*_input"))
		for _, in := range inputs {
			if milli, err := readFloat64File(in); err == nil && milli > 0 {
				return milli / 1000.0
			}
		}
	}

	// Fallback to /sys/class/thermal/thermal_zone*
	zones, _ := filepath.Glob(filepath.Join(c.sysRoot, "class/thermal/thermal_zone*"))
	preferredZones := map[string]bool{
		"x86_pkg_temp": true,
		"cpu-thermal":  true,
		"acpitz":       true,
	}
	var fallbackTemp float64
	for _, z := range zones {
		tVal, err := readFloat64File(filepath.Join(z, "temp"))
		if err != nil || tVal <= 0 {
			continue
		}
		tempC := tVal / 1000.0
		zType, _ := readTrimmed(filepath.Join(z, "type"))
		if preferredZones[zType] {
			return tempC
		}
		if fallbackTemp == 0 {
			fallbackTemp = tempC
		}
	}
	return fallbackTemp
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10.0
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100.0
}
