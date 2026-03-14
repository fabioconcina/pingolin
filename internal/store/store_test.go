package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestInsertAndQueryPing(t *testing.T) {
	s := testStore(t)
	now := time.Now().UnixMilli()
	rtt := 12.5

	err := s.InsertPing(PingResult{
		Timestamp:  now,
		Target:     "1.1.1.1",
		RTTMs:      &rtt,
		PacketLoss: false,
		ProbeType:  "icmp",
	})
	if err != nil {
		t.Fatalf("insert ping: %v", err)
	}

	results, err := s.QueryPings("1.1.1.1", now-1000, now+1000)
	if err != nil {
		t.Fatalf("query pings: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Target != "1.1.1.1" {
		t.Errorf("expected target 1.1.1.1, got %s", results[0].Target)
	}
	if results[0].RTTMs == nil || *results[0].RTTMs != 12.5 {
		t.Errorf("expected rtt 12.5, got %v", results[0].RTTMs)
	}
}

func TestInsertAndQueryDNS(t *testing.T) {
	s := testStore(t)
	now := time.Now().UnixMilli()
	resolveMs := 8.0

	err := s.InsertDNS(DNSResult{
		Timestamp:   now,
		Query:       "google.com",
		Resolver:    "system",
		ResolveMs:   &resolveMs,
		Success:     true,
		ResolvedIPs: "142.250.80.46",
	})
	if err != nil {
		t.Fatalf("insert dns: %v", err)
	}

	results, err := s.QueryDNS(now-1000, now+1000)
	if err != nil {
		t.Fatalf("query dns: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Error("expected success=true")
	}
}

func TestInsertAndQueryHTTP(t *testing.T) {
	s := testStore(t)
	now := time.Now().UnixMilli()
	totalMs := 45.0
	statusCode := 204

	err := s.InsertHTTP(HTTPResult{
		Timestamp:  now,
		Target:     "https://clients3.google.com/generate_204",
		TotalMs:    &totalMs,
		StatusCode: &statusCode,
		Success:    true,
	})
	if err != nil {
		t.Fatalf("insert http: %v", err)
	}

	results, err := s.QueryHTTP(now-1000, now+1000)
	if err != nil {
		t.Fatalf("query http: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Error("expected success=true")
	}
}

func TestOutageLifecycle(t *testing.T) {
	s := testStore(t)
	now := time.Now().UnixMilli()

	id, err := s.InsertOutage(Outage{
		StartedAt: now,
		Cause:     "connection_down",
	})
	if err != nil {
		t.Fatalf("insert outage: %v", err)
	}

	open, err := s.OpenOutage()
	if err != nil {
		t.Fatalf("open outage: %v", err)
	}
	if open == nil {
		t.Fatal("expected open outage")
	}
	if open.ID != id {
		t.Errorf("expected id %d, got %d", id, open.ID)
	}

	err = s.CloseOutage(id, now+60000)
	if err != nil {
		t.Fatalf("close outage: %v", err)
	}

	open, err = s.OpenOutage()
	if err != nil {
		t.Fatalf("open outage after close: %v", err)
	}
	if open != nil {
		t.Error("expected no open outage after close")
	}
}

func TestLatestPing(t *testing.T) {
	s := testStore(t)
	now := time.Now().UnixMilli()

	rtt1 := 10.0
	rtt2 := 20.0
	s.InsertPing(PingResult{Timestamp: now - 1000, Target: "1.1.1.1", RTTMs: &rtt1, ProbeType: "icmp"})
	s.InsertPing(PingResult{Timestamp: now, Target: "1.1.1.1", RTTMs: &rtt2, ProbeType: "icmp"})

	latest, err := s.LatestPing("1.1.1.1")
	if err != nil {
		t.Fatalf("latest ping: %v", err)
	}
	if latest == nil || latest.RTTMs == nil || *latest.RTTMs != 20.0 {
		t.Error("expected latest ping to have rtt=20.0")
	}
}

func TestDeleteOlderThan(t *testing.T) {
	s := testStore(t)
	old := time.Now().Add(-48 * time.Hour).UnixMilli()
	recent := time.Now().UnixMilli()

	rtt := 10.0
	s.InsertPing(PingResult{Timestamp: old, Target: "1.1.1.1", RTTMs: &rtt, ProbeType: "icmp"})
	s.InsertPing(PingResult{Timestamp: recent, Target: "1.1.1.1", RTTMs: &rtt, ProbeType: "icmp"})

	err := s.DeleteOlderThan(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("delete older: %v", err)
	}

	results, err := s.QueryPings("1.1.1.1", 0, time.Now().UnixMilli()+1000)
	if err != nil {
		t.Fatalf("query after delete: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result after cleanup, got %d", len(results))
	}
}

func TestPingStats(t *testing.T) {
	s := testStore(t)
	now := time.Now().UnixMilli()

	rtt1 := 10.0
	rtt2 := 20.0
	s.InsertPing(PingResult{Timestamp: now - 1000, Target: "1.1.1.1", RTTMs: &rtt1, ProbeType: "icmp"})
	s.InsertPing(PingResult{Timestamp: now, Target: "1.1.1.1", RTTMs: &rtt2, PacketLoss: true, ProbeType: "icmp"})

	avg, count, lossCount, err := s.PingStats("1.1.1.1", now-2000)
	if err != nil {
		t.Fatalf("ping stats: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}
	if lossCount != 1 {
		t.Errorf("expected lossCount=1, got %d", lossCount)
	}
	if avg != 15.0 {
		t.Errorf("expected avg=15.0, got %f", avg)
	}
}

func TestStoreCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sub", "dir", "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store with nested path: %v", err)
	}
	s.Close()

	if _, err := os.Stat(filepath.Dir(dbPath)); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}
