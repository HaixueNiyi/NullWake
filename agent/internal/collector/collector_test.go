package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFakeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCPU_RAM_Net_Collectors(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	sys := filepath.Join(root, "sys")

	writeFakeFile(t, filepath.Join(proc, "stat"), "cpu  100 0 100 800 0 0 0 0 0 0\ncpu0 100 0 100 800 0 0 0 0 0 0\n")
	writeFakeFile(t, filepath.Join(sys, "devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"), "3600000\n")
	writeFakeFile(t, filepath.Join(sys, "devices/system/cpu/cpu1/cpufreq/scaling_cur_freq"), "3600000\n")
	writeFakeFile(t, filepath.Join(sys, "class/hwmon/hwmon0/name"), "coretemp\n")
	writeFakeFile(t, filepath.Join(sys, "class/hwmon/hwmon0/temp1_input"), "52500\n")

	writeFakeFile(t, filepath.Join(proc, "meminfo"), ""+
		"MemTotal:       16384000 kB\n"+
		"MemFree:         4096000 kB\n"+
		"MemAvailable:    8192000 kB\n"+
		"SwapTotal:       2048000 kB\n"+
		"SwapFree:        1024000 kB\n")

	writeFakeFile(t, filepath.Join(proc, "net/dev"), ""+
		"Inter-|   Receive                                                |  Transmit\n"+
		" face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed\n"+
		"    lo: 9999999 100 0 0 0 0 0 0 9999999 100 0 0 0 0 0 0\n"+
		"  tun0: 8888888 100 0 0 0 0 0 0 8888888 100 0 0 0 0 0 0\n"+
		"  eth0:  100000  10 0 0 0 0 0 0   50000  10 0 0 0 0 0 0\n")

	now := time.Unix(1730000000, 0)
	c := New(WithRoots(proc, sys))
	c.net.nowFn = func() time.Time { return now }

	snap1, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("first collect failed: %v", err)
	}
	if snap1.CPU.UsagePercent != 0 {
		t.Fatalf("expected 0 initial CPU delta, got %v", snap1.CPU.UsagePercent)
	}
	if snap1.CPU.FreqMHz != 3600 || snap1.CPU.TempC != 52.5 {
		t.Fatalf("unexpected CPU freq/temp: %+v", snap1.CPU)
	}
	if snap1.RAM.TotalMB != 16000 || snap1.RAM.UsedMB != 8000 || snap1.RAM.SwapUsedMB != 1000 {
		t.Fatalf("unexpected RAM metrics: %+v", snap1.RAM)
	}
	if snap1.GPU != nil {
		t.Fatalf("expected nil GPU on headless mock, got %+v", snap1.GPU)
	}

	// Advance stats by 2 seconds: +25 busy +25 idle = +50 total (50% CPU)
	now = now.Add(2 * time.Second)
	writeFakeFile(t, filepath.Join(proc, "stat"), "cpu  125 0 100 825 0 0 0 0 0 0\n")
	writeFakeFile(t, filepath.Join(proc, "net/dev"), ""+
		"Inter-|   Receive\n face |bytes\n"+
		"    lo: 9999999 100 0 0 0 0 0 0 9999999 100 0 0 0 0 0 0\n"+
		"  tun0: 9999999 100 0 0 0 0 0 0 9999999 100 0 0 0 0 0 0\n"+
		"  eth0:  200000  20 0 0 0 0 0 0  110000  20 0 0 0 0 0 0\n")

	snap2, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("second collect failed: %v", err)
	}
	if snap2.CPU.UsagePercent != 50.0 {
		t.Fatalf("expected 50.0 CPU usage, got %v", snap2.CPU.UsagePercent)
	}
	if snap2.Net.RxBytesSec != 50000 || snap2.Net.TxBytesSec != 30000 {
		t.Fatalf("unexpected net rates: %+v", snap2.Net)
	}

	// Reset clears baseline.
	c.Reset()
	snap3, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("third collect failed: %v", err)
	}
	if snap3.CPU.UsagePercent != 0 || snap3.Net.RxBytesSec != 0 {
		t.Fatalf("expected zeroed deltas after Reset(), got CPU=%v Net=%+v", snap3.CPU.UsagePercent, snap3.Net)
	}
}

func TestAMDGPUDetection(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	sys := filepath.Join(root, "sys")

	writeFakeFile(t, filepath.Join(proc, "stat"), "cpu 10 0 10 80 0 0 0 0 0 0\n")
	writeFakeFile(t, filepath.Join(proc, "meminfo"), "MemTotal: 1024000 kB\nMemAvailable: 512000 kB\n")
	writeFakeFile(t, filepath.Join(proc, "net/dev"), "Inter-|\n face |\n")

	// Connector directory should be ignored.
	writeFakeFile(t, filepath.Join(sys, "class/drm/card0-HDMI-A-1/status"), "connected\n")
	writeFakeFile(t, filepath.Join(sys, "class/drm/card0/device/vendor"), "0x1002\n")
	writeFakeFile(t, filepath.Join(sys, "class/drm/card0/device/product_name"), "AMD Radeon RX 6600\n")
	writeFakeFile(t, filepath.Join(sys, "class/drm/card0/device/gpu_busy_percent"), "68\n")
	writeFakeFile(t, filepath.Join(sys, "class/drm/card0/device/mem_info_vram_used"), "2147483648\n")
	writeFakeFile(t, filepath.Join(sys, "class/drm/card0/device/mem_info_vram_total"), "8589934592\n")
	writeFakeFile(t, filepath.Join(sys, "class/drm/card0/device/hwmon/hwmon1/temp1_input"), "63400\n")

	c := New(WithRoots(proc, sys))
	snap, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if snap.GPU == nil {
		t.Fatalf("expected AMD GPU to be detected")
	}
	if snap.GPU.Vendor != "amd" || snap.GPU.Name != "AMD Radeon RX 6600" {
		t.Fatalf("unexpected GPU identity: %+v", snap.GPU)
	}
	if snap.GPU.UsagePercent != 68 || snap.GPU.VRAMUsedMB != 2048 || snap.GPU.VRAMTotalMB != 8192 || snap.GPU.TempC != 63.4 {
		t.Fatalf("unexpected AMD GPU metrics: %+v", snap.GPU)
	}
}

