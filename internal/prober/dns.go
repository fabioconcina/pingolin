package prober

import (
	"context"
	"log"
	"net"
	"strings"
	"time"

	"github.com/fabioconcina/pingolin/internal/store"
)

func (p *Prober) runDNS(query string, resolvers []string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, resolver := range resolvers {
				p.probeDNS(query, resolver)
			}
		case <-p.stop:
			return
		}
	}
}

func (p *Prober) probeDNS(query, resolver string) {
	now := time.Now()

	var ips []net.IP
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if resolver == "system" {
		ips, err = net.DefaultResolver.LookupIP(ctx, "ip", query)
	} else {
		r := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, "udp", resolver)
			},
		}
		ips, err = r.LookupIP(ctx, "ip", query)
	}

	elapsed := time.Since(now)
	resolveMs := float64(elapsed.Microseconds()) / 1000.0
	ts := now.UnixMilli()

	success := err == nil && len(ips) > 0
	var ipStrs []string
	for _, ip := range ips {
		ipStrs = append(ipStrs, ip.String())
	}

	result := store.DNSResult{
		Timestamp:   ts,
		Query:       query,
		Resolver:    resolver,
		Success:     success,
		ResolvedIPs: strings.Join(ipStrs, ","),
	}
	if success {
		result.ResolveMs = &resolveMs
	}

	if err := p.store.InsertDNS(result); err != nil {
		log.Printf("dns: error storing result: %v", err)
	}

	if p.Verbose {
		if success {
			log.Printf("dns: %s via %s resolved in %.1fms", query, resolver, resolveMs)
		} else {
			log.Printf("dns: %s via %s failed: %v", query, resolver, err)
		}
	}

}
