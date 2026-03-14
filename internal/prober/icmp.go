package prober

import (
	"log"
	"time"

	probing "github.com/prometheus-community/pro-bing"

	"github.com/fabioconcina/pingolin/internal/store"
)

func (p *Prober) runICMP(target string, interval time.Duration, jitterCalc *JitterCalculator) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.probeICMP(target, jitterCalc)
		case <-p.stop:
			return
		}
	}
}

func (p *Prober) probeICMP(target string, jitterCalc *JitterCalculator) {
	pinger, err := probing.NewPinger(target)
	if err != nil {
		log.Printf("icmp: error creating pinger for %s: %v", target, err)
		p.recordICMPFailure(target)
		return
	}
	pinger.Count = 1
	pinger.Timeout = 3 * time.Second
	pinger.SetPrivileged(true)

	if err := pinger.Run(); err != nil {
		log.Printf("icmp: error pinging %s: %v", target, err)
		p.recordICMPFailure(target)
		return
	}

	stats := pinger.Statistics()
	now := time.Now().UnixMilli()

	if stats.PacketsRecv == 0 {
		p.recordICMPFailure(target)
		result := store.PingResult{
			Timestamp:  now,
			Target:     target,
			PacketLoss: true,
			ProbeType:  "icmp",
		}
		if err := p.store.InsertPing(result); err != nil {
			log.Printf("icmp: error storing result for %s: %v", target, err)
		}
		return
	}

	rtt := float64(stats.AvgRtt.Microseconds()) / 1000.0
	jitter := jitterCalc.Add(rtt)

	result := store.PingResult{
		Timestamp:  now,
		Target:     target,
		RTTMs:      &rtt,
		PacketLoss: false,
		JitterMs:   jitter,
		ProbeType:  "icmp",
	}
	if err := p.store.InsertPing(result); err != nil {
		log.Printf("icmp: error storing result for %s: %v", target, err)
	}

	p.outageDetector.RecordSuccess(target)

	if p.Verbose {
		log.Printf("icmp: %s rtt=%.1fms", target, rtt)
	}
}

func (p *Prober) recordICMPFailure(target string) {
	p.outageDetector.RecordFailure(target)
}
