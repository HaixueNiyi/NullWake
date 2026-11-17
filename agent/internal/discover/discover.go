package discover

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/godbus/dbus/v5"
	"github.com/nullwake/nullwake/agent/internal/config"
)

// Candidate represents a discovered systemd service or listening TCP endpoint.
type Candidate struct {
	Kind        string // "systemd" | "tcp"
	ID          string
	Name        string
	Unit        string
	Description string
	Addr        string
	Port        uint16
	ProcessName string
}

type dbusUnitInfo struct {
	Name        string
	Description string
	LoadState   string
	ActiveState string
	SubState    string
	Following   string
	Path        dbus.ObjectPath
	JobID       uint32
	JobType     string
	JobPath     dbus.ObjectPath
}

// Options configures the interactive service discovery command.
type Options struct {
	ConfigPath string
	ProcRoot   string
	In         io.Reader
	Out        io.Writer
	ScanUnits  func(context.Context) ([]Candidate, error)
}

var noisySystemdExact = map[string]bool{
	"acpid.service":                  true,
	"accounts-daemon.service":        true,
	"alsa-restore.service":           true,
	"alsa-state.service":             true,
	"apparmor.service":               true,
	"atd.service":                    true,
	"auditd.service":                 true,
	"avahi-daemon.service":           true,
	"blk-availability.service":       true,
	"bluetooth.service":              true,
	"chrony.service":                 true,
	"chronyd.service":                true,
	"colord.service":                 true,
	"console-kit-daemon.service":     true,
	"cron.service":                   true,
	"crond.service":                  true,
	"cups.service":                   true,
	"dbus-broker.service":            true,
	"dbus.service":                   true,
	"dmesg.service":                  true,
	"e2scrub_reap.service":           true,
	"firewalld.service":              true,
	"fwupd.service":                  true,
	"gdm.service":                    true,
	"haveged.service":                true,
	"irqbalance.service":             true,
	"kmod-static-nodes.service":      true,
	"lightdm.service":                true,
	"lm-sensors.service":             true,
	"networkd-dispatcher.service":    true,
	"NetworkManager.service":         true,
	"packagekit.service":             true,
	"plymouth-quit-wait.service":     true,
	"plymouth-quit.service":          true,
	"polkit.service":                 true,
	"power-profiles-daemon.service":  true,
	"rngd.service":                   true,
	"rsyslog.service":                true,
	"rtkit-daemon.service":           true,
	"sddm.service":                   true,
	"smartd.service":                 true,
	"smartmontools.service":          true,
	"snapd.service":                  true,
	"ssh.service":                    true,
	"sshd.service":                   true,
	"switcheroo-control.service":     true,
	"thermald.service":               true,
	"udev.service":                   true,
	"udisks2.service":                true,
	"ufw.service":                    true,
	"unattended-upgrades.service":    true,
	"upower.service":                 true,
	"wpa_supplicant.service":         true,
}

var noisySystemdPrefixes = []string{
	"systemd-",
	"dbus-",
	"getty@",
	"serial-getty@",
	"user@",
	"user-runtime-dir@",
	"modprobe@",
	"NetworkManager-",
	"lvm2-",
	"dm-",
	"plymouth-",
	"console-",
	"dracut-",
}

// IsNoisySystemdUnit returns true for low-level OS plumbing units that should be hidden by default.
func IsNoisySystemdUnit(unitName string) bool {
	if noisySystemdExact[unitName] {
		return true
	}
	for _, prefix := range noisySystemdPrefixes {
		if strings.HasPrefix(unitName, prefix) {
			return true
		}
	}
	return false
}

