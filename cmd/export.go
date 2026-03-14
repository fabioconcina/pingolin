package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/fabioconcina/pingolin/internal/store"
)

var (
	exportFormat string
	exportLast   string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export raw data to CSV or JSON",
	RunE:  runExport,
}

func init() {
	exportCmd.Flags().StringVar(&exportFormat, "format", "csv", "output format: csv or json")
	exportCmd.Flags().StringVar(&exportLast, "last", "7d", "time window (e.g., 1h, 24h, 7d)")
}

func runExport(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	s, err := store.New(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	duration, err := parseWindowDuration(exportLast)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", exportLast, err)
	}

	now := time.Now()
	since := now.Add(-duration).UnixMilli()
	until := now.UnixMilli()

	switch exportFormat {
	case "csv":
		return exportCSV(s, since, until)
	case "json":
		return exportJSON(s, since, until)
	default:
		return fmt.Errorf("unsupported format: %s (use csv or json)", exportFormat)
	}
}

type exportData struct {
	Pings   []store.PingResult `json:"pings"`
	DNS     []store.DNSResult  `json:"dns"`
	HTTP    []store.HTTPResult `json:"http"`
	Outages []store.Outage     `json:"outages"`
}

func gatherData(s *store.Store, since, until int64) (*exportData, error) {
	pings, err := s.QueryAllPings(since, until)
	if err != nil {
		return nil, fmt.Errorf("querying pings: %w", err)
	}
	dns, err := s.QueryDNS(since, until)
	if err != nil {
		return nil, fmt.Errorf("querying DNS: %w", err)
	}
	http, err := s.QueryHTTP(since, until)
	if err != nil {
		return nil, fmt.Errorf("querying HTTP: %w", err)
	}
	outages, err := s.QueryOutages(since, until, 1000)
	if err != nil {
		return nil, fmt.Errorf("querying outages: %w", err)
	}
	return &exportData{
		Pings:   pings,
		DNS:     dns,
		HTTP:    http,
		Outages: outages,
	}, nil
}

func exportJSON(s *store.Store, since, until int64) error {
	data, err := gatherData(s, since, until)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func exportCSV(s *store.Store, since, until int64) error {
	data, err := gatherData(s, since, until)
	if err != nil {
		return err
	}

	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	// Ping results
	w.Write([]string{"# PING RESULTS"})
	w.Write([]string{"timestamp", "target", "rtt_ms", "packet_loss", "jitter_ms", "probe_type"})
	for _, p := range data.Pings {
		rtt := ""
		if p.RTTMs != nil {
			rtt = fmt.Sprintf("%.3f", *p.RTTMs)
		}
		jitter := ""
		if p.JitterMs != nil {
			jitter = fmt.Sprintf("%.3f", *p.JitterMs)
		}
		loss := "0"
		if p.PacketLoss {
			loss = "1"
		}
		w.Write([]string{
			time.UnixMilli(p.Timestamp).Format(time.RFC3339),
			p.Target, rtt, loss, jitter, p.ProbeType,
		})
	}

	w.Write([]string{})
	w.Write([]string{"# DNS RESULTS"})
	w.Write([]string{"timestamp", "query", "resolver", "resolve_ms", "success", "resolved_ips"})
	for _, d := range data.DNS {
		resolveMs := ""
		if d.ResolveMs != nil {
			resolveMs = fmt.Sprintf("%.3f", *d.ResolveMs)
		}
		success := "0"
		if d.Success {
			success = "1"
		}
		w.Write([]string{
			time.UnixMilli(d.Timestamp).Format(time.RFC3339),
			d.Query, d.Resolver, resolveMs, success, d.ResolvedIPs,
		})
	}

	w.Write([]string{})
	w.Write([]string{"# HTTP RESULTS"})
	w.Write([]string{"timestamp", "target", "total_ms", "tls_ms", "status_code", "success"})
	for _, h := range data.HTTP {
		totalMs := ""
		if h.TotalMs != nil {
			totalMs = fmt.Sprintf("%.3f", *h.TotalMs)
		}
		tlsMs := ""
		if h.TLSMs != nil {
			tlsMs = fmt.Sprintf("%.3f", *h.TLSMs)
		}
		statusCode := ""
		if h.StatusCode != nil {
			statusCode = fmt.Sprintf("%d", *h.StatusCode)
		}
		success := "0"
		if h.Success {
			success = "1"
		}
		w.Write([]string{
			time.UnixMilli(h.Timestamp).Format(time.RFC3339),
			h.Target, totalMs, tlsMs, statusCode, success,
		})
	}

	return nil
}
