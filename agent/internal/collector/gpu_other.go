//go:build !linux

package collector

func (g *gpuCollector) readIntelPMUUsage() (float64, bool) {
	return 0, false
}

func (g *gpuCollector) closePMU() {}
