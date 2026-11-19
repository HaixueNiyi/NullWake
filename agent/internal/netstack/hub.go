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

// Hub manages WebSocket sessions and enforces a single shared on-demand collection loop.
type Hub struct {
	collector       HardwareCollector
	monitor         ServiceMonitor
	watchdogTimeout time.Duration
	logger          *log.Logger

	mu             sync.Mutex
	sessions       map[*wsSession]time.Duration
	loopRunning    bool
	loopCancel     context.CancelFunc
	loopDone       chan struct{}
	intervalNotify chan struct{}
	warmedUp       bool
	lastPayload    []byte
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
		sessions:        make(map[*wsSession]time.Duration),
		intervalNotify:  make(chan struct{}, 1),
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
	return len(h.sessions)
}

// Handler returns an http.Handler routing /ws to the WebSocket session handler.
func (h *Hub) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.ServeWS)
	return mux
}

// ServeWS upgrades an incoming HTTP request to a NullWake WebSocket session.
// Requests without an Origin header (native Go/Android clients) and localhost origins are permitted.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*", "[::1]:*"},
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

func (h *Hub) acquireSession(s *wsSession, interval time.Duration) time.Duration {
	h.mu.Lock()
	h.sessions[s] = interval
	watchdogTimeout := h.watchdogTimeout
	if h.warmedUp && len(h.lastPayload) > 0 {
		s.enqueue(h.lastPayload)
	}
	if !h.loopRunning {
		loopCtx, loopCancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		h.loopRunning = true
		h.loopCancel = loopCancel
		h.loopDone = done
		go h.runCollectorLoop(loopCtx, done, watchdogTimeout)
	} else {
		select {
		case h.intervalNotify <- struct{}{}:
		default:
		}
	}
	h.mu.Unlock()
	return watchdogTimeout
}

func (h *Hub) releaseSession(s *wsSession) {
	h.mu.Lock()
	delete(h.sessions, s)
	remaining := len(h.sessions)
	var cancel context.CancelFunc
	var done chan struct{}
	if remaining == 0 && h.loopRunning {
		cancel = h.loopCancel
		done = h.loopDone
		h.loopRunning = false
		h.loopCancel = nil
		h.loopDone = nil
		h.warmedUp = false
		h.lastPayload = nil
	} else if remaining > 0 {
		select {
		case h.intervalNotify <- struct{}{}:
		default:
		}
	}
	h.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	if remaining == 0 {
		if h.collector != nil {
			h.collector.Reset()
		}
		if h.monitor != nil {
			h.monitor.Reset()
		}
	}
}

func (h *Hub) minInterval() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	min := time.Duration(defaultIntervalMs) * time.Millisecond
	first := true
	for _, iv := range h.sessions {
		if first || iv < min {
			min = iv
			first = false
		}
	}
	return min
}

func (h *Hub) runCollectorLoop(ctx context.Context, done chan struct{}, watchdogTimeout time.Duration) {
	defer close(done)

	// Warm up delta baselines (CPU, Net, Intel RC6) once across all subscribers.
	if h.collector != nil {
		warmup := 150 * time.Millisecond
		if capWarmup := watchdogTimeout / 3; capWarmup < warmup {
			warmup = capWarmup
		}
		_, _ = h.collector.Collect(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(warmup):
		}
		h.mu.Lock()
		for s := range h.sessions {
			s.touchPing()
		}
		h.warmedUp = true
		h.mu.Unlock()
	}

	h.collectAndBroadcast(ctx)

	curInterval := h.minInterval()
	ticker := time.NewTicker(curInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.intervalNotify:
			next := h.minInterval()
			if next != curInterval {
				curInterval = next
				ticker.Reset(curInterval)
			}
		case <-ticker.C:
			h.collectAndBroadcast(ctx)
		}
	}
}

func (h *Hub) collectAndBroadcast(ctx context.Context) {
	payload, err := h.collectPayload(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			h.logger.Printf("telemetry collect error: %v", err)
		}
		return
	}

	h.mu.Lock()
	h.lastPayload = payload
	targets := make([]*wsSession, 0, len(h.sessions))
	for s := range h.sessions {
		targets = append(targets, s)
	}
	h.mu.Unlock()

	for _, s := range targets {
		s.enqueue(payload)
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
	sendCh     chan []byte
	lastPingNs atomic.Int64
}

func (s *wsSession) touchPing() {
	s.lastPingNs.Store(time.Now().UnixNano())
}

func (s *wsSession) enqueue(payload []byte) {
	s.subMu.Lock()
	ch := s.sendCh
	subscribed := s.subscribed
	s.subMu.Unlock()
	if !subscribed || ch == nil {
		return
	}
	// Non-blocking send; drop stale buffered frame if client is lagging.
	select {
	case ch <- payload:
	default:
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- payload:
		default:
		}
	}
}

func (s *wsSession) startSubscription(sessionCtx context.Context, closeSession context.CancelFunc, interval time.Duration) {
	// Replace any existing subscription on this connection cleanly.
	s.stopSubscription()

	s.subMu.Lock()
	subCtx, subCancel := context.WithCancel(sessionCtx)
	done := make(chan struct{})
	sendCh := make(chan []byte, 2)
	s.subscribed = true
	s.subCancel = subCancel
	s.subDone = done
	s.sendCh = sendCh
	s.touchPing()
	s.subMu.Unlock()

	watchdogTimeout := s.hub.acquireSession(s, interval)

	go s.runLoop(subCtx, closeSession, done, sendCh, watchdogTimeout)
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
	s.sendCh = nil
	s.subMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	s.hub.releaseSession(s)
}

func (s *wsSession) runLoop(
	ctx context.Context,
	closeSession context.CancelFunc,
	done chan struct{},
	sendCh <-chan []byte,
	watchdogTimeout time.Duration,
) {
	defer close(done)

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
		case payload := <-sendCh:
			s.writeMu.Lock()
			writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err := s.conn.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			s.writeMu.Unlock()
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					closeSession()
				}
				return
			}
		}
	}
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
