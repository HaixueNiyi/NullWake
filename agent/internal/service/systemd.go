package service

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/nullwake/nullwake/agent/internal/protocol"
)

// UnitState represents the resolved status and memory usage of a systemd unit.
type UnitState struct {
	Status      string
	ActiveState string
	SubState    string
	Description string
	RSSBytes    uint64
	HasRSS      bool
}

// SystemdQuerier retrieves systemd unit states without spawning external processes.
type SystemdQuerier interface {
	QueryUnit(ctx context.Context, unit string) (UnitState, error)
	Close() error
}

// DBusSystemdQuerier communicates with systemd over the system DBus socket on demand.
type DBusSystemdQuerier struct {
	mu   sync.Mutex
	conn *dbus.Conn
}

// NewDBusSystemdQuerier creates a lazy DBus querier (no connection is opened until QueryUnit is called).
func NewDBusSystemdQuerier() *DBusSystemdQuerier {
	return &DBusSystemdQuerier{}
}

func (q *DBusSystemdQuerier) getConn() (*dbus.Conn, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.conn != nil && q.conn.Connected() {
		return q.conn, nil
	}
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect system dbus: %w", err)
	}
	q.conn = conn
	return q.conn, nil
}

// QueryUnit queries org.freedesktop.systemd1 for ActiveState, SubState, Description, and MemoryCurrent/VmRSS.
func (q *DBusSystemdQuerier) QueryUnit(ctx context.Context, unit string) (UnitState, error) {
	conn, err := q.getConn()
	if err != nil {
		return UnitState{}, err
	}

	mgr := conn.Object("org.freedesktop.systemd1", dbus.ObjectPath("/org/freedesktop/systemd1"))
	var unitPath dbus.ObjectPath
	if err := mgr.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.GetUnit", 0, unit).Store(&unitPath); err != nil {
		if loadErr := mgr.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.LoadUnit", 0, unit).Store(&unitPath); loadErr != nil {
			return UnitState{}, fmt.Errorf("systemd GetUnit/LoadUnit %q: %w", unit, err)
		}
	}

	unitObj := conn.Object("org.freedesktop.systemd1", unitPath)
	activeState, err := readUnitPropString(ctx, unitObj, "org.freedesktop.systemd1.Unit", "ActiveState")
	if err != nil {
		return UnitState{}, err
	}
	subState, _ := readUnitPropString(ctx, unitObj, "org.freedesktop.systemd1.Unit", "SubState")
	desc, _ := readUnitPropString(ctx, unitObj, "org.freedesktop.systemd1.Unit", "Description")

	var rssBytes uint64
	var hasRSS bool
	if memCur, err := readUnitPropUint64(ctx, unitObj, "org.freedesktop.systemd1.Service", "MemoryCurrent"); err == nil && memCur > 0 && memCur < math.MaxUint64 {
		rssBytes = memCur
		hasRSS = true
	} else if mainPID, err := readUnitPropUint32(ctx, unitObj, "org.freedesktop.systemd1.Service", "MainPID"); err == nil && mainPID > 0 {
		if pidRSS, ok := readProcVmRSS(int(mainPID)); ok {
			rssBytes = pidRSS
			hasRSS = true
		}
	}

	return UnitState{
		Status:      mapSystemdState(activeState),
		ActiveState: activeState,
		SubState:    subState,
		Description: desc,
		RSSBytes:    rssBytes,
		HasRSS:      hasRSS,
	}, nil
}

// Close closes the cached DBus connection if open.
func (q *DBusSystemdQuerier) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.conn != nil {
		err := q.conn.Close()
		q.conn = nil
		return err
	}
	return nil
}

func readUnitPropString(ctx context.Context, obj dbus.BusObject, iface, prop string) (string, error) {
	var variant dbus.Variant
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, iface, prop).Store(&variant); err != nil {
		return "", fmt.Errorf("read unit property %s: %w", prop, err)
	}
	s, ok := variant.Value().(string)
	if !ok {
		return "", fmt.Errorf("unexpected property %s type %T", prop, variant.Value())
	}
	return s, nil
}

func readUnitPropUint64(ctx context.Context, obj dbus.BusObject, iface, prop string) (uint64, error) {
	var variant dbus.Variant
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, iface, prop).Store(&variant); err != nil {
		return 0, err
	}
	v, ok := variant.Value().(uint64)
	if !ok {
		return 0, fmt.Errorf("unexpected property %s type %T", prop, variant.Value())
	}
	return v, nil
}

func readUnitPropUint32(ctx context.Context, obj dbus.BusObject, iface, prop string) (uint32, error) {
	var variant dbus.Variant
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, iface, prop).Store(&variant); err != nil {
		return 0, err
	}
	v, ok := variant.Value().(uint32)
	if !ok {
		return 0, fmt.Errorf("unexpected property %s type %T", prop, variant.Value())
	}
	return v, nil
}

func readProcVmRSS(pid int) (uint64, bool) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "VmRSS:"))
		if len(fields) == 0 {
			return 0, false
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}

func mapSystemdState(activeState string) string {
	switch activeState {
	case "active":
		return protocol.ServiceRunning
	case "failed", "activating", "deactivating", "reloading":
		return protocol.ServiceDegraded
	default:
		return protocol.ServiceStopped
	}
}
