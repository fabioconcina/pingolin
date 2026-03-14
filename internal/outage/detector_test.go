package outage

import (
	"path/filepath"
	"testing"

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
