package tsnetbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/tsnet"
)

const (
	defaultAgentPort  = "9100"
	defaultIntervalMs = 1000
	pingInterval      = 2 * time.Second
)

// BridgeListener receives asynchronous lifecycle, AuthURL, and telemetry callbacks from Go.
// All parameter types adhere strictly to gomobile bind rules.
type BridgeListener interface {
	OnAuthURL(url string)
	OnStateChange(state string)
	OnTelemetry(jsonPayload string)
	OnError(errMsg string)
}

type peerNode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IP       string `json:"ip"`
	Online   bool   `json:"online"`
	ConnMode string `json:"conn_mode"`
	Relay    string `json:"relay"`
	LastSeen int64  `json:"last_seen"`
}

type clientMessage struct {
	Type       string `json:"type"`
	IntervalMs int64  `json:"interval_ms,omitempty"`
}

var defaultBridge = NewBridge()

// Start initializes the default embedded tsnet bridge instance.
func Start(stateDir string, authKey string, proxyURL string, listener BridgeListener) error {
	return defaultBridge.Start(stateDir, authKey, proxyURL, listener)
}

// GetOnlineNodes returns a JSON array string of Tailscale peers on the default bridge.
func GetOnlineNodes() string {
	return defaultBridge.GetOnlineNodes()
}

// SubscribeNode opens a WebSocket stream to targetAddr over tsnet on the default bridge.
func SubscribeNode(targetAddr string, intervalMs int64) error {
	return defaultBridge.SubscribeNode(targetAddr, intervalMs)
}

// Unsubscribe closes the active telemetry subscription on the default bridge.
func Unsubscribe() {
	defaultBridge.Unsubscribe()
}

// Stop shuts down both the active subscription and the tsnet server on the default bridge.
func Stop() {
	defaultBridge.Stop()
}

// Bridge manages an embedded userspace tsnet node and WebSocket telemetry stream.
type Bridge struct {
	mu          sync.Mutex
	ts          *tsnet.Server
	localClient *local.Client
	listener    BridgeListener
	watchCancel context.CancelFunc
	watchDone   chan struct{}

	subMu     sync.Mutex
	subConn   *websocket.Conn
	subCancel context.CancelFunc
	subDone   chan struct{}
}

// NewBridge creates a new Bridge controller.
func NewBridge() *Bridge {
	return &Bridge{}
}

// Start configures optional control-plane proxy env vars and boots userspace tsnet without Android VpnService.
func (b *Bridge) Start(stateDir string, authKey string, proxyURL string, listener BridgeListener) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("tsnet start panic: %v\n%s", r, string(debug.Stack()))
			if listener != nil {
				listener.OnError(msg)
			}
			retErr = errors.New(msg)
		}
	}()

	b.Stop()

	if err := applyProxyEnv(proxyURL); err != nil {
		return err
	}
	if stateDir == "" {
		return errors.New("stateDir is required")
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	ts := &tsnet.Server{
		Dir:      stateDir,
		Hostname: "nullwake-android",
		AuthKey:  authKey,
	}
	if err := ts.Start(); err != nil {
		return fmt.Errorf("start tsnet: %w", err)
	}

	lc, err := ts.LocalClient()
	if err != nil {
		_ = ts.Close()
		return fmt.Errorf("tsnet local client: %w", err)
	}

	watchCtx, watchCancel := context.WithCancel(context.Background())
	watchDone := make(chan struct{})

	b.mu.Lock()
	b.ts = ts
	b.localClient = lc
	b.listener = listener
	b.watchCancel = watchCancel
	b.watchDone = watchDone
	b.mu.Unlock()

	go b.watchIPNBus(watchCtx, lc, listener, watchDone)
	return nil
}

// SetListener updates the callback listener (primarily useful when using SubscribeNode standalone in tests).
func (b *Bridge) SetListener(listener BridgeListener) {
	b.mu.Lock()
	b.listener = listener
	b.mu.Unlock()
}

