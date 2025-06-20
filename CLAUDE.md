# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Monres is a lightweight VPS resource monitoring tool written in Go. It monitors system resources (CPU, Memory, Disk I/O, Network I/O) by reading directly from `/proc` and `/sys`, triggers alerts based on configurable rules, and sends notifications via Email, Telegram, or stdout.

## Architecture

The application follows a modular architecture with these key components:

- **Main Loop** (`cmd/monres/main.go`): Orchestrates collection, alerting, and notification cycles
- **Collectors** (`internal/collector/`): Gather metrics from system files (`/proc`, `/sys`)
  - `GlobalCollector` coordinates individual metric collectors (CPU, Memory, Disk, Network)
  - Rate-based metrics (disk/network I/O) calculate deltas between collection cycles
- **History Buffer** (`internal/history/`): Maintains time-series data for duration-based alerts
- **Alerter** (`internal/alerter/`): Evaluates alert rules with configurable conditions, thresholds, durations, and aggregations
- **Notifiers** (`internal/notifier/`): Send notifications via Email (SMTP), Telegram, or stdout
- **State Management** (`internal/state/`): Persists alert states across restarts
- **Configuration** (`internal/config/`): YAML-based config with environment variable support for secrets

## Development Commands

### Building and Testing
```bash
# Build the binary
make build

# Clean build artifacts  
make clean

# Run with custom config
./monres -config /path/to/config.yaml
```

### Installation and Service Management
```bash
# Install system service (creates user, copies files, systemd service)
make install

# Uninstall completely
make uninstall

# Reinstall (uninstall + install)
make reinstall

# Create GitHub release
make release name=v1.0.0
```

### Docker
```bash
# Build Docker image
docker build -t monres .

# Run in container
docker run -v /path/to/config.yaml:/app/config.yaml monres
```

## Configuration

- Main config: `config.yaml` (see `config.example.yaml`)
- Secrets via environment variables with pattern: `MONRES_<FIELD>_<CHANNEL_NAME>`
- Default config path: `/etc/monres/config.yaml` for systemd service
- Environment file: `/etc/monres/monres.env`

## Key Metrics Collected

- `cpu_percent_total`: Total CPU usage percentage
- `mem_percent_used/free`: Memory usage based on MemAvailable
- `swap_percent_used/free`: Swap usage percentage  
- `disk_read/write_bytes_ps`: Disk I/O rates (bytes per second)
- `net_recv/sent_bytes_ps`: Network I/O rates (bytes per second)

## Alert System

Alerts support:
- **Conditions**: `>`, `<`, `>=`, `<=`
- **Durations**: Time window for sustained conditions (e.g., "1m", "5m")
- **Aggregations**: `average`, `max` over the duration window
- **Multiple channels**: Email, Telegram, stdout per alert

## Code Patterns

- Error handling follows Go conventions with explicit error returns
- Concurrent-safe operations use `sync.Mutex` where needed
- Rate calculations store previous state in collector structs
- Template-based notifications with customizable messages
- Environment variable injection for sensitive configuration values