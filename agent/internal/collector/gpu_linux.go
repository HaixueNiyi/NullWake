//go:build linux

package collector

import (
	"encoding/binary"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

func (g *gpuCollector) readIntelPMUUsage() (float64, bool) {
	if g.pmuFailed {
		return 0, false
	}

	if g.pmuFD < 0 {
		typeStr, err := readTrimmed(filepath.Join(g.sysRoot, "bus/event_source/devices/i915/type"))
		if err != nil {
			g.pmuFailed = true
			return 0, false
		}
		pmuType, err := strconv.ParseUint(typeStr, 10, 32)
		if err != nil {
			g.pmuFailed = true
			return 0, false
		}

		cfgStr, err := readTrimmed(filepath.Join(g.sysRoot, "bus/event_source/devices/i915/events/rcs0-busy"))
		if err != nil {
			g.pmuFailed = true
			return 0, false
		}
		eventCfg, ok := parseEventConfig(cfgStr)
		if !ok {
			g.pmuFailed = true
			return 0, false
		}

		attr := &unix.PerfEventAttr{
			Type:   uint32(pmuType),
			Size:   uint32(unix.PERF_ATTR_SIZE_VER0),
			Config: eventCfg,
		}
		// System-wide GPU PMU requires pid=-1 and cpu=0 (with CAP_PERFMON or root).
		fd, err := unix.PerfEventOpen(attr, -1, 0, -1, unix.PERF_FLAG_FD_CLOEXEC)
		if err != nil {
			g.pmuFailed = true
			return 0, false
		}
		g.pmuFD = fd
	}

	var buf [8]byte
	n, err := unix.Read(g.pmuFD, buf[:])
	if err != nil || n != 8 {
		g.closePMU()
		g.pmuFailed = true
		return 0, false
	}

	busyNs := binary.LittleEndian.Uint64(buf[:])
	now := time.Now()

	var usage float64
	if g.hasPrevSample {
		wallNs := now.Sub(g.prevSampleAt).Nanoseconds()
		if wallNs > 0 && busyNs >= g.prevBusyNs {
			usage = float64(busyNs-g.prevBusyNs) / float64(wallNs) * 100.0
		}
	}
	g.prevBusyNs = busyNs
	g.prevSampleAt = now
	g.hasPrevSample = true
	return clampPercent(usage), true
}

func (g *gpuCollector) closePMU() {
	if g.pmuFD >= 0 {
		_ = unix.Close(g.pmuFD)
		g.pmuFD = -1
	}
}
