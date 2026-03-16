package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/fabioconcina/pingolin/internal/store"
)

type connectionStatus struct {
	Status  string         `json:"status"`
	Targets []targetStatus `json:"targets"`
	DNS     *dnsStatus     `json:"dns,omitempty"`
	HTTP    *httpStatus    `json:"http,omitempty"`
	Outages []outageInfo   `json:"recent_outages"`
}

type targetStatus struct {
	Target    string   `json:"target"`
	LastRTTMs *float64 `json:"last_rtt_ms"`
	AvgRTTMs  float64  `json:"avg_rtt_ms"`
	LossRate  float64  `json:"loss_rate_pct"`
	Samples   int      `json:"samples"`
}

type dnsStatus struct {
	Query     string   `json:"query"`
	Resolver  string   `json:"resolver"`
	ResolveMs *float64 `json:"resolve_ms"`
	Success   bool     `json:"success"`
}

type httpStatus struct {
	Target     string   `json:"target"`
	TotalMs    *float64 `json:"total_ms"`
	StatusCode *int     `json:"status_code"`
	Success    bool     `json:"success"`
}

type outageInfo struct {
	StartedAt string `json:"started_at"`
	Duration  string `json:"duration"`
	Cause     string `json:"cause"`
}

// NewServer creates a configured MCP server without starting it.
func NewServer(s *store.Store, version string, targets []string) *server.MCPServer {
	srv := server.NewMCPServer(
		"pingolin",
		version,
		server.WithToolCapabilities(true),
	)

	tool := mcp.NewTool("check_connection",
		mcp.WithDescription(
			"Check internet connection health. Returns current status of ICMP, DNS, and HTTP probes plus recent outages.",
		),
	)

	srv.AddTool(tool, makeHandler(s, targets))

	return srv
}

// Run starts an MCP server on stdio, blocking until stdin closes.
func Run(s *store.Store, version string, targets []string) error {
	return server.ServeStdio(NewServer(s, version, targets))
}

func makeHandler(s *store.Store, targets []string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		status := gatherStatus(s, targets)
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return toolError(fmt.Sprintf("JSON encoding failed: %v", err)), nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(data),
				},
			},
		}, nil
	}
}

func toolError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: msg,
			},
		},
		IsError: true,
	}
}

func gatherStatus(s *store.Store, targets []string) *connectionStatus {
	since := time.Now().Add(-1 * time.Hour).UnixMilli()

	overall := "healthy"
	cs := &connectionStatus{
		Targets: make([]targetStatus, 0, len(targets)),
		Outages: make([]outageInfo, 0),
	}

	// Check for open outage
	if open, _ := s.OpenOutage(); open != nil {
		overall = "down"
	}

	// ICMP targets
	for _, target := range targets {
		ts := targetStatus{Target: target}
		if latest, err := s.LatestPing(target); err == nil && latest != nil {
			ts.LastRTTMs = latest.RTTMs
		}
		avg, count, lossCount, err := s.PingStats(target, since)
		if err == nil && count > 0 {
			ts.AvgRTTMs = avg
			ts.LossRate = float64(lossCount) / float64(count) * 100
			ts.Samples = count
		}
		cs.Targets = append(cs.Targets, ts)
	}

	// DNS
	if dns, err := s.LatestDNS(); err == nil && dns != nil {
		cs.DNS = &dnsStatus{
			Query:     dns.Query,
			Resolver:  dns.Resolver,
			ResolveMs: dns.ResolveMs,
			Success:   dns.Success,
		}
		if !dns.Success {
			overall = "degraded"
		}
	}

	// HTTP
	if http, err := s.LatestHTTP(); err == nil && http != nil {
		cs.HTTP = &httpStatus{
			Target:     http.Target,
			TotalMs:    http.TotalMs,
			StatusCode: http.StatusCode,
			Success:    http.Success,
		}
		if !http.Success {
			overall = "degraded"
		}
	}

	// Recent outages
	if outages, err := s.RecentOutages(5); err == nil {
		for _, o := range outages {
			dur := "ongoing"
			if o.DurationMs != nil {
				d := time.Duration(*o.DurationMs) * time.Millisecond
				dur = d.Truncate(time.Second).String()
			}
			cs.Outages = append(cs.Outages, outageInfo{
				StartedAt: time.UnixMilli(o.StartedAt).Format(time.RFC3339),
				Duration:  dur,
				Cause:     o.Cause,
			})
		}
	}

	cs.Status = overall
	return cs
}
