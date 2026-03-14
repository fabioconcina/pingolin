package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
	mu sync.Mutex
}

type PingResult struct {
	ID         int64
	Timestamp  int64
	Target     string
	RTTMs      *float64
	PacketLoss bool
	JitterMs   *float64
	ProbeType  string
}

type DNSResult struct {
	ID          int64
	Timestamp   int64
	Query       string
	Resolver    string
	ResolveMs   *float64
	Success     bool
	ResolvedIPs string
}

type HTTPResult struct {
	ID         int64
	Timestamp  int64
	Target     string
	TotalMs    *float64
	TLSMs      *float64
	StatusCode *int
	Success    bool
}

type Outage struct {
	ID         int64
	StartedAt  int64
	EndedAt    *int64
	DurationMs *int64
	Cause      string
}

func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode and set journal size limit
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA journal_size_limit=10485760",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("setting pragma %q: %w", pragma, err)
		}
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) InsertPing(result PingResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO ping_results (timestamp, target, rtt_ms, packet_loss, jitter_ms, probe_type)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		result.Timestamp, result.Target, result.RTTMs, boolToInt(result.PacketLoss), result.JitterMs, result.ProbeType,
	)
	return err
}

func (s *Store) InsertDNS(result DNSResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO dns_results (timestamp, query, resolver, resolve_ms, success, resolved_ips)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		result.Timestamp, result.Query, result.Resolver, result.ResolveMs, boolToInt(result.Success), result.ResolvedIPs,
	)
	return err
}

func (s *Store) InsertHTTP(result HTTPResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO http_results (timestamp, target, total_ms, tls_ms, status_code, success)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		result.Timestamp, result.Target, result.TotalMs, result.TLSMs, result.StatusCode, boolToInt(result.Success),
	)
	return err
}

func (s *Store) InsertOutage(outage Outage) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(
		`INSERT INTO outages (started_at, cause) VALUES (?, ?)`,
		outage.StartedAt, outage.Cause,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) CloseOutage(id int64, endedAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`UPDATE outages SET ended_at = ?, duration_ms = ? - started_at WHERE id = ?`,
		endedAt, endedAt, id,
	)
	return err
}

