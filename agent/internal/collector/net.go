package collector

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nullwake/nullwake/agent/internal/protocol"
)

type netCollector struct {
	procRoot  string
	prevRx    uint64
	prevTx    uint64
	prevTime  time.Time
	prevValid bool
	nowFn     func() time.Time
}

func newNetCollector(procRoot string) *netCollector {
	return &netCollector{
		procRoot: procRoot,
		nowFn:    time.Now,
	}
}

func (n *netCollector) reset() {
	n.prevRx = 0
	n.prevTx = 0
	n.prevTime = time.Time{}
	n.prevValid = false
}

func (n *netCollector) collect() (protocol.NetMetrics, error) {
	rx, tx, err := n.readCounters()
	if err != nil {
		return protocol.NetMetrics{}, err
	}
	now := n.nowFn()

	var rxRate, txRate uint64
	if n.prevValid {
		elapsed := now.Sub(n.prevTime).Seconds()
		if elapsed > 0 {
			if rx >= n.prevRx {
				rxRate = uint64(float64(rx-n.prevRx) / elapsed)
			}
			if tx >= n.prevTx {
				txRate = uint64(float64(tx-n.prevTx) / elapsed)
			}
		}
	}

	n.prevRx = rx
	n.prevTx = tx
	n.prevTime = now
	n.prevValid = true

	return protocol.NetMetrics{
		RxBytesSec: rxRate,
		TxBytesSec: txRate,
	}, nil
}

func (n *netCollector) readCounters() (rxTotal uint64, txTotal uint64, err error) {
	f, err := os.Open(filepath.Join(n.procRoot, "net/dev"))
	if err != nil {
		return 0, 0, fmt.Errorf("open proc net/dev: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum <= 2 {
			continue
		}
		line := scanner.Text()
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}
		iface := strings.TrimSpace(line[:colon])
		if isVirtualInterface(iface) {
			continue
		}
		fields := strings.Fields(line[colon+1:])
		if len(fields) < 9 {
			continue
		}
		rx, err1 := strconv.ParseUint(fields[0], 10, 64)
		tx, err2 := strconv.ParseUint(fields[8], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		rxTotal += rx
		txTotal += tx
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}
	return rxTotal, txTotal, nil
}

func isVirtualInterface(iface string) bool {
	if iface == "" || iface == "lo" {
		return true
	}
	virtualPrefixes := []string{
		"tun", "tap", "veth", "docker", "br-", "virbr",
		"tailscale", "wg", "ifb", "cni", "flannel", "cali", "kube-",
	}
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(iface, prefix) {
			return true
		}
	}
	return false
}
