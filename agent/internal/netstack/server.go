package netstack

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"tailscale.com/tsnet"
)

// ServerConfig configures the embedded tsnet listener or local debug listener.
type ServerConfig struct {
	Hostname    string
	StateDir    string
	AuthKey     string
	ListenPort  int
	ListenLocal string
}

// Serve starts the WebSocket HTTP server over embedded tsnet (or local TCP when ListenLocal is set)
// and blocks until ctx is cancelled or the server stops with an error.
func Serve(ctx context.Context, cfg ServerConfig, hub *Hub, logger *log.Logger) error {
	if logger == nil {
		logger = log.Default()
	}

	var ln net.Listener
	var cleanup func()

	if cfg.ListenLocal != "" {
		var err error
		ln, err = net.Listen("tcp", cfg.ListenLocal)
		if err != nil {
			return fmt.Errorf("listen local %q: %w", cfg.ListenLocal, err)
		}
		logger.Printf("nullwake-agent listening on local debug endpoint ws://%s/ws", ln.Addr().String())
		cleanup = func() { _ = ln.Close() }
	} else {
		if err := os.MkdirAll(cfg.StateDir, 0o700); err != nil {
			return fmt.Errorf("create tsnet state dir %q: %w", cfg.StateDir, err)
		}
		ts := &tsnet.Server{
			Dir:      cfg.StateDir,
			Hostname: cfg.Hostname,
			AuthKey:  cfg.AuthKey,
			UserLogf: logger.Printf,
		}
		if err := ts.Start(); err != nil {
			return fmt.Errorf("start tsnet server: %w", err)
		}

		port := cfg.ListenPort
		if port <= 0 {
			port = 9100
		}
		var err error
		ln, err = ts.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			_ = ts.Close()
			return fmt.Errorf("tsnet listen :%d: %w", port, err)
		}

		go func() {
			upCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if status, err := ts.Up(upCtx); err == nil && status != nil {
				ip4, _ := ts.TailscaleIPs()
				logger.Printf("tsnet ready: hostname=%s tailscale_ip=%s ws://%s:%d/ws", cfg.Hostname, ip4, ip4, port)
			}
		}()

		cleanup = func() {
			_ = ln.Close()
			_ = ts.Close()
		}
	}
	defer cleanup()

	httpSrv := &http.Server{
		Handler:           hub.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		err := httpSrv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
		return <-errCh
	case err := <-errCh:
		return err
	}
}
