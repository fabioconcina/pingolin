package mcpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fabioconcina/pingolin/internal/store"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type connectionStatus struct {
	Status  string         `json:"status"`
	Targets []targetStatus `json:"targets"`
	DNS     *dnsStatus     `json:"dns,omitempty"`
	HTTP    *httpStatus    `json:"http,omitempty"`
	Outages []outageInfo   `json:"recent_outages"`
}

type targetStatus struct {
	Target     string   `json:"target"`
	LastRTTMs  *float64 `json:"last_rtt_ms"`
	AvgRTTMs   float64  `json:"avg_rtt_ms"`
	LossRate   float64  `json:"loss_rate_pct"`
	Samples    int      `json:"samples"`
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
	StartedAt  string  `json:"started_at"`
	Duration   string  `json:"duration"`
	Cause      string  `json:"cause"`
}

// Run starts an MCP server on stdio, blocking until stdin closes.
func Run(s *store.Store, version string, targets []string) error {
	scanner := bufio.NewScanner(os.Stdin)
	out := os.Stdout

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeResponse(out, response{JSONRPC: "2.0", ID: nil, Error: &rpcError{Code: -32700, Message: "Parse error"}})
			continue
		}

		var id interface{}
		if req.ID != nil {
			json.Unmarshal(req.ID, &id)
		}

		switch req.Method {
		case "initialize":
			writeResponse(out, response{
				JSONRPC: "2.0",
				ID:      id,
				Result: map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
					"serverInfo":      map[string]interface{}{"name": "pingolin", "version": version},
				},
			})
		case "notifications/initialized":
			// notification, no response
		case "tools/list":
			writeResponse(out, response{
				JSONRPC: "2.0",
				ID:      id,
				Result: map[string]interface{}{
					"tools": []interface{}{
						map[string]interface{}{
							"name":        "check_connection",
							"description": "Check internet connection health. Returns current status of ICMP, DNS, and HTTP probes plus recent outages.",
							"inputSchema": map[string]interface{}{
								"type":                 "object",
								"properties":           map[string]interface{}{},
								"additionalProperties": false,
							},
						},
					},
				},
			})
		case "tools/call":
			handleToolsCall(out, id, req.Params, s, targets)
		case "ping":
			writeResponse(out, response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{}})
		default:
			writeResponse(out, response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}})
		}
	}

	return scanner.Err()
}

func handleToolsCall(out io.Writer, id interface{}, params json.RawMessage, s *store.Store, targets []string) {
	var p struct {
		Name string `json:"name"`
	}
	if params != nil {
		json.Unmarshal(params, &p)
	}

	if p.Name != "check_connection" {
		writeResponse(out, response{
			JSONRPC: "2.0",
			ID:      id,
			Result: map[string]interface{}{
				"content": []interface{}{map[string]interface{}{"type": "text", "text": fmt.Sprintf("Unknown tool: %s", p.Name)}},
				"isError": true,
			},
		})
		return
	}

	status := gatherStatus(s, targets)
	data, _ := json.MarshalIndent(status, "", "  ")

	writeResponse(out, response{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []interface{}{map[string]interface{}{"type": "text", "text": string(data)}},
			"isError": false,
		},
	})
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

func writeResponse(out io.Writer, resp response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	fmt.Fprintf(out, "%s\n", data)
}
