package outage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/fabioconcina/pingolin/internal/store"
)

func testDetector(t *testing.T) (*Detector, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	targets := []string{"1.1.1.1", "8.8.8.8"}
	d := NewDetector(s, targets, 3)
	return d, s
}

func TestNoOutageOnPartialFailure(t *testing.T) {
	d, s := testDetector(t)

	// Only one target fails
	for i := 0; i < 5; i++ {
		d.RecordFailure("1.1.1.1")
		d.RecordSuccess("8.8.8.8")
	}

	open, err := s.OpenOutage()
	if err != nil {
		t.Fatalf("open outage: %v", err)
	}
	if open != nil {
		t.Error("expected no outage when only one target fails")
	}
}

func TestOutageOnAllTargetsFailure(t *testing.T) {
	d, s := testDetector(t)

	for i := 0; i < 3; i++ {
		d.RecordFailure("1.1.1.1")
		d.RecordFailure("8.8.8.8")
	}

	open, err := s.OpenOutage()
	if err != nil {
		t.Fatalf("open outage: %v", err)
	}
	if open == nil {
		t.Fatal("expected outage when all targets fail")
	}
}

func TestOutageClosedOnRecovery(t *testing.T) {
	d, s := testDetector(t)

	// Trigger outage
	for i := 0; i < 3; i++ {
		d.RecordFailure("1.1.1.1")
		d.RecordFailure("8.8.8.8")
	}

	// Recovery
	d.RecordSuccess("1.1.1.1")

	open, err := s.OpenOutage()
	if err != nil {
		t.Fatalf("open outage: %v", err)
	}
	if open != nil {
		t.Error("expected outage to be closed after recovery")
	}
}

func TestNoDuplicateOutage(t *testing.T) {
	d, s := testDetector(t)

	// Trigger outage
	for i := 0; i < 5; i++ {
		d.RecordFailure("1.1.1.1")
		d.RecordFailure("8.8.8.8")
	}

	outages, err := s.RecentOutages(10)
	if err != nil {
		t.Fatalf("recent outages: %v", err)
	}
	if len(outages) != 1 {
		t.Errorf("expected exactly 1 outage, got %d", len(outages))
	}
}

func triggerOutage(t *testing.T, d *Detector, s *store.Store) *store.Outage {
	t.Helper()
	for i := 0; i < 3; i++ {
		d.RecordFailure("1.1.1.1")
		d.RecordFailure("8.8.8.8")
	}
	open, err := s.OpenOutage()
	if err != nil || open == nil {
		t.Fatalf("expected open outage, got %v (err %v)", open, err)
	}
	return open
}

func closedCause(t *testing.T, d *Detector, s *store.Store) string {
	t.Helper()
	d.RecordSuccess("1.1.1.1")
	outages, err := s.RecentOutages(1)
	if err != nil || len(outages) != 1 {
		t.Fatalf("expected one outage, got %d (err %v)", len(outages), err)
	}
	if outages[0].EndedAt == nil {
		t.Fatal("expected outage to be closed")
	}
	return outages[0].Cause
}

func insertDNS(t *testing.T, s *store.Store, ts int64, resolver string, ok bool) {
	t.Helper()
	if err := s.InsertDNS(store.DNSResult{Timestamp: ts, Query: "google.com", Resolver: resolver, Success: ok}); err != nil {
		t.Fatal(err)
	}
}

func insertHTTP(t *testing.T, s *store.Store, ts int64, ok bool) {
	t.Helper()
	if err := s.InsertHTTP(store.HTTPResult{Timestamp: ts, Target: "https://example.com", Success: ok}); err != nil {
		t.Fatal(err)
	}
}

// Regression: a DNS success from just before the outage, plus a stale
// answer from a local cache during it, used to label a full uplink loss
// as icmp_blocked.
func TestCauseConnectionDownDespiteCachedDNS(t *testing.T) {
	d, s := testDetector(t)
	open := triggerOutage(t, d, s)

	insertDNS(t, s, open.StartedAt-1000, "system", true)
	insertDNS(t, s, open.StartedAt-1000, "1.1.1.1:53", true)
	insertHTTP(t, s, open.StartedAt-1000, true)

	insertDNS(t, s, open.StartedAt, "system", true)
	insertDNS(t, s, open.StartedAt, "1.1.1.1:53", false)
	insertHTTP(t, s, open.StartedAt, false)

	if got := closedCause(t, d, s); got != CauseConnectionDown {
		t.Errorf("cause = %q, want %q", got, CauseConnectionDown)
	}
}

func TestCauseICMPBlockedWhenHTTPWorks(t *testing.T) {
	d, s := testDetector(t)
	open := triggerOutage(t, d, s)

	insertDNS(t, s, open.StartedAt, "1.1.1.1:53", true)
	insertHTTP(t, s, open.StartedAt, true)

	if got := closedCause(t, d, s); got != CauseICMPBlocked {
		t.Errorf("cause = %q, want %q", got, CauseICMPBlocked)
	}
}

func TestCauseUpstreamDNS(t *testing.T) {
	d, s := testDetector(t)
	open := triggerOutage(t, d, s)

	insertDNS(t, s, open.StartedAt, "1.1.1.1:53", false)
	insertHTTP(t, s, open.StartedAt, true)

	if got := closedCause(t, d, s); got != CauseUpstreamDNS {
		t.Errorf("cause = %q, want %q", got, CauseUpstreamDNS)
	}
}

func TestCauseDefaultsToConnectionDownWithoutEvidence(t *testing.T) {
	d, s := testDetector(t)
	open := triggerOutage(t, d, s)
	if open.Cause != CauseConnectionDown {
		t.Errorf("preliminary cause = %q, want %q", open.Cause, CauseConnectionDown)
	}
	if got := closedCause(t, d, s); got != CauseConnectionDown {
		t.Errorf("cause = %q, want %q", got, CauseConnectionDown)
	}
}

func TestOpenOutageReclassified(t *testing.T) {
	d, s := testDetector(t)
	open := triggerOutage(t, d, s)
	insertHTTP(t, s, open.StartedAt, true)

	d.lastClassified = time.Now().Add(-2 * reclassifyInterval)
	d.RecordFailure("1.1.1.1")

	open, err := s.OpenOutage()
	if err != nil || open == nil {
		t.Fatalf("expected open outage, got %v (err %v)", open, err)
	}
	if open.Cause != CauseICMPBlocked {
		t.Errorf("cause = %q, want %q", open.Cause, CauseICMPBlocked)
	}
}
