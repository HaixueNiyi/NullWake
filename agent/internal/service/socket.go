package service

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nullwake/nullwake/agent/internal/protocol"
)

const (
	maxSocketConns    = 32
	socketReadTimeout = 30 * time.Second
)

// SocketServer accepts single-line JSON push events over a Unix Domain Socket
// and inserts them into the in-memory RingBuffer without waking hardware collectors.
type SocketServer struct {
	path     string
	mode     os.FileMode
	rb       *RingBuffer
	listener net.Listener
	sem      chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	closed   bool
}

// NewSocketServer creates and starts listening on a Unix Domain Socket.
func NewSocketServer(path string, mode os.FileMode, rb *RingBuffer) (*SocketServer, error) {
	if mode == 0 {
		mode = 0o666
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create socket parent dir: %w", err)
	}
	// Remove stale socket file if present.
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("socket path %q exists and is not a Unix socket", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove stale socket %q: %w", path, err)
		}
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen unix %q: %w", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		_ = ln.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("chmod unix socket %q (%o): %w", path, mode, err)
	}

	s := &SocketServer{
		path:     path,
		mode:     mode,
		rb:       rb,
		listener: ln,
		sem:      make(chan struct{}, maxSocketConns),
	}

	s.wg.Add(1)
	go s.acceptLoop()
	return s, nil
}

func (s *SocketServer) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed || errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		select {
		case s.sem <- struct{}{}:
			s.wg.Add(1)
			go s.handleConn(conn)
		default:
			// Shed excess concurrent local writers rather than exhausting file descriptors.
			_ = conn.Close()
		}
	}
}

func (s *SocketServer) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer func() { <-s.sem }()
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(socketReadTimeout))
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		_ = conn.SetReadDeadline(time.Now().Add(socketReadTimeout))
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev protocol.PushEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.ServiceID == "" || ev.Event == "" {
			continue
		}
		s.rb.Push(ev.ServiceID, protocol.HistoryEvent{
			TS:    ev.TS,
			Event: ev.Event,
		})
	}
}

// Close stops accepting new connections, waits for active readers, and removes the socket file.
func (s *SocketServer) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	err := s.listener.Close()
	s.wg.Wait()
	_ = os.Remove(s.path)
	return err
}
