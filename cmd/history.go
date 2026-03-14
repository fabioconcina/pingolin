package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/fabioconcina/pingolin/internal/store"
)

var historyLast string

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Print summary stats for a time window",
	RunE:  runHistory,
}

func init() {
	historyCmd.Flags().StringVar(&historyLast, "last", "24h", "time window (e.g., 1h, 24h, 7d, 30d)")
}

func runHistory(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	s, err := store.New(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	duration, err := parseWindowDuration(historyLast)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", historyLast, err)
	}

	now := time.Now()
	since := now.Add(-duration).UnixMilli()
	until := now.UnixMilli()

	fmt.Printf("Pingolin History — last %s\n", historyLast)
	fmt.Println(repeatChar('─', 50))
	fmt.Println()

	// ICMP stats
	fmt.Println("ICMP Ping:")
	for _, target := range cfg.Targets.ICMP {
		avg, count, lossCount, err := s.PingStats(target, since)
		if err != nil || count == 0 {
			fmt.Printf("  %s: no data\n", target)
			continue
		}
		lossRate := float64(lossCount) / float64(count) * 100
		fmt.Printf("  %s: avg=%.1fms loss=%.1f%% samples=%d\n",
			target, avg, lossRate, count)
	}
	fmt.Println()

	// DNS stats
	fmt.Println("DNS Resolution:")
	avgDNS, dnsCount, err := s.DNSStats(since)
	if err != nil || dnsCount == 0 {
		fmt.Println("  no data")
	} else {
		fmt.Printf("  avg=%.0fms successful_queries=%d\n", avgDNS, dnsCount)
	}
	fmt.Println()

	// HTTP stats
	fmt.Println("HTTP Probe:")
	httpResults, err := s.QueryHTTP(since, until)
	if err != nil || len(httpResults) == 0 {
		fmt.Println("  no data")
	} else {
		var totalMs float64
		var successCount int
		for _, r := range httpResults {
			if r.Success && r.TotalMs != nil {
				totalMs += *r.TotalMs
				successCount++
			}
		}
		avgHTTP := 0.0
		if successCount > 0 {
			avgHTTP = totalMs / float64(successCount)
		}
		failRate := float64(len(httpResults)-successCount) / float64(len(httpResults)) * 100
		fmt.Printf("  avg=%.0fms fail=%.1f%% samples=%d\n", avgHTTP, failRate, len(httpResults))
	}
	fmt.Println()

	// Outages
	fmt.Println("Outages:")
	outages, err := s.QueryOutages(since, until, 50)
	if err != nil || len(outages) == 0 {
		fmt.Println("  none")
	} else {
		for _, o := range outages {
			ts := time.UnixMilli(o.StartedAt).Format("2006-01-02 15:04")
			dur := "ongoing"
			if o.DurationMs != nil {
				d := time.Duration(*o.DurationMs) * time.Millisecond
				dur = d.Truncate(time.Second).String()
			}
			fmt.Printf("  %s  duration: %s  cause: %s\n", ts, dur, o.Cause)
		}
	}

	return nil
}

func parseWindowDuration(s string) (time.Duration, error) {
	if len(s) > 1 && s[len(s)-1] == 'd' {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

func repeatChar(c rune, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(c)
	}
	return string(b)
}
