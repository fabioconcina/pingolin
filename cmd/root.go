package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/fabioconcina/pingolin/internal/config"
	"github.com/fabioconcina/pingolin/internal/outage"
	"github.com/fabioconcina/pingolin/internal/prober"
	"github.com/fabioconcina/pingolin/internal/store"
	"github.com/fabioconcina/pingolin/internal/tui"
)

var (
	version    = "dev"
	cfgFile    string
	dbPath     string
	pingInt    string
	dnsInt     string
	httpInt    string
	targets    string
	retention  string
	verbose    bool
	unhealthy  bool
)

func SetVersion(v string) {
	version = v
}

func IsUnhealthy() bool {
	return unhealthy
}

func setUnhealthy() {
	unhealthy = true
}

var rootCmd = &cobra.Command{
	Use:   "pingolin",
	Short: "Internet connection health monitor",
	Long:  "pingolin — Internet connection health monitor with historical tracking.",
	RunE:  runTUI,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/pingolin/config.toml)")
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "database path (default: ~/.local/share/pingolin/pingolin.db)")
	rootCmd.PersistentFlags().StringVar(&pingInt, "ping-interval", "", "ICMP probe interval (default: 5s)")
	rootCmd.PersistentFlags().StringVar(&dnsInt, "dns-interval", "", "DNS probe interval (default: 30s)")
	rootCmd.PersistentFlags().StringVar(&httpInt, "http-interval", "", "HTTP probe interval (default: 30s)")
	rootCmd.PersistentFlags().StringVar(&targets, "targets", "", "comma-separated ICMP targets")
	rootCmd.PersistentFlags().StringVar(&retention, "retention", "", "data retention period (default: 30d)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable debug logging")

	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(webCmd)
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, err
	}
	applyFlags(cfg)
	return cfg, nil
}

func applyFlags(cfg *config.Config) {
	if dbPath != "" {
		cfg.Storage.Path = dbPath
	}
	if targets != "" {
		cfg.Targets.ICMP = splitTargets(targets)
	}
	if pingInt != "" {
		if d, err := config.ParseDuration(pingInt); err == nil {
			cfg.Intervals.ICMP = config.Duration{Duration: d}
		}
	}
	if dnsInt != "" {
		if d, err := config.ParseDuration(dnsInt); err == nil {
			cfg.Intervals.DNS = config.Duration{Duration: d}
		}
	}
	if httpInt != "" {
		if d, err := config.ParseDuration(httpInt); err == nil {
			cfg.Intervals.HTTP = config.Duration{Duration: d}
		}
	}
	if retention != "" {
		if d, err := config.ParseDuration(retention); err == nil {
			cfg.Storage.Retention = config.Duration{Duration: d}
		}
	}
	cfg.Verbose = verbose
}

func splitTargets(s string) []string {
	var result []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

func runTUI(cmd *cobra.Command, args []string) error {
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

	od := outage.NewDetector(s, cfg.Targets.ICMP, cfg.Outage.ConsecutiveFailures)
	p := prober.New(s, cfg, od)
	p.Verbose = cfg.Verbose

	// Start retention cleanup
	stopRetention := make(chan struct{})
	go s.StartRetentionCleanup(cfg.Storage.Retention.Duration, stopRetention)

	// Check if daemon is already running; if not, start embedded prober
	if !isDaemonRunning() {
		p.Start()
		defer p.Stop()
	}

	model := tui.NewModel(s, cfg.Targets.ICMP, cfg.TUI.DefaultTimeRange, version)
	program := tea.NewProgram(model, tea.WithAltScreen())

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		program.Quit()
	}()

	if _, err := program.Run(); err != nil {
		close(stopRetention)
		return fmt.Errorf("running TUI: %w", err)
	}

	close(stopRetention)
	return nil
}

func isDaemonRunning() bool {
	pidPath := config.DefaultPIDPath()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Check if the process is actually running
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// waitForDaemon waits briefly for the daemon to start and write its PID file.
// This avoids a race when both services start at the same time via systemd.
func waitForDaemon() bool {
	if isDaemonRunning() {
		return true
	}
	for range 5 {
		time.Sleep(500 * time.Millisecond)
		if isDaemonRunning() {
			return true
		}
	}
	return false
}