// GetOnlineNodes returns a JSON-encoded slice of tailnet peer descriptors.
func (b *Bridge) GetOnlineNodes() string {
	b.mu.Lock()
	lc := b.localClient
	b.mu.Unlock()

	if lc == nil {
		return "[]"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	status, err := lc.Status(ctx)
	if err != nil || status == nil {
		return "[]"
	}

	nodes := make([]peerNode, 0, len(status.Peer))
	for _, peer := range status.Peer {
		if peer == nil || peer.ShareeNode {
			continue
		}
		var ip string
		for _, addr := range peer.TailscaleIPs {
			if addr.Is4() {
				ip = addr.String()
				break
			}
		}
		if ip == "" && len(peer.TailscaleIPs) > 0 {
			ip = peer.TailscaleIPs[0].String()
		}
		if ip == "" {
			continue
		}

		name := peer.HostName
		if name == "" && peer.DNSName != "" {
			name = strings.Split(strings.TrimSuffix(peer.DNSName, "."), ".")[0]
		}
		if name == "" {
			name = ip
		}

		connMode := "DERP"
		if peer.CurAddr != "" {
			connMode = "P2P"
		}

		var lastSeen int64
		if !peer.LastSeen.IsZero() {
			lastSeen = peer.LastSeen.Unix()
		}

		nodes = append(nodes, peerNode{
			ID:       string(peer.ID),
			Name:     name,
			IP:       ip,
			Online:   peer.Online,
			ConnMode: connMode,
			Relay:    peer.Relay,
			LastSeen: lastSeen,
		})
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Online != nodes[j].Online {
			return nodes[i].Online
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})

	encoded, err := json.Marshal(nodes)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// SubscribeNode dials targetAddr over embedded tsnet (or direct TCP if tsnet isn't started),
// sends SUBSCRIBE, streams incoming TELEMETRY JSON frames to BridgeListener, and emits PING every 2s.
func (b *Bridge) SubscribeNode(targetAddr string, intervalMs int64) error {
	b.Unsubscribe()

	hostPort, err := normalizeTargetAddr(targetAddr)
	if err != nil {
		return err
	}
	if intervalMs <= 0 {
		intervalMs = defaultIntervalMs
	}

	b.mu.Lock()
	ts := b.ts
	listener := b.listener
	b.mu.Unlock()

	dialer := &net.Dialer{Timeout: 8 * time.Second}
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if ts != nil {
					return ts.Dial(ctx, network, addr)
				}
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dialCancel()

	wsURL := "ws://" + hostPort + "/ws"
	conn, _, err := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{
		HTTPClient: httpClient,
	})
	if err != nil {
		if listener != nil {
			listener.OnError(fmt.Sprintf("connect %s: %v", wsURL, err))
		}
		return fmt.Errorf("dial websocket %s: %w", wsURL, err)
	}

	subCtx, subCancel := context.WithCancel(context.Background())
	subDone := make(chan struct{})

	subMsg, _ := json.Marshal(clientMessage{
		Type:       "SUBSCRIBE",
		IntervalMs: intervalMs,
	})
	writeCtx, writeCancel := context.WithTimeout(subCtx, 3*time.Second)
	err = conn.Write(writeCtx, websocket.MessageText, subMsg)
	writeCancel()
	if err != nil {
		subCancel()
		_ = conn.CloseNow()
		if listener != nil {
			listener.OnError(fmt.Sprintf("subscribe send failed: %v", err))
		}
		return fmt.Errorf("send SUBSCRIBE: %w", err)
	}

	b.subMu.Lock()
	b.subConn = conn
	b.subCancel = subCancel
	b.subDone = subDone
	b.subMu.Unlock()

	go b.runSubscription(subCtx, subCancel, conn, listener, subDone)
	return nil
}

// Unsubscribe sends UNSUBSCRIBE, stops the 2-second PING heartbeat, and closes the WebSocket.
func (b *Bridge) Unsubscribe() {
	b.subMu.Lock()
	conn := b.subConn
	cancel := b.subCancel
	done := b.subDone
	b.subConn = nil
	b.subCancel = nil
	b.subDone = nil
	b.subMu.Unlock()

	if conn != nil {
		unsubPayload, _ := json.Marshal(clientMessage{Type: "UNSUBSCRIBE"})
		wCtx, wCancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		_ = conn.Write(wCtx, websocket.MessageText, unsubPayload)
		wCancel()
		_ = conn.Close(websocket.StatusNormalClosure, "unsubscribe")
	}
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

// Stop terminates any active subscription and shuts down the embedded tsnet instance.
func (b *Bridge) Stop() {
	b.Unsubscribe()

	b.mu.Lock()
	watchCancel := b.watchCancel
	watchDone := b.watchDone
	ts := b.ts
	b.watchCancel = nil
	b.watchDone = nil
	b.ts = nil
	b.localClient = nil
	b.mu.Unlock()

	if watchCancel != nil {
		watchCancel()
	}
	if watchDone != nil {
		<-watchDone
	}
	if ts != nil {
		_ = ts.Close()
	}
}

func (b *Bridge) runSubscription(
	ctx context.Context,
	cancel context.CancelFunc,
	conn *websocket.Conn,
	listener BridgeListener,
	done chan struct{},
) {
	defer close(done)
	defer cancel()
	defer conn.CloseNow()

	var writeMu sync.Mutex
	pingPayload, _ := json.Marshal(clientMessage{Type: "PING"})

	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				writeMu.Lock()
				wCtx, wCancel := context.WithTimeout(ctx, 2*time.Second)
				err := conn.Write(wCtx, websocket.MessageText, pingPayload)
				wCancel()
				writeMu.Unlock()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()

	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() == nil && listener != nil {
				listener.OnError(fmt.Sprintf("stream closed: %v", err))
			}
			return
		}
		if msgType == websocket.MessageText && listener != nil {
			listener.OnTelemetry(string(data))
		}
	}
}

