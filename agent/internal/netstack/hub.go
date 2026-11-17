package netstack

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/nullwake/nullwake/agent/internal/collector"
	"github.com/nullwake/nullwake/agent/internal/protocol"
)

const (
	defaultIntervalMs     = 1000
	minIntervalMs         = 200
	maxIntervalMs         = 60000
	defaultWatchdogPeriod = 5 * time.Second
)

// HardwareCollector gathers hardware telemetry strictly on demand.
type HardwareCollector interface {
	Collect(ctx context.Context) (collector.Snapshot, error)
	Reset()
}

// ServiceMonitor gathers service statuses and history strictly on demand.
type ServiceMonitor interface {
	CollectServices(ctx context.Context) []protocol.ServiceStatus
	Reset()
}

// Hub manages WebSocket sessions and enforces strict on-demand telemetry collection.
type Hub struct {
	collector       HardwareCollector
	monitor         ServiceMonitor
	watchdogTimeout time.Duration
	logger          *log.Logger

	mu          sync.Mutex
	subscribers int
}

// NewHub constructs a Hub with a 5-second ping watchdog timeout.
func NewHub(c HardwareCollector, m ServiceMonitor, logger *log.Logger) *Hub {
	if logger == nil {
		logger = log.Default()
	}
	return &Hub{
		collector:       c,
		monitor:         m,
		watchdogTimeout: defaultWatchdogPeriod,
		logger:          logger,
	}
}

// SetWatchdogTimeout overrides the ping timeout (used by tests).
func (h *Hub) SetWatchdogTimeout(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if d > 0 {
		h.watchdogTimeout = d
	}
}

// ActiveSubscribers returns the number of currently subscribed sessions.
func (h *Hub) ActiveSubscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.subscribers
}

// Handler returns an http.Handler routing /ws to the WebSocket session handler.
func (h *Hub) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.ServeWS)
	return mux
}

// ServeWS upgrades an incoming HTTP request to a NullWake WebSocket session.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		h.logger.Printf("websocket accept error: %v", err)
		return
	}
	conn.SetReadLimit(16 * 1024)
	defer conn.CloseNow()

	h.handleSession(r.Context(), conn)
}

func (h *Hub) handleSession(parentCtx context.Context, conn *websocket.Conn) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	sess := &wsSession{
		hub:  h,
		conn: conn,
	}
	defer sess.stopSubscription()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		var msg protocol.ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case protocol.MsgSubscribe:
			sess.startSubscription(ctx, cancel, clampInterval(msg.IntervalMs))
		case protocol.MsgPing:
			sess.touchPing()
		case protocol.MsgUnsubscribe:
			sess.stopSubscription()
		}
	}
}

func (h *Hub) acquireSubscriber() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subscribers++
}

func (h *Hub) releaseSubscriber() {
	h.mu.Lock()
	if h.subscribers > 0 {
		h.subscribers--
	}
	remaining := h.subscribers
	h.mu.Unlock()

	if remaining == 0 {
		if h.collector != nil {
			h.collector.Reset()
		}
		if h.monitor != nil {
			h.monitor.Reset()
		}
	}
}

func (h *Hub) collectPayload(ctx context.Context) ([]byte, error) {
	var snap collector.Snapshot
	if h.collector != nil {
		var err error
		snap, err = h.collector.Collect(ctx)
		if err != nil {
			return nil, err
		}
	}

	var services []protocol.ServiceStatus
	if h.monitor != nil {
		services = h.monitor.CollectServices(ctx)
	}

	msg := protocol.NewTelemetryMessage(time.Now().Unix(), protocol.TelemetryData{
		CPU:      snap.CPU,
		RAM:      snap.RAM,
		Net:      snap.Net,
		GPU:      snap.GPU,
		Services: services,
	})
	return json.Marshal(msg)
}

type wsSession struct {
	hub  *Hub
	conn *websocket.Conn

	writeMu sync.Mutex

	subMu      sync.Mutex
	subscribed bool
	subCancel  context.CancelFunc
	subDone    chan struct{}
	lastPingNs atomic.Int64
}

func (s *wsSession) touchPing() {
	s.lastPingNs.Store(time.Now().UnixNano())
}

func (s *wsSession) startSubscription(sessionCtx context.Context, closeSession context.CancelFunc, interval time.Duration) {
	// Replace any existing subscription on this connection cleanly.
	s.stopSubscription()

	s.subMu.Lock()
	subCtx, subCancel := context.WithCancel(sessionCtx)
	done := make(chan struct{})
	s.subscribed = true
	s.subCancel = subCancel
	s.subDone = done
	s.touchPing()
	s.subMu.Unlock()

	s.hub.acquireSubscriber()

	s.hub.mu.Lock()
	watchdogTimeout := s.hub.watchdogTimeout
	s.hub.mu.Unlock()

	go s.runLoop(subCtx, closeSession, done, interval, watchdogTimeout)
}

func (s *wsSession) stopSubscription() {
	s.subMu.Lock()
	if !s.subscribed {
		s.subMu.Unlock()
		return
	}
	s.subscribed = false
	cancel := s.subCancel
	done := s.subDone
	s.subCancel = nil
	s.subDone = nil
	s.subMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	s.hub.releaseSubscriber()
}

func (s *wsSession) runLoop(
	ctx context.Context,
	closeSession context.CancelFunc,
	done chan struct{},
	interval time.Duration,
	watchdogTimeout time.Duration,
) {
	defer close(done)

	// Warm up delta baselines (CPU, Net, Intel RC6) briefly so the initial frame is accurate.
	if s.hub.collector != nil {
		warmup := 150 * time.Millisecond
		if capWarmup := watchdogTimeout / 3; capWarmup < warmup {
			warmup = capWarmup
		}
		_, _ = s.hub.collector.Collect(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(warmup):
		}
		s.touchPing()
	}

	// Send initial telemetry + ring-buffer history immediately upon SUBSCRIBE.
	if err := s.sendTelemetry(ctx); err != nil && !errors.Is(err, context.Canceled) {
		closeSession()
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	watchdogPeriod := watchdogTimeout / 4
	if watchdogPeriod > 500*time.Millisecond {
		watchdogPeriod = 500 * time.Millisecond
	}
	if watchdogPeriod < 10*time.Millisecond {
		watchdogPeriod = 10 * time.Millisecond
	}
	watchdog := time.NewTicker(watchdogPeriod)
	defer watchdog.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-watchdog.C:
			lastPing := time.Unix(0, s.lastPingNs.Load())
			if time.Since(lastPing) > watchdogTimeout {
				s.hub.logger.Printf("client ping watchdog expired (>%v); terminating session", watchdogTimeout)
				_ = s.conn.Close(websocket.StatusPolicyViolation, "ping timeout")
				closeSession()
				return
			}
		case <-ticker.C:
			if err := s.sendTelemetry(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					closeSession()
				}
				return
			}
		}
	}
}

func (s *wsSession) sendTelemetry(ctx context.Context) error {
	payload, err := s.hub.collectPayload(ctx)
	if err != nil {
		return err
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.conn.Write(writeCtx, websocket.MessageText, payload)
}

func clampInterval(ms int) time.Duration {
	if ms <= 0 {
		ms = defaultIntervalMs
	}
	if ms < minIntervalMs {
		ms = minIntervalMs
	}
	if ms > maxIntervalMs {
		ms = maxIntervalMs
	}
	return time.Duration(ms) * time.Millisecond
}
