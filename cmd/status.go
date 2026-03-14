package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/fabioconcina/pingolin/internal/store"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print current connection status and exit",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	s, err := store.New(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	since := time.Now().Add(-1 * time.Hour).UnixMilli()

	// Overall status
	open, _ := s.OpenOutage()
	unhealthy := open != nil
	if unhealthy {
		fmt.Println("Status: DOWN")
		fmt.Printf("  Outage started: %s\n", time.UnixMilli(open.StartedAt).Format(time.DateTime))
		fmt.Printf("  Cause: %s\n", open.Cause)
	} else {
		fmt.Println("Status: HEALTHY")
	}
	fmt.Println()

	// ICMP results
	fmt.Println("ICMP Ping:")
	for _, target := range cfg.Targets.ICMP {
		latest, err := s.LatestPing(target)
		if err != nil || latest == nil {
			fmt.Printf("  %s: no data\n", target)
			continue
		}
		avg, count, lossCount, _ := s.PingStats(target, since)
		rttStr := "timeout"
		if latest.RTTMs != nil {
			rttStr = fmt.Sprintf("%.1fms", *latest.RTTMs)
		}
		lossRate := 0.0
		if count > 0 {
			lossRate = float64(lossCount) / float64(count) * 100
		}
		fmt.Printf("  %s: last=%s avg=%.1fms loss=%.1f%% (%d samples)\n",
			target, rttStr, avg, lossRate, count)
	}
	fmt.Println()

	// DNS
	fmt.Println("DNS Resolution:")
	dns, err := s.LatestDNS()
	if err != nil || dns == nil {
		fmt.Println("  no data")
	} else {
		status := "OK"
		if !dns.Success {
			status = "FAILED"
		}
		resolveStr := "--"
		if dns.ResolveMs != nil {
			resolveStr = fmt.Sprintf("%.0fms", *dns.ResolveMs)
		}
		fmt.Printf("  %s via %s: %s (%s)\n", dns.Query, dns.Resolver, resolveStr, status)
	}
	fmt.Println()

	// HTTP
	fmt.Println("HTTP Probe:")
	http, err := s.LatestHTTP()
	if err != nil || http == nil {
		fmt.Println("  no data")
	} else {
		status := "OK"
		if !http.Success {
			status = "FAILED"
		}
		totalStr := "--"
		if http.TotalMs != nil {
			totalStr = fmt.Sprintf("%.0fms", *http.TotalMs)
		}
		statusCode := "--"
		if http.StatusCode != nil {
			statusCode = fmt.Sprintf("%d", *http.StatusCode)
		}
		fmt.Printf("  %s: %s status=%s (%s)\n", http.Target, totalStr, statusCode, status)
	}

	if unhealthy {
		setUnhealthy()
		return fmt.Errorf("connection unhealthy")
	}

	return nil
}
