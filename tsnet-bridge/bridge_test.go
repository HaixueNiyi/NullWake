package tsnetbridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func isGomobileSupportedType(t reflect.Type) bool {
	if t == nil {
		return true
	}
	// error interface is supported as return type.
	errType := reflect.TypeOf((*error)(nil)).Elem()
	if t.Implements(errType) && t == errType {
		return true
	}
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64, reflect.String:
		return true
	case reflect.Slice:
		return t.Elem().Kind() == reflect.Uint8 // []byte
	case reflect.Ptr:
		return t.Elem().Kind() == reflect.Struct
	case reflect.Interface:
		return true
	default:
		return false
	}
}

func TestGomobileExportedSignatures(t *testing.T) {
	// Verify BridgeListener methods obey gomobile type constraints.
	listenerType := reflect.TypeOf((*BridgeListener)(nil)).Elem()
	for i := 0; i < listenerType.NumMethod(); i++ {
		m := listenerType.Method(i)
		for p := 0; p < m.Type.NumIn(); p++ {
			if !isGomobileSupportedType(m.Type.In(p)) {
				t.Fatalf("BridgeListener.%s param %d (%v) unsupported by gomobile", m.Name, p, m.Type.In(p))
			}
		}
		if m.Type.NumOut() > 2 {
			t.Fatalf("BridgeListener.%s has %d return values (max 2)", m.Name, m.Type.NumOut())
		}
		for r := 0; r < m.Type.NumOut(); r++ {
			if !isGomobileSupportedType(m.Type.Out(r)) {
				t.Fatalf("BridgeListener.%s return %d (%v) unsupported by gomobile", m.Name, r, m.Type.Out(r))
			}
		}
	}

	// Verify Bridge methods obey gomobile type constraints.
	bridgeType := reflect.TypeOf(&Bridge{})
	for i := 0; i < bridgeType.NumMethod(); i++ {
		m := bridgeType.Method(i)
		for p := 1; p < m.Type.NumIn(); p++ { // skip receiver
			if !isGomobileSupportedType(m.Type.In(p)) {
				t.Fatalf("Bridge.%s param %d (%v) unsupported by gomobile", m.Name, p, m.Type.In(p))
			}
		}
		for r := 0; r < m.Type.NumOut(); r++ {
			if !isGomobileSupportedType(m.Type.Out(r)) {
				t.Fatalf("Bridge.%s return %d (%v) unsupported by gomobile", m.Name, r, m.Type.Out(r))
			}
		}
	}
}

func TestApplyProxyEnv(t *testing.T) {
	t.Cleanup(func() { _ = applyProxyEnv("") })

	if err := applyProxyEnv("socks5://127.0.0.1:10808"); err != nil {
		t.Fatalf("apply valid socks5 proxy failed: %v", err)
	}
	if got := os.Getenv("HTTP_PROXY"); got != "socks5://127.0.0.1:10808" {
		t.Fatalf("HTTP_PROXY = %q, want socks5://127.0.0.1:10808", got)
	}
	if got := os.Getenv("HTTPS_PROXY"); got != "socks5://127.0.0.1:10808" {
		t.Fatalf("HTTPS_PROXY = %q, want socks5://127.0.0.1:10808", got)
	}

	if err := applyProxyEnv("ftp://bad-scheme:21"); err == nil {
		t.Fatalf("expected error for unsupported proxy scheme")
	}

	if err := applyProxyEnv(""); err != nil {
		t.Fatalf("clear proxy failed: %v", err)
	}
	if got := os.Getenv("HTTP_PROXY"); got != "" {
		t.Fatalf("expected cleared HTTP_PROXY, got %q", got)
	}
}

func TestEnsureRuntimeDirsSetsAndroidFallbackEnv(t *testing.T) {
	for _, k := range []string{"TS_LOGS_DIR", "TMPDIR", "HOME", "XDG_CACHE_HOME", "XDG_CONFIG_HOME"} {
		t.Setenv(k, "")
	}

	stateDir := t.TempDir()
	if err := ensureRuntimeDirs(stateDir); err != nil {
		t.Fatalf("ensureRuntimeDirs failed: %v", err)
	}
	if got := os.Getenv("TS_LOGS_DIR"); got == "" {
		t.Fatalf("expected TS_LOGS_DIR to be set")
	}
	if _, err := os.UserCacheDir(); err != nil {
		t.Fatalf("expected os.UserCacheDir() to succeed, got: %v", err)
	}
	if _, err := os.UserConfigDir(); err != nil {
		t.Fatalf("expected os.UserConfigDir() to succeed, got: %v", err)
	}
}

type mockBridgeListener struct {
	mu        sync.Mutex
	telemetry []string
	errors    []string
}

func (m *mockBridgeListener) OnAuthURL(string)     {}
func (m *mockBridgeListener) OnStateChange(string) {}
func (m *mockBridgeListener) OnTelemetry(payload string) {
	m.mu.Lock()
	m.telemetry = append(m.telemetry, payload)
	m.mu.Unlock()
}
func (m *mockBridgeListener) OnError(errMsg string) {
	m.mu.Lock()
	m.errors = append(m.errors, errMsg)
	m.mu.Unlock()
}

func TestSubscribeNodeAndUnsubscribe(t *testing.T) {
	var gotSub, gotUnsub bool
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" {
			http.NotFound(w, r)
			return
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.CloseNow()

		ctx := r.Context()
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var msg clientMessage
		if err := json.Unmarshal(raw, &msg); err == nil && msg.Type == "SUBSCRIBE" && msg.IntervalMs == 500 {
			mu.Lock()
			gotSub = true
			mu.Unlock()
		}

		_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"TELEMETRY","timestamp":1730000000,"data":{"cpu":{"usage_percent":10}}}`))

		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var m clientMessage
			if err := json.Unmarshal(raw, &m); err == nil && m.Type == "UNSUBSCRIBE" {
				mu.Lock()
				gotUnsub = true
				mu.Unlock()
				return
			}
		}
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	listener := &mockBridgeListener{}
	b := NewBridge()
	b.SetListener(listener)

	if err := b.SubscribeNode(addr, 500); err != nil {
		t.Fatalf("SubscribeNode failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		listener.mu.Lock()
		n := len(listener.telemetry)
		listener.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	b.Unsubscribe()

	listener.mu.Lock()
	if len(listener.telemetry) == 0 || !strings.Contains(listener.telemetry[0], `"TELEMETRY"`) {
		listener.mu.Unlock()
		t.Fatalf("expected telemetry callback, got %v", listener.telemetry)
	}
	listener.mu.Unlock()

	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		ok := gotSub && gotUnsub
		mu.Unlock()
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	t.Fatalf("expected SUBSCRIBE (%v) and UNSUBSCRIBE (%v) frames received by server", gotSub, gotUnsub)
}
