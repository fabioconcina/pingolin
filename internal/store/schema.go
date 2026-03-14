package store

const schema = `
CREATE TABLE IF NOT EXISTS ping_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    target TEXT NOT NULL,
    rtt_ms REAL,
    packet_loss INTEGER NOT NULL,
    jitter_ms REAL,
    probe_type TEXT NOT NULL DEFAULT 'icmp'
);

CREATE TABLE IF NOT EXISTS dns_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    query TEXT NOT NULL,
    resolver TEXT NOT NULL,
    resolve_ms REAL,
    success INTEGER NOT NULL,
    resolved_ips TEXT
);

CREATE TABLE IF NOT EXISTS http_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    target TEXT NOT NULL,
    total_ms REAL,
    tls_ms REAL,
    status_code INTEGER,
    success INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS outages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at INTEGER NOT NULL,
    ended_at INTEGER,
    duration_ms INTEGER,
    cause TEXT
);

CREATE INDEX IF NOT EXISTS idx_ping_ts ON ping_results(timestamp);
CREATE INDEX IF NOT EXISTS idx_dns_ts ON dns_results(timestamp);
CREATE INDEX IF NOT EXISTS idx_http_ts ON http_results(timestamp);
CREATE INDEX IF NOT EXISTS idx_outages_started ON outages(started_at);
`
