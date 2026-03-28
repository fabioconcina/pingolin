package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Targets   TargetsConfig   `toml:"targets"`
	Intervals IntervalsConfig `toml:"intervals"`
	Storage   StorageConfig   `toml:"storage"`
	Outage    OutageConfig    `toml:"outage"`
	TUI       TUIConfig       `toml:"tui"`
	Web       WebConfig       `toml:"web"`
	Verbose   bool            `toml:"-"`
}

type TargetsConfig struct {
	ICMP         []string `toml:"icmp"`
	DNSQuery     string   `toml:"dns_query"`
	DNSResolvers []string `toml:"dns_resolvers"`
	HTTP         string   `toml:"http"`
}

type IntervalsConfig struct {
	ICMP Duration `toml:"icmp"`
	DNS  Duration `toml:"dns"`
	HTTP Duration `toml:"http"`
}

type StorageConfig struct {
	Path      string   `toml:"path"`
	Retention Duration `toml:"retention"`
}

type OutageConfig struct {
	ConsecutiveFailures int `toml:"consecutive_failures"`
}

type TUIConfig struct {
	DefaultTimeRange string `toml:"default_timerange"`
}

type WebConfig struct {
	Listen string `toml:"listen"`
	Port   int    `toml:"port"`
}

// Duration wraps time.Duration for TOML parsing.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = ParseDuration(string(text))
	return err
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

// ParseDuration parses durations like "30s", "5m", "24h", or "30d".
func ParseDuration(s string) (time.Duration, error) {
	if len(s) > 1 && s[len(s)-1] == 'd' {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

// FormatDuration formats a duration in a human-readable short form.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

func DefaultConfig() *Config {
	return &Config{
		Targets: TargetsConfig{
			ICMP:         []string{"1.1.1.1", "8.8.8.8", "9.9.9.9"},
			DNSQuery:     "google.com",
			DNSResolvers: []string{"system", "1.1.1.1:53"},
			HTTP:         "https://clients3.google.com/generate_204",
		},
		Intervals: IntervalsConfig{
			ICMP: Duration{5 * time.Second},
			DNS:  Duration{30 * time.Second},
			HTTP: Duration{30 * time.Second},
		},
		Storage: StorageConfig{
			Path:      defaultDBPath(),
			Retention: Duration{30 * 24 * time.Hour},
		},
		Outage: OutageConfig{
			ConsecutiveFailures: 3,
		},
		TUI: TUIConfig{
			DefaultTimeRange: "1h",
		},
		Web: WebConfig{
			Listen: "0.0.0.0",
			Port:   8080,
		},
	}
}

func dataDir() string {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "pingolin")
}

func defaultDBPath() string {
	return filepath.Join(dataDir(), "pingolin.db")
}

func DefaultPIDPath() string {
	return filepath.Join(dataDir(), "pingolin.pid")
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		path = defaultConfigPath()
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}

func defaultConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "pingolin", "config.toml")
}