func (s *Store) OpenOutage() (*Outage, error) {
	row := s.db.QueryRow(`SELECT id, started_at, cause FROM outages WHERE ended_at IS NULL ORDER BY started_at DESC LIMIT 1`)
	var o Outage
	err := row.Scan(&o.ID, &o.StartedAt, &o.Cause)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Store) QueryPings(target string, since, until int64) ([]PingResult, error) {
	rows, err := s.db.Query(
		`SELECT id, timestamp, target, rtt_ms, packet_loss, jitter_ms, probe_type
		 FROM ping_results WHERE target = ? AND timestamp >= ? AND timestamp <= ?
		 ORDER BY timestamp ASC`,
		target, since, until,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PingResult
	for rows.Next() {
		var r PingResult
		var loss int
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.Target, &r.RTTMs, &loss, &r.JitterMs, &r.ProbeType); err != nil {
			return nil, err
		}
		r.PacketLoss = loss != 0
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *Store) QueryAllPings(since, until int64) ([]PingResult, error) {
	rows, err := s.db.Query(
		`SELECT id, timestamp, target, rtt_ms, packet_loss, jitter_ms, probe_type
		 FROM ping_results WHERE timestamp >= ? AND timestamp <= ?
		 ORDER BY timestamp ASC`,
		since, until,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PingResult
	for rows.Next() {
		var r PingResult
		var loss int
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.Target, &r.RTTMs, &loss, &r.JitterMs, &r.ProbeType); err != nil {
			return nil, err
		}
		r.PacketLoss = loss != 0
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *Store) QueryDNS(since, until int64) ([]DNSResult, error) {
	rows, err := s.db.Query(
		`SELECT id, timestamp, query, resolver, resolve_ms, success, resolved_ips
		 FROM dns_results WHERE timestamp >= ? AND timestamp <= ?
		 ORDER BY timestamp ASC`,
		since, until,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DNSResult
	for rows.Next() {
		var r DNSResult
		var success int
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.Query, &r.Resolver, &r.ResolveMs, &success, &r.ResolvedIPs); err != nil {
			return nil, err
		}
		r.Success = success != 0
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *Store) QueryHTTP(since, until int64) ([]HTTPResult, error) {
	rows, err := s.db.Query(
		`SELECT id, timestamp, target, total_ms, tls_ms, status_code, success
		 FROM http_results WHERE timestamp >= ? AND timestamp <= ?
		 ORDER BY timestamp ASC`,
		since, until,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []HTTPResult
	for rows.Next() {
		var r HTTPResult
		var success int
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.Target, &r.TotalMs, &r.TLSMs, &r.StatusCode, &success); err != nil {
			return nil, err
		}
		r.Success = success != 0
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *Store) QueryOutages(since, until int64, limit int) ([]Outage, error) {
	rows, err := s.db.Query(
		`SELECT id, started_at, ended_at, duration_ms, cause
		 FROM outages WHERE started_at >= ? AND started_at <= ?
		 ORDER BY started_at DESC LIMIT ?`,
		since, until, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Outage
	for rows.Next() {
		var o Outage
		if err := rows.Scan(&o.ID, &o.StartedAt, &o.EndedAt, &o.DurationMs, &o.Cause); err != nil {
			return nil, err
		}
		results = append(results, o)
	}
	return results, rows.Err()
}

func (s *Store) RecentOutages(limit int) ([]Outage, error) {
	rows, err := s.db.Query(
		`SELECT id, started_at, ended_at, duration_ms, cause
		 FROM outages ORDER BY started_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Outage
	for rows.Next() {
		var o Outage
		if err := rows.Scan(&o.ID, &o.StartedAt, &o.EndedAt, &o.DurationMs, &o.Cause); err != nil {
			return nil, err
		}
		results = append(results, o)
	}
	return results, rows.Err()
}

func (s *Store) LatestPing(target string) (*PingResult, error) {
	row := s.db.QueryRow(
		`SELECT id, timestamp, target, rtt_ms, packet_loss, jitter_ms, probe_type
		 FROM ping_results WHERE target = ? ORDER BY timestamp DESC LIMIT 1`,
		target,
	)
	var r PingResult
	var loss int
	err := row.Scan(&r.ID, &r.Timestamp, &r.Target, &r.RTTMs, &loss, &r.JitterMs, &r.ProbeType)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.PacketLoss = loss != 0
	return &r, nil
}

func (s *Store) LatestDNS() (*DNSResult, error) {
	row := s.db.QueryRow(
		`SELECT id, timestamp, query, resolver, resolve_ms, success, resolved_ips
		 FROM dns_results ORDER BY timestamp DESC LIMIT 1`,
	)
	var r DNSResult
	var success int
	err := row.Scan(&r.ID, &r.Timestamp, &r.Query, &r.Resolver, &r.ResolveMs, &success, &r.ResolvedIPs)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Success = success != 0
	return &r, nil
}

func (s *Store) LatestHTTP() (*HTTPResult, error) {
	row := s.db.QueryRow(
		`SELECT id, timestamp, target, total_ms, tls_ms, status_code, success
		 FROM http_results ORDER BY timestamp DESC LIMIT 1`,
	)
	var r HTTPResult
	var success int
	err := row.Scan(&r.ID, &r.Timestamp, &r.Target, &r.TotalMs, &r.TLSMs, &r.StatusCode, &success)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Success = success != 0
	return &r, nil
}

func (s *Store) PingStats(target string, since int64) (avg float64, count int, lossCount int, err error) {
	row := s.db.QueryRow(
		`SELECT COALESCE(AVG(rtt_ms), 0), COUNT(*), COALESCE(SUM(packet_loss), 0)
		 FROM ping_results WHERE target = ? AND timestamp >= ?`,
		target, since,
	)
	err = row.Scan(&avg, &count, &lossCount)
	return
}

func (s *Store) DNSStats(since int64) (avg float64, count int, err error) {
	row := s.db.QueryRow(
		`SELECT COALESCE(AVG(resolve_ms), 0), COUNT(*)
		 FROM dns_results WHERE timestamp >= ? AND success = 1`,
		since,
	)
	err = row.Scan(&avg, &count)
	return
}

func (s *Store) DeleteOlderThan(before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := before.UnixMilli()
	for _, table := range []string{"ping_results", "dns_results", "http_results"} {
		if _, err := s.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE timestamp < ?", table), ts); err != nil {
			return fmt.Errorf("cleaning %s: %w", table, err)
		}
	}
	if _, err := s.db.Exec("DELETE FROM outages WHERE ended_at IS NOT NULL AND ended_at < ?", ts); err != nil {
		return fmt.Errorf("cleaning outages: %w", err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
