# ResMon - Lightweight VPS Resource Monitor

ResMon is a simple, lightweight, and easy-to-install software tool for monitoring
core system resources (CPU, Memory, Disk I/O, Network I/O) on an Ubuntu VPS.
It runs as a background service, triggers alerts based on user-defined thresholds,
and sends notifications via Email and Telegram.

Version: 1.1

## Features

- Monitors CPU, Memory, Disk I/O, Network I/O.
- Direct OS metric collection (reads `/proc`, `/sys`).
- Configurable alert rules (threshold, duration, aggregation).
- Notifications via Email (SMTP) and Telegram.
- Customizable notification templates.
- Sensitive credentials read from environment variables.
- Persistent alert state across restarts.
- Designed for minimal resource consumption.
- Single binary deployment (Go).
- systemd service for management.

## Installation

### Prerequisites

- Go (latest stable version, for building)
- An Ubuntu VPS

### Building from Source

1.  Clone the repository:
    ```bash
    git clone [https://github.com/mattmezza/resmon.git](https://github.com/mattmezza/resmon.git)
    cd resmon
    ```

2.  Build the binary:
    ```bash
    go build -ldflags="-s -w" -o resmon cmd/resmon/main.go
    ```
    The `-s -w` flags strip debug information and symbol table, reducing binary size.

3.  Copy the binary to a suitable location:
    ```bash
    sudo cp resmon /usr/local/bin/
    ```

### Configuration

1.  Create the configuration directory:
    ```bash
    sudo mkdir -p /etc/resmon
    ```

2.  Copy the example configuration:
    ```bash
    sudo cp config.example.yaml /etc/resmon/config.yaml
    ```

3.  Edit `/etc/resmon/config.yaml` to suit your needs. Refer to the comments within the file and the "Configuration Details" section below.

4.  Create the environment file for sensitive credentials:
    ```bash
    sudo touch /etc/resmon/resmon.env
    sudo chown resmon:resmon /etc/resmon/resmon.env # Assuming 'resmon' user/group
    sudo chmod 600 /etc/resmon/resmon.env
    ```
    Edit `/etc/resmon/resmon.env` and add your secrets, for example:
    ```ini
    RESMON_SMTP_PASSWORD_CRITICAL_EMAIL="your_smtp_password"
    RESMON_TELEGRAM_TOKEN_OPS_TELEGRAM="your_telegram_bot_token"
    ```
    The environment variable names are constructed as `RESMON_<SENSITIVE_FIELD_UPPERCASE>_<CHANNEL_NAME_UPPERCASE_UNDERSCORED>`.
    For example, for a channel named `my-email-channel` and field `smtp_password`, the env var would be `RESMON_SMTP_PASSWORD_MY_EMAIL_CHANNEL`.

### Setup as a Systemd Service

1.  Create a dedicated system user for ResMon:
    ```bash
    sudo groupadd --system resmon
    sudo useradd --system --gid resmon --shell /sbin/nologin --home-dir /var/lib/resmon resmon
    sudo mkdir -p /var/lib/resmon
    sudo chown -R resmon:resmon /var/lib/resmon
    # Grant read access to config for the resmon user
    sudo chown root:resmon /etc/resmon # dir owned by root, group readable by resmon
    sudo chmod 750 /etc/resmon
    sudo chown root:resmon /etc/resmon/config.yaml
    sudo chmod 640 /etc/resmon/config.yaml
    # Ensure resmon.env is also correctly permissioned and owned as above
    sudo chown resmon:resmon /etc/resmon/resmon.env
    sudo chmod 600 /etc/resmon/resmon.env

    ```

2.  Copy the systemd service file:
    ```bash
    sudo cp deploy/systemd/resmon.service /etc/systemd/system/
    ```

3.  Reload systemd, enable, and start the service:
    ```bash
    sudo systemctl daemon-reload
    sudo systemctl enable resmon.service
    sudo systemctl start resmon.service
    ```

4.  Check the status:
    ```bash
    sudo systemctl status resmon.service
    journalctl -u resmon -f
    ```

## Configuration Details

*(Explain `interval_seconds`, `state_file`, `hostname`, `alerts` (all fields), `notification_channels` (all fields per type), `templates` here)*

## Metrics Collected

-   `cpu_percent_total`: Total CPU usage percentage.
-   `mem_percent_used`: Used memory percentage (based on MemAvailable).
-   `mem_percent_free`: Free memory percentage (based on MemAvailable).
-   `swap_percent_used`: Used swap percentage.
-   `swap_percent_free`: Free swap percentage.
-   `disk_read_bytes_ps`: Aggregated disk read bytes per second.
-   `disk_write_bytes_ps`: Aggregated disk write bytes per second.
-   `net_recv_bytes_ps`: Aggregated network received bytes per second.
-   `net_sent_bytes_ps`: Aggregated network transmitted bytes per second.

## Testing

(Details on how to run unit/integration tests if provided)

```bash
go test ./...