func (b *Bridge) watchIPNBus(ctx context.Context, lc *local.Client, listener BridgeListener, done chan struct{}) {
	defer close(done)
	defer func() {
		if r := recover(); r != nil && listener != nil {
			listener.OnError(fmt.Sprintf("ipn bus panic: %v\n%s", r, string(debug.Stack())))
		}
	}()

	var lastState string
	var lastAuthURL string

	watcher, err := lc.WatchIPNBus(ctx, ipn.NotifyInitialState)
	if err != nil {
		if ctx.Err() == nil && listener != nil {
			listener.OnError(fmt.Sprintf("watch ipn bus: %v", err))
		}
		return
	}
	defer watcher.Close()

	for {
		n, err := watcher.Next()
		if err != nil {
			if ctx.Err() == nil && listener != nil {
				listener.OnError(fmt.Sprintf("ipn bus error: %v", err))
			}
			return
		}
		if n.State != nil {
			stateStr := n.State.String()
			if stateStr != lastState {
				lastState = stateStr
				if listener != nil {
					listener.OnStateChange(stateStr)
				}
			}
		}
		if n.BrowseToURL != nil && *n.BrowseToURL != "" && *n.BrowseToURL != lastAuthURL {
			lastAuthURL = *n.BrowseToURL
			if listener != nil {
				listener.OnAuthURL(*n.BrowseToURL)
			}
		}
	}
}

func applyProxyEnv(proxyURL string) error {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
			_ = os.Unsetenv(k)
		}
		return nil
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid proxy URL %q (expected scheme://host:port)", proxyURL)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		return fmt.Errorf("unsupported proxy scheme %q", parsed.Scheme)
	}

	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		if err := os.Setenv(k, proxyURL); err != nil {
			return fmt.Errorf("set env %s: %w", k, err)
		}
	}
	return nil
}

func normalizeTargetAddr(targetAddr string) (string, error) {
	trimmed := strings.TrimSpace(targetAddr)
	if trimmed == "" {
		return "", errors.New("targetAddr is empty")
	}
	if _, _, err := net.SplitHostPort(trimmed); err == nil {
		return trimmed, nil
	}
	// Strip accidental brackets around bare IPv6 before joining default port.
	trimmed = strings.TrimPrefix(strings.TrimSuffix(trimmed, "]"), "[")
	return net.JoinHostPort(trimmed, defaultAgentPort), nil
}
