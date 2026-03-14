package prober

import (
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/fabioconcina/pingolin/internal/store"
)

func (p *Prober) runHTTP(target string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.probeHTTP(target)
		case <-p.stop:
			return
		}
	}
}

func (p *Prober) probeHTTP(target string) {
	now := time.Now()

	var tlsStart, tlsEnd time.Time
	trace := &httptrace.ClientTrace{
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) { tlsEnd = time.Now() },
	}

	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		log.Printf("http: error creating request: %v", err)
		return
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)

	elapsed := time.Since(now)
	totalMs := float64(elapsed.Microseconds()) / 1000.0
	ts := now.UnixMilli()

	result := store.HTTPResult{
		Timestamp: ts,
		Target:    target,
		TotalMs:   &totalMs,
	}

	if err != nil {
		result.Success = false
		if storeErr := p.store.InsertHTTP(result); storeErr != nil {
			log.Printf("http: error storing result: %v", storeErr)
		}
		if p.Verbose {
			log.Printf("http: %s failed: %v", target, err)
		}
		return
	}
	defer resp.Body.Close()

	result.Success = resp.StatusCode >= 200 && resp.StatusCode < 400
	statusCode := resp.StatusCode
	result.StatusCode = &statusCode

	if !tlsStart.IsZero() && !tlsEnd.IsZero() {
		tlsMs := float64(tlsEnd.Sub(tlsStart).Microseconds()) / 1000.0
		result.TLSMs = &tlsMs
	}

	if err := p.store.InsertHTTP(result); err != nil {
		log.Printf("http: error storing result: %v", err)
	}

	if p.Verbose {
		log.Printf("http: %s status=%d total=%.1fms", target, resp.StatusCode, totalMs)
	}
}
