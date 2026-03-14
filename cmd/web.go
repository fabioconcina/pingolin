package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fabioconcina/pingolin/internal/outage"
	"github.com/fabioconcina/pingolin/internal/prober"
	"github.com/fabioconcina/pingolin/internal/store"
	"github.com/fabioconcina/pingolin/internal/web"
)

var webListen string

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start web dashboard server",
	Long:  "Start an HTTP server serving a live dashboard for viewing in a browser.",
	RunE:  runWeb,
}

func init() {
	webCmd.Flags().StringVar(&webListen, "listen", "", "listen address (default: 0.0.0.0:8080)")
}

func runWeb(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if !verbose {
		log.SetOutput(os.Stderr)
	}

	s, err := store.New(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	// Start retention cleanup
	stopRetention := make(chan struct{})
	go s.StartRetentionCleanup(cfg.Storage.Retention.Duration, stopRetention)
	defer close(stopRetention)

	// Start embedded prober if daemon is not running
	if !isDaemonRunning() {
		od := outage.NewDetector(s, cfg.Targets.ICMP, cfg.Outage.ConsecutiveFailures)
		p := prober.New(s, cfg, od)
		p.Verbose = cfg.Verbose
		p.Start()
		defer p.Stop()
	}

	// Determine listen address
	listen := webListen
	if listen == "" {
		listen = fmt.Sprintf("%s:%d", cfg.Web.Listen, cfg.Web.Port)
	}

	srv := web.NewServer(s, cfg.Targets.ICMP, version, listen)

	// Graceful shutdown on signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down web server...")
		os.Exit(0)
	}()

	return srv.Start()
}
