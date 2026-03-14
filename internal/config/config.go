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

// Duration wraps time.Duration for TOML parsing.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = parseDuration(string(text))
	return err
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

func parseDuration(s string) (time.Duration, error) {
	// Support "30d" style durations
	if len(s) > 1 && s[len(s)-1] == 'd' {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
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
	}
}

func defaultDBPath() string {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "pingolin", "pingolin.db")
}

func DefaultPIDPath() string {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "pingolin", "pingolin.pid")
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
