package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fabioconcina/pingolin/internal/config"
	"github.com/fabioconcina/pingolin/internal/outage"
	"github.com/fabioconcina/pingolin/internal/prober"
	"github.com/fabioconcina/pingolin/internal/store"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run as headless daemon (prober only, no TUI)",
	RunE:  runDaemon,
}

func runDaemon(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Check if daemon is already running
	if isDaemonRunning() {
		return fmt.Errorf("daemon is already running")
	}

	s, err := store.New(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	// Write PID file
	pidPath := config.DefaultPIDPath()
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		return fmt.Errorf("creating PID directory: %w", err)
	}
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0o644); err != nil {
		return fmt.Errorf("writing PID file: %w", err)
	}
	defer os.Remove(pidPath)

	od := outage.NewDetector(s, cfg.Targets.ICMP, cfg.Outage.ConsecutiveFailures)
	p := prober.New(s, cfg, od)
	p.Verbose = cfg.Verbose
	p.Start()
	defer p.Stop()

	// Start retention cleanup
	stopRetention := make(chan struct{})
	go s.StartRetentionCleanup(cfg.Storage.Retention.Duration, stopRetention)

	log.Printf("pingolin daemon started (pid: %d)", os.Getpid())
	log.Printf("targets: %v", cfg.Targets.ICMP)
	log.Printf("database: %s", cfg.Storage.Path)

	// Wait for signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// SIGHUP reloads config
	hupCh := make(chan os.Signal, 1)
	signal.Notify(hupCh, syscall.SIGHUP)

	for {
		select {
		case sig := <-sigCh:
			log.Printf("received %s, shutting down", sig)
			close(stopRetention)
			return nil
		case <-hupCh:
			log.Printf("received SIGHUP, reloading config")
			newCfg, err := loadConfig()
			if err != nil {
				log.Printf("error reloading config: %v", err)
				continue
			}
			// Restart prober with new config
			p.Stop()
			od = outage.NewDetector(s, newCfg.Targets.ICMP, newCfg.Outage.ConsecutiveFailures)
			p = prober.New(s, newCfg, od)
			p.Verbose = newCfg.Verbose
			p.Start()
			log.Printf("config reloaded successfully")
		}
	}
}
