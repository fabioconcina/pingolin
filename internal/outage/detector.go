package outage

import (
	"log"
	"sync"
	"time"

	"github.com/fabioconcina/pingolin/internal/store"
)

type Detector struct {
	mu                  sync.Mutex
	store               *store.Store
	targets             []string
	consecutiveFailures map[string]int
	threshold           int
}

func NewDetector(s *store.Store, targets []string, threshold int) *Detector {
	failures := make(map[string]int, len(targets))
	for _, t := range targets {
		failures[t] = 0
	}
	d := &Detector{
		store:               s,
		targets:             targets,
		consecutiveFailures: failures,
		threshold:           threshold,
	}
	// Close any stale outage left from a previous run
	d.maybeCloseOutage()
	return d
}

func (d *Detector) RecordFailure(target string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.consecutiveFailures[target]++

	if d.allAboveThreshold() {
		d.maybeOpenOutage()
	}
}

func (d *Detector) RecordSuccess(target string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	wasAbove := d.consecutiveFailures[target] >= d.threshold
	d.consecutiveFailures[target] = 0

	if wasAbove {
		d.maybeCloseOutage()
	}
}

func (d *Detector) allAboveThreshold() bool {
	for _, count := range d.consecutiveFailures {
		if count < d.threshold {
			return false
		}
	}
	return true
}

func (d *Detector) maybeOpenOutage() {
	open, err := d.store.OpenOutage()
	if err != nil {
		log.Printf("outage: error checking open outage: %v", err)
		return
	}
	if open != nil {
		return // already have an open outage
	}

	cause := d.classifyCause()
	now := time.Now().UnixMilli()
	if _, err := d.store.InsertOutage(store.Outage{
		StartedAt: now,
		Cause:     cause,
	}); err != nil {
		log.Printf("outage: error inserting outage: %v", err)
	} else {
		log.Printf("outage: STARTED (cause: %s)", cause)
	}
}

func (d *Detector) maybeCloseOutage() {
	open, err := d.store.OpenOutage()
	if err != nil {
		log.Printf("outage: error checking open outage: %v", err)
		return
	}
	if open == nil {
		return
	}

	now := time.Now().UnixMilli()
	if err := d.store.CloseOutage(open.ID, now); err != nil {
		log.Printf("outage: error closing outage: %v", err)
	} else {
		duration := time.Duration(now-open.StartedAt) * time.Millisecond
		log.Printf("outage: ENDED (duration: %s)", duration)
	}
}

func (d *Detector) classifyCause() string {
	// Check latest DNS and HTTP results to classify
	dns, err := d.store.LatestDNS()
	if err != nil {
		return "connection_down"
	}
	http, err := d.store.LatestHTTP()
	if err != nil {
		return "connection_down"
	}

	icmpFailing := d.allAboveThreshold()

	if icmpFailing && dns != nil && dns.Success {
		return "icmp_blocked"
	}
	if icmpFailing && (dns == nil || !dns.Success) && http != nil && http.Success {
		// HTTP worked recently but DNS failed
		recentThreshold := time.Now().Add(-2 * time.Minute).UnixMilli()
		if http.Timestamp > recentThreshold {
			return "upstream_dns"
		}
	}
	return "connection_down"
}
