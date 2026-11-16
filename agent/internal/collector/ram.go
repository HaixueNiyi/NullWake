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

type ramCollector struct {
	procRoot string
}

func newRAMCollector(procRoot string) *ramCollector {
	return &ramCollector{procRoot: procRoot}
}

func (r *ramCollector) collect() (protocol.RAMMetrics, error) {
	f, err := os.Open(filepath.Join(r.procRoot, "meminfo"))
	if err != nil {
		return protocol.RAMMetrics{}, fmt.Errorf("open meminfo: %w", err)
	}
	defer f.Close()

	values := make(map[string]uint64, 8)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}
		key := line[:colon]
		fields := strings.Fields(line[colon+1:])
		if len(fields) == 0 {
			continue
		}
		val, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		values[key] = val
	}
	if err := scanner.Err(); err != nil {
		return protocol.RAMMetrics{}, err
	}

	memTotal := values["MemTotal"]
	memAvail, hasAvail := values["MemAvailable"]
	if !hasAvail {
		memAvail = values["MemFree"] + values["Buffers"] + values["Cached"]
	}

	var usedKB uint64
	if memTotal > memAvail {
		usedKB = memTotal - memAvail
	}

	swapTotal := values["SwapTotal"]
	swapFree := values["SwapFree"]
	var swapUsedKB uint64
	if swapTotal > swapFree {
		swapUsedKB = swapTotal - swapFree
	}

	return protocol.RAMMetrics{
		UsedMB:     usedKB / 1024,
		TotalMB:    memTotal / 1024,
		SwapUsedMB: swapUsedKB / 1024,
	}, nil
}