// Run executes the interactive `nullwake-agent discover` flow.
func Run(ctx context.Context, opts Options) error {
	if opts.ConfigPath == "" {
		opts.ConfigPath = "/etc/nullwake/config.toml"
	}
	if opts.ProcRoot == "" {
		opts.ProcRoot = "/proc"
	}
	if opts.In == nil {
		opts.In = os.Stdin
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	scanUnits := opts.ScanUnits
	if scanUnits == nil {
		scanUnits = ScanSystemdUnits
	}

	candidates, warnings := DiscoverAll(ctx, opts.ProcRoot, scanUnits)
	for _, w := range warnings {
		fmt.Fprintf(opts.Out, "note: %s\n", w)
	}

	if len(candidates) == 0 {
		fmt.Fprintln(opts.Out, "No discoverable systemd services or listening TCP ports found.")
		return nil
	}

	existingIDs := loadExistingServiceIDs(opts.ConfigPath)
	AssignUniqueIDs(candidates, existingIDs)

	fmt.Fprintln(opts.Out, "Discovered services and listening endpoints:")
	for i, c := range candidates {
		switch c.Kind {
		case "systemd":
			desc := c.Description
			if desc == "" {
				desc = c.Unit
			}
			fmt.Fprintf(opts.Out, "  [%d] [systemd] %-24s (%s) -> id=%s\n", i+1, c.Unit, desc, c.ID)
		case "tcp":
			proc := c.ProcessName
			if proc == "" {
				proc = "unknown"
			}
			fmt.Fprintf(opts.Out, "  [%d] [tcp]     %-24s (process: %s) -> id=%s\n", i+1, c.Addr, proc, c.ID)
		}
	}

	reader := bufio.NewReader(opts.In)
	fmt.Fprint(opts.Out, "\nSelect comma-separated numbers to add (e.g. 1,3) or 'q' to quit: ")
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return nil
	}
	line = strings.TrimSpace(line)
	if line == "" || strings.EqualFold(line, "q") || strings.EqualFold(line, "quit") {
		fmt.Fprintln(opts.Out, "Cancelled; no changes written.")
		return nil
	}

	selected, err := parseSelectionIndices(line, len(candidates))
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		fmt.Fprintln(opts.Out, "No services selected.")
		return nil
	}

	var picked []Candidate
	for _, idx := range selected {
		picked = append(picked, candidates[idx])
	}

	tomlSnippet := RenderTOMLBlocks(picked)
	fmt.Fprintf(opts.Out, "\nGenerated TOML preview (%s):\n%s\n", opts.ConfigPath, tomlSnippet)

	fmt.Fprintf(opts.Out, "Append %d [[services]] block(s) to %s? [Y/n]: ", len(picked), opts.ConfigPath)
	confirm, err := reader.ReadString('\n')
	if err == nil || len(confirm) > 0 {
		ans := strings.ToLower(strings.TrimSpace(confirm))
		if ans == "n" || ans == "no" {
			fmt.Fprintln(opts.Out, "Aborted; configuration untouched.")
			return nil
		}
	}

	if err := AppendTOMLFile(opts.ConfigPath, tomlSnippet); err != nil {
		return fmt.Errorf("write %s: %w", opts.ConfigPath, err)
	}
	fmt.Fprintf(opts.Out, "Added %d service(s) to %s.\n", len(picked), opts.ConfigPath)
	return nil
}

// DiscoverAll runs systemd DBus and /proc/net TCP discovery and returns combined candidates.
func DiscoverAll(ctx context.Context, procRoot string, scanUnits func(context.Context) ([]Candidate, error)) ([]Candidate, []string) {
	var all []Candidate
	var warnings []string

	if scanUnits != nil {
		units, err := scanUnits(ctx)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("systemd DBus scan skipped: %v", err))
		} else {
			all = append(all, units...)
		}
	}

	tcpCandidates, err := ScanListeningTCP(procRoot)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("TCP socket scan skipped: %v", err))
	} else {
		all = append(all, tcpCandidates...)
	}

	return all, warnings
}