func TestIntelGPUDetection(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	sys := filepath.Join(root, "sys")

	writeFakeFile(t, filepath.Join(proc, "stat"), "cpu 10 0 10 80 0 0 0 0 0 0\n")
	writeFakeFile(t, filepath.Join(proc, "meminfo"), "MemTotal: 1024000 kB\nMemAvailable: 512000 kB\n")
	writeFakeFile(t, filepath.Join(proc, "net/dev"), "Inter-|\n face |\n")

	writeFakeFile(t, filepath.Join(sys, "class/drm/card0/device/vendor"), "0x8086\n")
	writeFakeFile(t, filepath.Join(sys, "class/drm/card0/device/label"), "Intel HD Graphics 630\n")
	writeFakeFile(t, filepath.Join(sys, "class/drm/card0/gt_cur_freq_mhz"), "600\n")
	writeFakeFile(t, filepath.Join(sys, "class/drm/card0/gt_max_freq_mhz"), "1200\n")
	writeFakeFile(t, filepath.Join(sys, "class/thermal/thermal_zone0/type"), "x86_pkg_temp\n")
	writeFakeFile(t, filepath.Join(sys, "class/thermal/thermal_zone0/temp"), "49000\n")

	c := New(WithRoots(proc, sys))
	snap, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if snap.GPU == nil {
		t.Fatalf("expected Intel GPU to be detected")
	}
	if snap.GPU.Vendor != "intel" || snap.GPU.Name != "Intel HD Graphics 630" {
		t.Fatalf("unexpected Intel GPU identity: %+v", snap.GPU)
	}
	if snap.GPU.UsagePercent != 50.0 || snap.GPU.TempC != 49.0 {
		t.Fatalf("unexpected Intel GPU metrics: %+v", snap.GPU)
	}
}

func TestParseIntelGPUTopJSON(t *testing.T) {
	sample := `[
		{"period": {"duration": 100.0}, "engines": {"Render/3D": {"busy": 42.5}, "Video": {"busy": 12.0}}}
	]`
	val, ok := parseIntelGPUTopJSON([]byte(sample))
	if !ok || val != 42.5 {
		t.Fatalf("got (%v, %v), want (42.5, true)", val, ok)
	}
}

func TestIntelRC6ResidencyAndCoretempPriority(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	sys := filepath.Join(root, "sys")

	writeFakeFile(t, filepath.Join(proc, "stat"), "cpu 10 0 10 80 0 0 0 0 0 0\n")
	writeFakeFile(t, filepath.Join(proc, "meminfo"), "MemTotal: 1024000 kB\nMemAvailable: 512000 kB\n")
	writeFakeFile(t, filepath.Join(proc, "net/dev"), "Inter-|\n face |\n")

	// hwmon1 is acpitz, hwmon5 is coretemp -> coretemp must win regardless of glob order.
	writeFakeFile(t, filepath.Join(sys, "class/hwmon/hwmon1/name"), "acpitz\n")
	writeFakeFile(t, filepath.Join(sys, "class/hwmon/hwmon1/temp1_input"), "29000\n")
	writeFakeFile(t, filepath.Join(sys, "class/hwmon/hwmon5/name"), "coretemp\n")
	writeFakeFile(t, filepath.Join(sys, "class/hwmon/hwmon5/temp1_input"), "41500\n")

	// Intel i915 card with high requested gt_cur_freq_mhz (883) but 100% RC6 idle residency.
	writeFakeFile(t, filepath.Join(sys, "class/drm/card1/device/vendor"), "0x8086\n")
	writeFakeFile(t, filepath.Join(sys, "class/drm/card1/gt_cur_freq_mhz"), "883\n")
	writeFakeFile(t, filepath.Join(sys, "class/drm/card1/gt_max_freq_mhz"), "1000\n")
	writeFakeFile(t, filepath.Join(sys, "class/drm/card1/gt/gt0/rc6_residency_ms"), "10000\n")

	now := time.Unix(1730000000, 0)
	c := New(WithRoots(proc, sys))
	c.gpu.rc6NowFn = func() time.Time { return now }

	snap1, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("first collect failed: %v", err)
	}
	if snap1.CPU.TempC != 41.5 {
		t.Fatalf("expected coretemp 41.5°C over acpitz 29.0°C, got %v", snap1.CPU.TempC)
	}
	if snap1.GPU == nil || snap1.GPU.UsagePercent != 0 {
		t.Fatalf("expected 0 initial GPU usage, got %+v", snap1.GPU)
	}

	// Simulate 1000ms elapsed with 750ms in RC6 idle -> 25% active GPU busy.
	now = now.Add(1000 * time.Millisecond)
	writeFakeFile(t, filepath.Join(sys, "class/drm/card1/gt/gt0/rc6_residency_ms"), "10750\n")

	snap2, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("second collect failed: %v", err)
	}
	if snap2.GPU.UsagePercent != 25.0 {
		t.Fatalf("expected 25.0%% GPU busy from RC6 delta, got %v", snap2.GPU.UsagePercent)
	}
}
