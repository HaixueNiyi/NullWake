package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nullwake/nullwake/agent/internal/collector"
	"github.com/nullwake/nullwake/agent/internal/config"
	"github.com/nullwake/nullwake/agent/internal/netstack"
	"github.com/nullwake/nullwake/agent/internal/service"
)

var version = "1.2.0"

func main() {
	var (
		configPath  = flag.String("config", "", "Path to NullWake TOML config file")
		stateDir    = flag.String("state-dir", "", "Override tsnet state directory")
		socketPath  = flag.String("socket", "", "Override Unix Domain Socket path")
		listenLocal = flag.String("listen-local", "", "Optional local TCP listen address (e.g. 127.0.0.1:9100) bypassing tsnet for local testing")
		showVersion = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("nullwake-agent v%s\n", version)
		return
	}

	logger := log.New(os.Stdout, "[nullwake-agent] ", log.LstdFlags)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatalf("failed to load configuration: %v", err)
	}
	if *stateDir != "" {
		cfg.Tailscale.StateDir = *stateDir
	}
	if *socketPath != "" {
		cfg.Socket.Path = *socketPath
	}

	rb := service.NewRingBuffer(cfg.Socket.RingBufferSize)
	sockSrv, err := service.NewSocketServer(cfg.Socket.Path, os.FileMode(cfg.Socket.FileMode), rb)
	if err != nil {
		logger.Fatalf("failed to start unix push socket %q: %v", cfg.Socket.Path, err)
	}
	defer func() {
		if err := sockSrv.Close(); err != nil {
			logger.Printf("socket close warning: %v", err)
		}
	}()
	logger.Printf("push socket listening at %s (mode=%04o, ring_capacity=%d)", cfg.Socket.Path, os.FileMode(cfg.Socket.FileMode), cfg.Socket.RingBufferSize)

	hw := collector.New()
	defer hw.Close()

	mon := service.NewMonitor(cfg.Services, rb, nil)
	defer mon.Reset()

	hub := netstack.NewHub(hw, mon, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := netstack.Serve(ctx, netstack.ServerConfig{
		Hostname:    cfg.Tailscale.Hostname,
		StateDir:    cfg.Tailscale.StateDir,
		AuthKey:     cfg.Tailscale.AuthKey,
		ListenPort:  cfg.Tailscale.ListenPort,
		ListenLocal: *listenLocal,
	}, hub, logger); err != nil {
		logger.Fatalf("server exited with error: %v", err)
	}
	logger.Printf("nullwake-agent shut down cleanly")
}