// ScanSystemdUnits queries org.freedesktop.systemd1.Manager.ListUnits on the system DBus.
func ScanSystemdUnits(ctx context.Context) ([]Candidate, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.systemd1", dbus.ObjectPath("/org/freedesktop/systemd1"))
	var units []dbusUnitInfo
	call := obj.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.ListUnits", 0)
	if err := call.Store(&units); err != nil {
		return nil, err
	}

	var out []Candidate
	for _, u := range units {
		if u.LoadState != "loaded" || u.ActiveState != "active" {
			continue
		}
		if !strings.HasSuffix(u.Name, ".service") || IsNoisySystemdUnit(u.Name) {
			continue
		}
		base := strings.TrimSuffix(u.Name, ".service")
		name := titleFromSlug(base)
		if name == "" {
			name = u.Name
		}
		out = append(out, Candidate{
			Kind:        "systemd",
			ID:          sanitizeSlug(base),
			Name:        name,
			Unit:        u.Name,
			Description: u.Description,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Unit < out[j].Unit
	})
	return out, nil
}

// ScanListeningTCP inspects <procRoot>/net/tcp and <procRoot>/net/tcp6 for TCP_LISTEN sockets.
func ScanListeningTCP(procRoot string) ([]Candidate, error) {
	inodeToProc := mapSocketInodesToProcesses(procRoot)

	var raw []Candidate
	for _, spec := range []struct {
		file string
		isV6 bool
	}{
		{file: filepath.Join(procRoot, "net", "tcp"), isV6: false},
		{file: filepath.Join(procRoot, "net", "tcp6"), isV6: true},
	} {
		data, err := os.ReadFile(spec.file)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		parsed := ParseProcNetTCP(string(data), spec.isV6, inodeToProc)
		raw = append(raw, parsed...)
	}

	// Deduplicate entries representing the same process+port across v4/v6 or duplicate listeners.
	seenKey := make(map[string]bool)
	var out []Candidate
	for _, c := range raw {
		key := fmt.Sprintf("%d|%s|%s", c.Port, c.ProcessName, c.Addr)
		if c.ProcessName != "" {
			// Collapse IPv4 and IPv6 wildcard bindings for the same process and port.
			if strings.HasPrefix(c.Addr, "127.0.0.1:") {
				key = fmt.Sprintf("%d|%s", c.Port, c.ProcessName)
			}
		}
		if seenKey[key] {
			continue
		}
		seenKey[key] = true
		out = append(out, c)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].Addr < out[j].Addr
	})
	return out, nil
}

// ParseProcNetTCP extracts TCP_LISTEN (0A) sockets from /proc/net/tcp or /proc/net/tcp6 content.
func ParseProcNetTCP(content string, isV6 bool, inodeToProc map[string]string) []Candidate {
	var out []Candidate
	lines := strings.Split(content, "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		// Column 3 is connection state; 0A == TCP_LISTEN.
		if !strings.EqualFold(fields[3], "0A") {
			continue
		}
		addr, port, ok := parseProcHexAddr(fields[1], isV6)
		if !ok || port == 0 {
			continue
		}
		procName := inodeToProc[fields[9]]
		slugSource := procName
		if slugSource == "" {
			slugSource = fmt.Sprintf("tcp-%d", port)
		}
		displayName := fmt.Sprintf("TCP :%d", port)
		if procName != "" {
			displayName = fmt.Sprintf("%s (:%d)", procName, port)
		}
		out = append(out, Candidate{
			Kind:        "tcp",
			ID:          sanitizeSlug(slugSource),
			Name:        displayName,
			Addr:        addr,
			Port:        port,
			ProcessName: procName,
		})
	}
	return out
}

func parseProcHexAddr(raw string, isV6 bool) (string, uint16, bool) {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return "", 0, false
	}
	portVal, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return "", 0, false
	}
	port := uint16(portVal)

	hexIP := parts[0]
	if !isV6 {
		if len(hexIP) != 8 {
			return "", 0, false
		}
		word, err := strconv.ParseUint(hexIP, 16, 32)
		if err != nil {
			return "", 0, false
		}
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], uint32(word))
		ip := net.IP(b[:])
		if ip.IsUnspecified() {
			ip = net.IPv4(127, 0, 0, 1)
		}
		return net.JoinHostPort(ip.String(), strconv.Itoa(int(port))), port, true
	}

	if len(hexIP) != 32 {
		return "", 0, false
	}
	rawBytes, err := hex.DecodeString(hexIP)
	if err != nil || len(rawBytes) != 16 {
		return "", 0, false
	}
	// /proc/net/tcp6 emits four 32-bit host-order words.
	var ipBytes [16]byte
	for i := 0; i < 4; i++ {
		word := binary.BigEndian.Uint32(rawBytes[i*4 : (i+1)*4])
		binary.LittleEndian.PutUint32(ipBytes[i*4:(i+1)*4], word)
	}
	ip := net.IP(ipBytes[:])
	if ip.IsUnspecified() {
		return net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))), port, true
	}
	if v4 := ip.To4(); v4 != nil {
		return net.JoinHostPort(v4.String(), strconv.Itoa(int(port))), port, true
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(int(port))), port, true
}

