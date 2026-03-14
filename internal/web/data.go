package web

import (
	"fmt"
	"time"

	"github.com/fabioconcina/pingolin/internal/store"
)

type DashboardData struct {
	Status    string       `json:"status"`
	Targets   []TargetData `json:"targets"`
	DNS       *DNSData     `json:"dns"`
	HTTP      *HTTPData    `json:"http"`
	Outages   []OutageData `json:"outages"`
	TimeRange string       `json:"time_range"`
	UpdatedAt int64        `json:"updated_at"`
}

type TargetData struct {
	Target      string    `json:"target"`
	LastRTT     *float64  `json:"last_rtt"`
	AvgRTT      float64   `json:"avg_rtt"`
	LossPercent float64   `json:"loss_pct"`
	Jitter      *float64  `json:"jitter"`
	Sparkline   []float64 `json:"sparkline"`
}

type DNSData struct {
	Resolver  string   `json:"resolver"`
	LastMs    *float64 `json:"last_ms"`
	AvgMs     float64  `json:"avg_ms"`
	Success   bool     `json:"success"`
}

type HTTPData struct {
	Target     string   `json:"target"`
	LastMs     *float64 `json:"last_ms"`
	TLSMs      *float64 `json:"tls_ms"`
	StatusCode *int     `json:"status_code"`
	Success    bool     `json:"success"`
}

type OutageData struct {
	StartedAt  string  `json:"started_at"`
	Duration   string  `json:"duration"`
	Cause      string  `json:"cause"`
}

var timeRanges = map[string]time.Duration{
	"1h":  1 * time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

func FetchDashboardData(s *store.Store, targets []string, timeRange string) DashboardData {
	dur, ok := timeRanges[timeRange]
	if !ok {
		dur = 1 * time.Hour
		timeRange = "1h"
	}

	now := time.Now()
	since := now.Add(-dur).UnixMilli()
	until := now.UnixMilli()

	data := DashboardData{
		Status:    "healthy",
		TimeRange: timeRange,
		UpdatedAt: now.UnixMilli(),
	}

	// Per-target ICMP data
	for _, target := range targets {
		td := TargetData{Target: target}

		if latest, err := s.LatestPing(target); err == nil && latest != nil {
			td.LastRTT = latest.RTTMs
			td.Jitter = latest.JitterMs
		}
		if avg, count, lossCount, err := s.PingStats(target, since); err == nil && count > 0 {
			td.AvgRTT = avg
			td.LossPercent = float64(lossCount) / float64(count) * 100
		}
		if pings, err := s.QueryPings(target, since, until); err == nil {
			td.Sparkline = downsampleRTTs(pings, 120)
		}

		data.Targets = append(data.Targets, td)
	}

	// DNS
	if dns, err := s.LatestDNS(); err == nil && dns != nil {
		dd := &DNSData{
			Resolver: dns.Resolver,
			LastMs:   dns.ResolveMs,
			Success:  dns.Success,
		}
		if avg, _, err := s.DNSStats(since); err == nil {
			dd.AvgMs = avg
		}
		data.DNS = dd
	}

	// HTTP
	if h, err := s.LatestHTTP(); err == nil && h != nil {
		data.HTTP = &HTTPData{
			Target:     h.Target,
			LastMs:     h.TotalMs,
			TLSMs:      h.TLSMs,
			StatusCode: h.StatusCode,
			Success:    h.Success,
		}
	}

	// Outages
	if outages, err := s.RecentOutages(10); err == nil {
		for _, o := range outages {
			od := OutageData{
				StartedAt: time.UnixMilli(o.StartedAt).Format("2006-01-02 15:04"),
				Cause:     o.Cause,
			}
			if o.DurationMs != nil {
				od.Duration = formatDuration(time.Duration(*o.DurationMs) * time.Millisecond)
			} else {
				od.Duration = "ongoing"
			}
			data.Outages = append(data.Outages, od)
		}
	}

	// Determine status (same logic as TUI)
	data.Status = determineStatus(s, targets, data)

	return data
}

func determineStatus(s *store.Store, targets []string, data DashboardData) string {
	if open, err := s.OpenOutage(); err == nil && open != nil {
		return "down"
	}

	for _, td := range data.Targets {
		if td.LossPercent > 5 {
			return "degraded"
		}
		if td.LastRTT != nil && td.AvgRTT > 0 && *td.LastRTT > 3*td.AvgRTT {
			return "degraded"
		}
	}

	if data.DNS != nil && !data.DNS.Success {
		return "degraded"
	}
	if data.HTTP != nil && !data.HTTP.Success {
		return "degraded"
	}

	return "healthy"
}

func downsampleRTTs(pings []store.PingResult, maxPoints int) []float64 {
	if len(pings) == 0 {
		return nil
	}
	if len(pings) <= maxPoints {
		values := make([]float64, len(pings))
		for i, p := range pings {
			if p.RTTMs != nil {
				values[i] = *p.RTTMs
			} else {
				values[i] = -1 // packet loss
			}
		}
		return values
	}

	// Bucket and take max RTT per bucket
	bucketSize := float64(len(pings)) / float64(maxPoints)
	values := make([]float64, maxPoints)
	for i := 0; i < maxPoints; i++ {
		start := int(float64(i) * bucketSize)
		end := int(float64(i+1) * bucketSize)
		if end > len(pings) {
			end = len(pings)
		}
		maxRTT := -1.0
		for j := start; j < end; j++ {
			if pings[j].RTTMs != nil && *pings[j].RTTMs > maxRTT {
				maxRTT = *pings[j].RTTMs
			}
		}
		values[i] = maxRTT
	}
	return values
}

func formatDuration(d time.Duration) string {
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
