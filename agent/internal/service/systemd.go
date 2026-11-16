package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/nullwake/nullwake/agent/internal/protocol"
)

// UnitState represents the resolved status of a systemd unit.
type UnitState struct {
	Status      string
	ActiveState string
	SubState    string
	Description string
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

// QueryUnit queries org.freedesktop.systemd1 for ActiveState, SubState, and Description.
func (q *DBusSystemdQuerier) QueryUnit(ctx context.Context, unit string) (UnitState, error) {
	conn, err := q.getConn()
	if err != nil {
		return UnitState{}, err
	}

	mgr := conn.Object("org.freedesktop.systemd1", dbus.ObjectPath("/org/freedesktop/systemd1"))
	var unitPath dbus.ObjectPath
	if err := mgr.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.GetUnit", 0, unit).Store(&unitPath); err != nil {
		// Try LoadUnit in case the unit is inactive and currently unloaded.
		if loadErr := mgr.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.LoadUnit", 0, unit).Store(&unitPath); loadErr != nil {
			return UnitState{}, fmt.Errorf("systemd GetUnit/LoadUnit %q: %w", unit, err)
		}
	}

	unitObj := conn.Object("org.freedesktop.systemd1", unitPath)
	activeState, err := readUnitPropString(ctx, unitObj, "ActiveState")
	if err != nil {
		return UnitState{}, err
	}
	subState, _ := readUnitPropString(ctx, unitObj, "SubState")
	desc, _ := readUnitPropString(ctx, unitObj, "Description")

	return UnitState{
		Status:      mapSystemdState(activeState),
		ActiveState: activeState,
		SubState:    subState,
		Description: desc,
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

func readUnitPropString(ctx context.Context, obj dbus.BusObject, prop string) (string, error) {
	var variant dbus.Variant
	err := obj.CallWithContext(
		ctx,
		"org.freedesktop.DBus.Properties.Get",
		0,
		"org.freedesktop.systemd1.Unit",
		prop,
	).Store(&variant)
	if err != nil {
		return "", fmt.Errorf("read unit property %s: %w", prop, err)
	}
	s, ok := variant.Value().(string)
	if !ok {
		return "", fmt.Errorf("unexpected property %s type %T", prop, variant.Value())
	}
	return s, nil
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