func mapSocketInodesToProcesses(procRoot string) map[string]string {
	out := make(map[string]string)
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid := entry.Name()
		if !isAllDigits(pid) {
			continue
		}
		fdDir := filepath.Join(procRoot, pid, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		var comm string
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if strings.HasPrefix(target, "socket:[") && strings.HasSuffix(target, "]") {
				inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
				if comm == "" {
					if raw, err := os.ReadFile(filepath.Join(procRoot, pid, "comm")); err == nil {
						comm = strings.TrimSpace(string(raw))
					} else {
						comm = "pid-" + pid
					}
				}
				if _, exists := out[inode]; !exists {
					out[inode] = comm
				}
			}
		}
	}
	return out
}

func loadExistingServiceIDs(configPath string) map[string]bool {
	ids := make(map[string]bool)
	cfg, err := config.Load(configPath)
	if err != nil {
		return ids
	}
	for _, svc := range cfg.Services {
		ids[svc.ID] = true
	}
	return ids
}

// AssignUniqueIDs guarantees distinct, config-safe IDs across existing and new candidates.
func AssignUniqueIDs(candidates []Candidate, existing map[string]bool) {
	used := make(map[string]bool, len(existing)+len(candidates))
	for k, v := range existing {
		if v {
			used[k] = true
		}
	}
	for i := range candidates {
		base := candidates[i].ID
		if base == "" {
			base = "service"
		}
		candidateID := base
		if used[candidateID] && candidates[i].Kind == "tcp" && candidates[i].Port > 0 {
			candidateID = fmt.Sprintf("%s-%d", base, candidates[i].Port)
		}
		suffix := 2
		for used[candidateID] {
			candidateID = fmt.Sprintf("%s-%d", base, suffix)
			suffix++
		}
		candidates[i].ID = candidateID
		used[candidateID] = true
	}
}

// RenderTOMLBlocks renders [[services]] blocks for the selected candidates.
func RenderTOMLBlocks(picked []Candidate) string {
	var b strings.Builder
	for i, c := range picked {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("[[services]]\n")
		fmt.Fprintf(&b, "id = %s\n", strconv.Quote(c.ID))
		fmt.Fprintf(&b, "name = %s\n", strconv.Quote(c.Name))
		fmt.Fprintf(&b, "type = %s\n", strconv.Quote(c.Kind))
		switch c.Kind {
		case "systemd":
			fmt.Fprintf(&b, "unit = %s\n", strconv.Quote(c.Unit))
		case "tcp":
			fmt.Fprintf(&b, "addr = %s\n", strconv.Quote(c.Addr))
			b.WriteString("timeout_ms = 500\n")
		}
	}
	return b.String()
}

// AppendTOMLFile safely appends generated [[services]] blocks to configPath.
func AppendTOMLFile(configPath, snippet string) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}
	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var prefix string
	if len(existing) > 0 {
		trimmed := strings.TrimRight(string(existing), "\r\n")
		prefix = trimmed + "\n\n"
	}
	out := prefix + strings.TrimSpace(snippet) + "\n"
	return os.WriteFile(configPath, []byte(out), 0o644)
}

func parseSelectionIndices(input string, maxLen int) ([]int, error) {
	parts := strings.Split(input, ",")
	seen := make(map[int]bool)
	var out []int
	for _, raw := range parts {
		token := strings.TrimSpace(raw)
		if token == "" {
			continue
		}
		num, err := strconv.Atoi(token)
		if err != nil || num < 1 || num > maxLen {
			return nil, fmt.Errorf("invalid selection %q (expected 1..%d)", token, maxLen)
		}
		idx := num - 1
		if !seen[idx] {
			seen[idx] = true
			out = append(out, idx)
		}
	}
	return out, nil
}

func sanitizeSlug(raw string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(raw) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func titleFromSlug(slug string) string {
	parts := strings.FieldsFunc(slug, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '@'
	})
	for i, p := range parts {
		runes := []rune(p)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
			parts[i] = string(runes)
		}
	}
	return strings.Join(parts, " ")
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
