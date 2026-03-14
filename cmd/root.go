package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

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
	cfg.Verbose = verbose
}

func splitTargets(s string) []string {
	var result []string
	for _, t := range splitComma(s) {
		t = trimSpace(t)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

func splitComma(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == ',' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
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
