<p align="center">
  <img src="assets/banner.png" alt="pingolin" width="400">
</p>

[![Go Report Card](https://goreportcard.com/badge/github.com/fabioconcina/pingolin)](https://goreportcard.com/report/github.com/fabioconcina/pingolin)
[![GitHub release](https://img.shields.io/github/v/release/fabioconcina/pingolin)](https://github.com/fabioconcina/pingolin/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/fabioconcina/pingolin)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

# pingolin

Internet connection health monitor — run it, see what's going on.

## Install

```
go install github.com/fabioconcina/pingolin@latest
```

## Usage

```
pingolin              # launch TUI with live monitoring
pingolin daemon       # run as background service
pingolin status       # quick one-shot health check
pingolin history      # show stats for last 24h
pingolin export       # export data as CSV or JSON
pingolin mcp          # run as MCP server (stdio)
```

## What it monitors

- ICMP latency and packet loss to multiple targets (1.1.1.1, 8.8.8.8, 9.9.9.9)
- DNS resolution time (system resolver + 1.1.1.1)
- HTTP connectivity (Google generate_204)
- Jitter calculation over sliding window
- Outage detection with historical logging and cause classification

## AI integration

### MCP server (Model Context Protocol)

```
pingolin mcp
```

Runs an MCP server on stdio, exposing a `check_connection` tool. AI agents (Claude Desktop, Claude Code) can query connection health directly.

Claude Desktop configuration:

```json
{
  "mcpServers": {
    "pingolin": {
      "command": "/path/to/pingolin",
      "args": ["mcp"]
    }
  }
}
```

### JSON export

```
pingolin export --format json
```

Returns structured JSON to stdout. Pipe to any tool:

```
pingolin export --format json | jq '.pings[] | select(.packet_loss == true)'
```

### Exit codes

- 0: success
- 1: error (config load failure, database error, I/O error)
- 2: connection unhealthy (status command only — at least one probe failing)

## Running as a service

pingolin monitors continuously — to collect data 24/7, run it as a systemd service:

```
sudo pingolin service install
```

This creates a systemd unit, enables it, and starts it. The TUI auto-detects the running daemon and displays its data.

```
sudo pingolin service status      # check service status
sudo pingolin service logs        # view recent logs
sudo pingolin service uninstall   # stop and remove
```

On Linux, ICMP requires `CAP_NET_RAW`. The service unit grants this automatically via `AmbientCapabilities`. For manual use, set it with:

```
sudo setcap cap_net_raw+ep /path/to/pingolin
```

## Configuration

Copy `config.toml.example` to `~/.config/pingolin/config.toml` and edit as needed.
CLI flags override config file values.

```
--config PATH        Config file path
--db PATH            Database path
--ping-interval 5s   ICMP probe interval
--dns-interval 30s   DNS probe interval
--http-interval 30s  HTTP probe interval
--targets 1.1.1.1,8.8.8.8  Comma-separated ICMP targets
--retention 30d      Data retention period
--verbose            Debug logging
```

## License

MIT
