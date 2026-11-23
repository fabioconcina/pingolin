package outage

import (
	"log"
	"sync"
	"time"

	"github.com/fabioconcina/pingolin/internal/store"
)

// Outage causes stored in the outages table.
const (
	CauseConnectionDown = "connection_down"
	CauseICMPBlocked    = "icmp_blocked"
	CauseUpstreamDNS    = "upstream_dns"
)

// reclassifyInterval limits how often an open outage's cause is recomputed.
const reclassifyInterval = time.Minute

type Detector struct {
	mu                  sync.Mutex
	store               *store.Store
	targets             []string
	consecutiveFailures map[string]int
	threshold           int
	lastClassified      time.Time
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
		d.maybeReclassify(open)
		return
	}

	// No DNS or HTTP result from inside the outage exists yet, so start with
	// the most likely cause and refine it as evidence comes in.
	cause := CauseConnectionDown
	now := time.Now()
	if _, err := d.store.InsertOutage(store.Outage{
		StartedAt: now.UnixMilli(),
		Cause:     cause,
	}); err != nil {
		log.Printf("outage: error inserting outage: %v", err)
	} else {
		d.lastClassified = now
		log.Printf("outage: STARTED (cause: %s)", cause)
	}
}

// maybeReclassify updates the cause of an open outage, at most once per
// reclassifyInterval, so status and the dashboards show a cause based on
// what the DNS and HTTP probes saw during the outage.
func (d *Detector) maybeReclassify(open *store.Outage) {
	if time.Since(d.lastClassified) < reclassifyInterval {
		return
	}
	d.lastClassified = time.Now()
	d.updateCause(open, d.lastClassified.UnixMilli())
}

func (d *Detector) updateCause(o *store.Outage, until int64) {
	cause, err := d.classifyCause(o.StartedAt, until)
	if err != nil {
		log.Printf("outage: error classifying cause: %v", err)
		return
	}
	if cause == o.Cause {
		return
	}
	if err := d.store.UpdateOutageCause(o.ID, cause); err != nil {
		log.Printf("outage: %v", err)
		return
	}
	log.Printf("outage: cause %s -> %s", o.Cause, cause)
	o.Cause = cause
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
	d.updateCause(open, now)
	if err := d.store.CloseOutage(open.ID, now); err != nil {
		log.Printf("outage: error closing outage: %v", err)
	} else {
		duration := time.Duration(now-open.StartedAt) * time.Millisecond
		log.Printf("outage: ENDED (duration: %s, cause: %s)", duration, open.Cause)
	}
}

// classifyCause decides why all ICMP targets failed, using only the DNS and
// HTTP results recorded between since and until. Results from before the
// outage say nothing about it. The system resolver is ignored because a
// local cache (Pi-hole, systemd-resolved) keeps answering with stale records
// while the uplink is down.
func (d *Detector) classifyCause(since, until int64) (string, error) {
	httpResults, err := d.store.QueryHTTP(since, until)
	if err != nil {
		return "", err
	}
	dnsResults, err := d.store.QueryDNS(since, until)
	if err != nil {
		return "", err
	}

	var httpOK, dnsOK, dnsTotal int
	for _, r := range httpResults {
		if r.Success {
			httpOK++
		}
	}
	for _, r := range dnsResults {
		if r.Resolver == "system" {
			continue
		}
		dnsTotal++
		if r.Success {
			dnsOK++
		}
	}
	httpTotal := len(httpResults)

	switch {
	case httpTotal > 0 && 2*httpOK > httpTotal:
		// The web is reachable, so the connection is up and only ICMP fails.
		if dnsTotal > 0 && dnsOK == 0 {
			return CauseUpstreamDNS, nil
		}
		return CauseICMPBlocked, nil
	case httpTotal > 0:
		return CauseConnectionDown, nil
	case dnsTotal > 0 && 2*dnsOK > dnsTotal:
		return CauseICMPBlocked, nil
	default:
		return CauseConnectionDown, nil
	}
}
