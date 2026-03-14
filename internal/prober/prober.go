package prober

import (
	"github.com/fabioconcina/pingolin/internal/config"
	"github.com/fabioconcina/pingolin/internal/outage"
	"github.com/fabioconcina/pingolin/internal/store"
)

type Prober struct {
	store           *store.Store
	cfg             *config.Config
	outageDetector  *outage.Detector
	jitterCalcs     map[string]*JitterCalculator
	stop            chan struct{}
	Verbose         bool
}

func New(s *store.Store, cfg *config.Config, od *outage.Detector) *Prober {
	jc := make(map[string]*JitterCalculator, len(cfg.Targets.ICMP))
	for _, target := range cfg.Targets.ICMP {
		jc[target] = NewJitterCalculator()
	}
	return &Prober{
		store:          s,
		cfg:            cfg,
		outageDetector: od,
		jitterCalcs:    jc,
		stop:           make(chan struct{}),
	}
}

func (p *Prober) Start() {
	// Start ICMP probers for each target
	for _, target := range p.cfg.Targets.ICMP {
		go p.runICMP(target, p.cfg.Intervals.ICMP.Duration, p.jitterCalcs[target])
	}

	// Start DNS prober
	go p.runDNS(p.cfg.Targets.DNSQuery, p.cfg.Targets.DNSResolvers, p.cfg.Intervals.DNS.Duration)

	// Start HTTP prober
	go p.runHTTP(p.cfg.Targets.HTTP, p.cfg.Intervals.HTTP.Duration)
}

func (p *Prober) Stop() {
	close(p.stop)
}
